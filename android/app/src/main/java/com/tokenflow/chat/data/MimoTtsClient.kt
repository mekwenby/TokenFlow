package com.tokenflow.chat.data

import android.content.Context
import java.io.File
import java.io.IOException
import java.security.MessageDigest
import java.util.Base64
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonArray
import kotlinx.serialization.json.putJsonObject
import okhttp3.Call
import okhttp3.Callback
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response

data class TtsAudio(val file: File, val fromCache: Boolean)

class MimoTtsClient(
    context: Context,
    private val secrets: SecretStore,
    private val json: Json = DirectApiTransport.defaultJson,
    private val client: OkHttpClient = OkHttpClient(),
    private val endpoint: String = ENDPOINT,
) {
    private val cacheDir = File(context.applicationContext.cacheDir, "mimo_tts").apply { mkdirs() }

    fun configured(): Boolean = !secrets.read(SecretStore.MIMO_TTS_KEY).isNullOrBlank()

    fun saveKey(value: String) {
        if (value.isBlank()) secrets.remove(SecretStore.MIMO_TTS_KEY)
        else secrets.write(SecretStore.MIMO_TTS_KEY, value.trim())
    }

    suspend fun synthesize(messageId: String, markdown: String, voice: String, force: Boolean = false): TtsAudio {
        val key = secrets.read(SecretStore.MIMO_TTS_KEY)
            ?.takeIf(String::isNotBlank) ?: throw ConfigurationException("MiMo TTS API key is not configured")
        require(voice in VOICES) { "Unsupported MiMo voice" }
        val text = markdownToSpeech(markdown)
        require(text.isNotBlank()) { "There is no text to synthesize" }
        val hash = sha256("$messageId\u0000$voice\u0000$text")
        val output = File(cacheDir, "$hash.wav")
        if (!force && output.isFile && output.length() > 44) return TtsAudio(output, true)

        val body = buildJsonObject {
            put("model", MODEL)
            put("stream", false)
            putJsonArray("modalities") { add(JsonPrimitive("text")); add(JsonPrimitive("audio")) }
            putJsonArray("messages") { add(buildJsonObject { put("role", "assistant"); put("content", text) }) }
            putJsonObject("audio") { put("voice", voice); put("format", "wav") }
        }
        val request = Request.Builder()
            .url(endpoint)
            .header("Authorization", "Bearer $key")
            .header("Content-Type", "application/json")
            .post(json.encodeToString(body).toRequestBody(JSON_MEDIA_TYPE))
            .build()
        val raw = execute(request)
        val payload = runCatching { json.parseToJsonElement(raw).jsonObject }
            .getOrElse { throw IOException("MiMo returned invalid JSON") }
        val audio = runCatching {
            payload["choices"]!!.jsonArray[0].jsonObject["message"]!!.jsonObject["audio"]!!.jsonObject["data"]!!
                .jsonPrimitive.content
        }.getOrElse { throw IOException("MiMo response did not contain WAV audio") }
        val bytes = runCatching { Base64.getDecoder().decode(audio) }
            .getOrElse { throw IOException("MiMo returned invalid audio data") }
        require(bytes.size > 44) { "MiMo returned empty audio" }
        val temporary = File(cacheDir, "$hash.tmp")
        temporary.writeBytes(bytes)
        if (output.exists()) output.delete()
        check(temporary.renameTo(output)) { "Unable to cache generated audio" }
        return TtsAudio(output, false)
    }

    private suspend fun execute(request: Request): String = suspendCancellableCoroutine { continuation ->
        val call = client.newCall(request)
        continuation.invokeOnCancellation { call.cancel() }
        call.enqueue(object : Callback {
            override fun onFailure(call: Call, error: IOException) {
                if (continuation.isActive) continuation.resumeWithException(error)
            }

            override fun onResponse(call: Call, response: Response) {
                response.use {
                    val raw = it.body?.string().orEmpty()
                    if (!it.isSuccessful) {
                        val message = runCatching {
                            json.parseToJsonElement(raw).jsonObject["error"]?.jsonObject?.get("message")?.jsonPrimitive?.content
                        }.getOrNull().orEmpty().ifBlank { "MiMo TTS request failed (${it.code})" }
                        if (continuation.isActive) continuation.resumeWithException(ApiException(it.code, message = message))
                    } else if (continuation.isActive) continuation.resume(raw)
                }
            }
        })
    }

    companion object {
        const val ENDPOINT = "https://api.xiaomimimo.com/v1/chat/completions"
        const val MODEL = "mimo-v2.5-tts"
        val VOICES = listOf("mimo_default", "冰糖", "茉莉", "苏打", "白桦", "Mia", "Chloe", "Milo", "Dean")
        private val JSON_MEDIA_TYPE = "application/json; charset=utf-8".toMediaType()
    }
}

internal fun markdownToSpeech(value: String): String = value
    .replace(Regex("```[\\s\\S]*?```"), " code block ")
    .replace(Regex("`([^`]+)`"), "$1")
    .replace(Regex("!\\[[^]]*]\\([^)]*\\)"), "")
    .replace(Regex("\\[([^]]+)]\\([^)]*\\)"), "$1")
    .replace(Regex("<[^>]+>"), " ")
    .replace(Regex("(?m)^#{1,6}\\s*"), "")
    .replace(Regex("[*_~>|]"), "")
    .replace(Regex("\\s+"), " ")
    .trim()

private fun sha256(value: String): String = MessageDigest.getInstance("SHA-256")
    .digest(value.encodeToByteArray()).joinToString("") { "%02x".format(it) }
