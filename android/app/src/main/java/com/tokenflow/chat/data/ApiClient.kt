package com.tokenflow.chat.data

import com.tokenflow.chat.BuildConfig
import android.util.Log
import java.io.IOException
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.withContext
import kotlinx.coroutines.job
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.longOrNull
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonArray
import kotlinx.serialization.json.putJsonObject
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

object ProviderValidator {
    fun normalizeBaseUrl(value: String): String {
        val url = value.trim().trimEnd('/').toHttpUrlOrNull()
            ?: throw ConfigurationException("Invalid API base URL")
        if (!url.isHttps) throw ConfigurationException("API base URL must use HTTPS")
        if (url.username.isNotEmpty() || url.password.isNotEmpty()) {
            throw ConfigurationException("API base URL cannot contain credentials")
        }
        if (url.query != null || url.fragment != null) {
            throw ConfigurationException("API base URL cannot contain a query or fragment")
        }
        return url.toString().trimEnd('/')
    }

    fun validate(draft: ProviderDraft) {
        if (draft.name.trim().isEmpty()) throw ConfigurationException("Provider name is required")
        normalizeBaseUrl(draft.baseUrl)
        if (draft.apiKey.isBlank()) throw ConfigurationException("API key is required")
    }
}

class DirectApiTransport(
    private val json: Json = defaultJson,
    client: OkHttpClient? = null,
) {
    private val client = (client?.newBuilder() ?: OkHttpClient.Builder()
        .connectTimeout(20, TimeUnit.SECONDS)
        .readTimeout(45, TimeUnit.SECONDS)
        .writeTimeout(30, TimeUnit.SECONDS))
        .build()
    private val streamingClient = this.client.newBuilder().readTimeout(0, TimeUnit.MILLISECONDS).build()
    private val parser = SseParser()

    suspend fun getJson(
        provider: ProviderConfig,
        apiKey: String,
        path: String,
        query: Map<String, String> = emptyMap(),
    ): JsonObject = withContext(Dispatchers.IO) {
        val url = endpoint(provider.baseUrl, path).newBuilder().apply {
            query.forEach { (name, value) -> addQueryParameter(name, value) }
        }.build()
        val request = authenticated(Request.Builder().url(url).get(), provider.protocol, apiKey).build()
        client.newCall(request).execute().use { response ->
            val body = response.body?.string().orEmpty()
            if (!response.isSuccessful) throw apiException(response.code, body)
            parseObject(body)
        }
    }

    fun stream(
        provider: ProviderConfig,
        apiKey: String,
        path: String,
        body: JsonObject,
        requestId: String,
    ): Flow<RawSseEvent> = flow {
        val request = authenticated(
            Request.Builder()
                .url(endpoint(provider.baseUrl, path))
                .header("Accept", "text/event-stream")
                .header("X-Client-Request-Id", requestId)
                .post(json.encodeToString(body).toRequestBody(JSON_MEDIA_TYPE)),
            provider.protocol,
            apiKey,
        ).build()
        val call = streamingClient.newCall(request)
        val cancellation = currentCoroutineContext().job.invokeOnCompletion { call.cancel() }
        try {
            call.execute().use { response ->
                if (!response.isSuccessful) {
                    val error = apiException(response.code, response.body?.string().orEmpty())
                    runCatching {
                        Log.w(
                            TAG,
                            "Provider request failed: protocol=${provider.protocol}, path=${path.trim('/')}, " +
                                "status=${response.code}, code=${error.code ?: "none"}",
                        )
                    }
                    throw error
                }
                val source = response.body?.source() ?: throw IOException("Provider returned an empty stream")
                parser.read(source) { emit(it) }
            }
        } finally {
            cancellation.dispose()
            call.cancel()
        }
    }.flowOn(Dispatchers.IO)

    private fun authenticated(builder: Request.Builder, protocol: ProviderProtocol, apiKey: String): Request.Builder {
        builder.header("Content-Type", "application/json")
            .header("User-Agent", "TokenFlow-Android/${BuildConfig.VERSION_NAME}")
        if (protocol == ProviderProtocol.ANTHROPIC_MESSAGES) {
            builder.header("x-api-key", apiKey).header("anthropic-version", ANTHROPIC_VERSION)
        } else {
            builder.header("Authorization", "Bearer $apiKey")
        }
        return builder
    }

    private fun endpoint(baseUrl: String, path: String): HttpUrl =
        "${baseUrl.trimEnd('/')}/${path.trimStart('/')}".toHttpUrl()

    private fun parseObject(raw: String): JsonObject = runCatching {
        json.parseToJsonElement(raw).jsonObject
    }.getOrElse { throw IOException("Provider returned invalid JSON", it) }

    private fun apiException(status: Int, body: String): ApiException {
        val parsed = runCatching { json.parseToJsonElement(body).jsonObject }.getOrNull()
        val errorElement = parsed?.get("error")
        val errorObject = (errorElement as? JsonObject)
        val message = errorObject?.string("message").orEmpty()
            .ifBlank { (errorElement as? JsonPrimitive)?.contentOrNull.orEmpty() }
            .ifBlank { "Provider request failed ($status)" }
        val code = errorObject?.string("code").orEmpty().ifBlank { parsed?.string("code").orEmpty() }.ifBlank { null }
        return ApiException(status, code, message)
    }

    companion object {
        private const val TAG = "TokenFlowProvider"
        const val ANTHROPIC_VERSION = "2023-06-01"
        private val JSON_MEDIA_TYPE = "application/json; charset=utf-8".toMediaType()
        val defaultJson = Json { ignoreUnknownKeys = true; explicitNulls = false; encodeDefaults = true }
    }
}

interface ModelApiAdapter {
    suspend fun listModels(provider: ProviderConfig, apiKey: String): List<RemoteModel>
    fun stream(request: ModelCallRequest): Flow<ModelStreamEvent>
}

class ModelGateway(
    private val transport: DirectApiTransport = DirectApiTransport(),
    private val json: Json = DirectApiTransport.defaultJson,
) {
    private val adapters: Map<ProviderProtocol, ModelApiAdapter> = mapOf(
        ProviderProtocol.OPENAI_CHAT_COMPLETIONS to OpenAIChatAdapter(transport, json),
        ProviderProtocol.OPENAI_RESPONSES to OpenAIResponsesAdapter(transport, json),
        ProviderProtocol.ANTHROPIC_MESSAGES to AnthropicMessagesAdapter(transport, json),
    )

    suspend fun listModels(draft: ProviderDraft): List<RemoteModel> {
        ProviderValidator.validate(draft)
        val now = System.currentTimeMillis()
        val provider = ProviderConfig(
            id = draft.id,
            name = draft.name.trim(),
            baseUrl = ProviderValidator.normalizeBaseUrl(draft.baseUrl),
            protocol = draft.protocol,
            apiKeyConfigured = true,
            createdAt = now,
            updatedAt = now,
        )
        return adapter(provider.protocol).listModels(provider, draft.apiKey.trim())
    }

    suspend fun listModels(provider: ProviderConfig, apiKey: String): List<RemoteModel> =
        adapter(provider.protocol).listModels(provider, apiKey)

    fun stream(request: ModelCallRequest): Flow<ModelStreamEvent> = adapter(request.provider.protocol).stream(request)

    private fun adapter(protocol: ProviderProtocol): ModelApiAdapter = requireNotNull(adapters[protocol])
}

private abstract class OpenAIAdapter(
    protected val transport: DirectApiTransport,
    protected val json: Json,
) : ModelApiAdapter {
    override suspend fun listModels(provider: ProviderConfig, apiKey: String): List<RemoteModel> {
        val body = transport.getJson(provider, apiKey, "models")
        return body.array("data").mapNotNull { element ->
            val item = element as? JsonObject ?: return@mapNotNull null
            item.string("id").takeIf(String::isNotBlank)?.let(::RemoteModel)
        }.distinctBy { it.id }.sortedBy { it.id.lowercase() }
    }
}

private class OpenAIChatAdapter(
    transport: DirectApiTransport,
    json: Json,
) : OpenAIAdapter(transport, json) {
    override fun stream(request: ModelCallRequest): Flow<ModelStreamEvent> = flow {
        transport.stream(request.provider, request.apiKey, "chat/completions", chatBody(request), request.requestId).collect { raw ->
            if (raw.data.trim() == "[DONE]") return@collect
            val payload = jsonObject(raw.data, json)
            payload["error"]?.let { throw streamError(it) }
            payload.array("choices").forEach { choiceElement ->
                val choice = choiceElement as? JsonObject ?: return@forEach
                val delta = choice.obj("delta")
                delta.string("content").takeIf(String::isNotEmpty)?.let { emit(ModelStreamEvent.TextDelta(it)) }
                listOf("reasoning_content", "reasoning", "thinking").firstNotNullOfOrNull { key ->
                    delta.string(key).takeIf(String::isNotEmpty)
                }?.let { emit(ModelStreamEvent.ThinkingDelta(it)) }
                delta.array("tool_calls").forEach { callElement ->
                    val call = callElement as? JsonObject ?: return@forEach
                    val function = call.obj("function")
                    emit(
                        ModelStreamEvent.ToolCallDelta(
                            index = call.int("index"),
                            id = call.string("id"),
                            name = function.string("name"),
                            arguments = function.string("arguments"),
                        ),
                    )
                }
            }
            payload["usage"]?.let { usageElement ->
                (usageElement as? JsonObject)?.let { emit(ModelStreamEvent.TokenUsage(openAIUsage(it))) }
            }
        }
        emit(ModelStreamEvent.Completed)
    }

    private fun chatBody(request: ModelCallRequest) = buildJsonObject {
        put("model", request.model.remoteId)
        put("stream", true)
        put("max_tokens", request.maxOutputTokens.coerceIn(1, 131_072))
        putJsonObject("stream_options") { put("include_usage", true) }
        if (request.thinkingEffort != "off") put("reasoning_effort", request.thinkingEffort)
        putJsonArray("messages") {
            add(buildJsonObject { put("role", "system"); put("content", request.systemPrompt) })
            request.messages.mapNotNull(::chatMessage).forEach(::add)
        }
        if (request.tools.isNotEmpty()) {
            putJsonArray("tools") { request.tools.forEach { add(openAITool(it, nestedFunction = true)) } }
            put("tool_choice", "auto")
        }
    }

    private fun chatMessage(message: CanonicalMessage): JsonObject? {
        val parts = message.contentParts()
        if (message.role != "tool" && parts.isEmpty() && message.toolCalls.isEmpty()) return null
        return buildJsonObject {
            put("role", message.role)
            when (message.role) {
                "tool" -> {
                    put("tool_call_id", message.toolCallId.orEmpty())
                    put("content", message.content)
                }
                else -> {
                if (parts.any { it is CanonicalContentPart.Image }) putJsonArray("content") {
                    parts.forEach { part -> when (part) {
                        is CanonicalContentPart.Text -> add(buildJsonObject { put("type", "text"); put("text", part.text) })
                        is CanonicalContentPart.Document -> add(buildJsonObject {
                            put("type", "text")
                            put("text", "[Document: ${part.fileName}]\n${part.text}")
                        })
                        is CanonicalContentPart.Image -> add(buildJsonObject {
                            put("type", "image_url")
                            putJsonObject("image_url") { put("url", "data:${part.mimeType};base64,${part.base64}") }
                        })
                    } }
                    } else if (parts.isNotEmpty()) put("content", flattenedContent(parts))
                    if (message.toolCalls.isNotEmpty()) putJsonArray("tool_calls") {
                        message.toolCalls.forEach { call ->
                            add(buildJsonObject {
                                put("id", call.id)
                                put("type", "function")
                                putJsonObject("function") { put("name", call.name); put("arguments", call.arguments) }
                            })
                        }
                    }
                }
            }
        }
    }
}

private class OpenAIResponsesAdapter(
    transport: DirectApiTransport,
    json: Json,
) : OpenAIAdapter(transport, json) {
    override fun stream(request: ModelCallRequest): Flow<ModelStreamEvent> = flow {
        transport.stream(request.provider, request.apiKey, "responses", responsesBody(request), request.requestId).collect { raw ->
            val payload = jsonObject(raw.data, json)
            val type = payload.string("type").ifBlank { raw.event }
            when (type) {
                "response.output_text.delta" -> payload.string("delta").takeIf(String::isNotEmpty)
                    ?.let { emit(ModelStreamEvent.TextDelta(it)) }
                "response.reasoning_summary_text.delta", "response.reasoning_text.delta" ->
                    payload.string("delta").takeIf(String::isNotEmpty)?.let { emit(ModelStreamEvent.ThinkingDelta(it)) }
                "response.output_item.added", "response.output_item.done" -> {
                    val item = payload.obj("item")
                    if (item.string("type") == "function_call") {
                        val arguments = item.string("arguments").let { value ->
                            if (type == "response.output_item.added" && value == "{}") "" else value
                        }
                        emit(
                            ModelStreamEvent.ToolCallDelta(
                                index = payload.int("output_index"),
                                id = item.string("call_id").ifBlank { item.string("id") },
                                name = item.string("name"),
                                arguments = arguments,
                                replaceArguments = true,
                            ),
                        )
                    }
                    if (type == "response.output_item.done" && item.isNotEmpty()) emit(
                        ModelStreamEvent.ReplayItem(
                            ProtocolReplayItem(
                                protocol = ProviderProtocol.OPENAI_RESPONSES,
                                index = payload.int("output_index"),
                                payload = item,
                            ),
                        ),
                    )
                }
                "response.function_call_arguments.delta" -> emit(
                    ModelStreamEvent.ToolCallDelta(
                        index = payload.int("output_index"),
                        id = payload.string("call_id"),
                        arguments = payload.string("delta"),
                    ),
                )
                "response.function_call_arguments.done" -> emit(
                    ModelStreamEvent.ToolCallDelta(
                        index = payload.int("output_index"),
                        id = payload.string("call_id"),
                        arguments = payload.string("arguments"),
                        replaceArguments = true,
                    ),
                )
                "response.completed" -> payload.obj("response").obj("usage").takeIf { it.isNotEmpty() }
                    ?.let { emit(ModelStreamEvent.TokenUsage(responsesUsage(it))) }
                "response.failed", "error" -> throw ApiException(
                    status = 0,
                    code = payload.obj("error").string("code").ifBlank { payload.string("code") },
                    message = payload.obj("error").string("message").ifBlank { payload.string("message") }
                        .ifBlank { "Provider response failed" },
                )
            }
        }
        emit(ModelStreamEvent.Completed)
    }

    private fun responsesBody(request: ModelCallRequest) = buildJsonObject {
        put("model", request.model.remoteId)
        put("instructions", request.systemPrompt)
        put("stream", true)
        put("max_output_tokens", request.maxOutputTokens.coerceIn(1, 131_072))
        if (request.thinkingEffort != "off") putJsonObject("reasoning") {
            put("effort", request.thinkingEffort)
            put("summary", "auto")
        }
        putJsonArray("input") {
            request.messages.flatMap(::responseItems).forEach(::add)
        }
        if (request.tools.isNotEmpty()) {
            putJsonArray("tools") { request.tools.forEach { add(openAITool(it, nestedFunction = false)) } }
            put("tool_choice", "auto")
        }
    }

    private fun responseItems(message: CanonicalMessage): List<JsonObject> {
        if (message.role == "tool") {
            val callId = message.toolCallId.orEmpty()
            if (callId.isBlank()) return emptyList()
            return listOf(buildJsonObject {
                put("type", "function_call_output")
                put("call_id", callId)
                put("output", message.content)
            })
        }

        val replay = message.replayItems
            .filter { it.protocol == ProviderProtocol.OPENAI_RESPONSES }
            .sortedBy(ProtocolReplayItem::index)
            .map(ProtocolReplayItem::payload)
        val result = replay.toMutableList()
        val hasMessageReplay = replay.any { it.string("type") == "message" }
        val parts = message.contentParts()
        if (!hasMessageReplay && parts.isNotEmpty()) {
            val easyMessage = buildJsonObject {
                put("role", message.role)
                if (message.role == "user" && parts.any { it is CanonicalContentPart.Image }) {
                    putJsonArray("content") {
                        parts.forEach { part -> when (part) {
                            is CanonicalContentPart.Text -> add(buildJsonObject {
                                put("type", "input_text")
                                put("text", part.text)
                            })
                            is CanonicalContentPart.Document -> add(buildJsonObject {
                                put("type", "input_text")
                                put("text", "[Document: ${part.fileName}]\n${part.text}")
                            })
                            is CanonicalContentPart.Image -> add(buildJsonObject {
                                put("type", "input_image")
                                put("image_url", "data:${part.mimeType};base64,${part.base64}")
                            })
                        } }
                    }
                } else {
                    put("content", flattenedContent(parts))
                }
            }
            val firstNonReasoning = result.indexOfFirst { it.string("type") != "reasoning" }
                .let { if (it < 0) result.size else it }
            result.add(firstNonReasoning, easyMessage)
        }

        val replayedCallIds = replay
            .filter { it.string("type") == "function_call" }
            .map { it.string("call_id") }
            .toSet()
        message.toolCalls.filterNot { it.id in replayedCallIds }.forEach { call ->
            result += buildJsonObject {
                put("type", "function_call")
                put("call_id", call.id)
                put("name", call.name)
                put("arguments", call.arguments)
            }
        }
        return result
    }
}

private class AnthropicMessagesAdapter(
    private val transport: DirectApiTransport,
    private val json: Json,
) : ModelApiAdapter {
    override suspend fun listModels(provider: ProviderConfig, apiKey: String): List<RemoteModel> {
        val models = mutableListOf<RemoteModel>()
        var afterId = ""
        var page = 0
        while (page++ < 20) {
            val payload = transport.getJson(
                provider,
                apiKey,
                "models",
                if (afterId.isBlank()) emptyMap() else mapOf("after_id" to afterId),
            )
            payload.array("data").forEach { element ->
                val item = element as? JsonObject ?: return@forEach
                item.string("id").takeIf(String::isNotBlank)?.let { id ->
                    models += RemoteModel(id, item.string("display_name").ifBlank { id })
                }
            }
            if (payload["has_more"]?.jsonPrimitive?.booleanOrNull != true) break
            afterId = payload.string("last_id")
            if (afterId.isBlank()) break
        }
        return models.distinctBy { it.id }.sortedBy { it.displayName.lowercase() }
    }

    override fun stream(request: ModelCallRequest): Flow<ModelStreamEvent> = flow {
        val replayBlocks = mutableMapOf<Int, AnthropicReplayAccumulator>()
        transport.stream(request.provider, request.apiKey, "messages", anthropicBody(request), request.requestId).collect { raw ->
            val payload = jsonObject(raw.data, json)
            val type = payload.string("type").ifBlank { raw.event }
            when (type) {
                "message_start" -> payload.obj("message").obj("usage").takeIf { it.isNotEmpty() }
                    ?.let { emit(ModelStreamEvent.TokenUsage(anthropicUsage(it))) }
                "content_block_start" -> {
                    val block = payload.obj("content_block")
                    val index = payload.int("index")
                    when (block.string("type")) {
                        "tool_use" -> emit(
                            ModelStreamEvent.ToolCallDelta(
                                index = index,
                                id = block.string("id"),
                                name = block.string("name"),
                                arguments = block["input"]?.toString().orEmpty().takeUnless { it == "{}" }.orEmpty(),
                                replaceArguments = true,
                            ),
                        )
                        "thinking", "redacted_thinking" -> replayBlocks[index] = AnthropicReplayAccumulator(block)
                    }
                }
                "content_block_delta" -> {
                    val delta = payload.obj("delta")
                    when (delta.string("type")) {
                        "text_delta" -> emit(ModelStreamEvent.TextDelta(delta.string("text")))
                        "thinking_delta" -> {
                            replayBlocks[payload.int("index")]?.appendThinking(delta.string("thinking"))
                            emit(ModelStreamEvent.ThinkingDelta(delta.string("thinking")))
                        }
                        "signature_delta" -> replayBlocks[payload.int("index")]?.appendSignature(delta.string("signature"))
                        "input_json_delta" -> emit(
                            ModelStreamEvent.ToolCallDelta(
                                index = payload.int("index"),
                                arguments = delta.string("partial_json"),
                            ),
                        )
                    }
                }
                "content_block_stop" -> replayBlocks.remove(payload.int("index"))?.let { block ->
                    emit(
                        ModelStreamEvent.ReplayItem(
                            ProtocolReplayItem(
                                protocol = ProviderProtocol.ANTHROPIC_MESSAGES,
                                index = payload.int("index"),
                                payload = block.payload(),
                            ),
                        ),
                    )
                }
                "message_delta" -> payload.obj("usage").takeIf { it.isNotEmpty() }
                    ?.let { emit(ModelStreamEvent.TokenUsage(anthropicUsage(it))) }
                "error" -> throw ApiException(
                    status = 0,
                    code = payload.obj("error").string("type"),
                    message = payload.obj("error").string("message").ifBlank { "Provider response failed" },
                )
            }
        }
        emit(ModelStreamEvent.Completed)
    }

    private fun anthropicBody(request: ModelCallRequest) = buildJsonObject {
        put("model", request.model.remoteId)
        put("system", request.systemPrompt)
        put("stream", true)
        put("max_tokens", request.maxOutputTokens.coerceIn(1, 131_072))
        val budget = thinkingBudget(request.thinkingEffort, request.maxOutputTokens)
        if (budget > 0) putJsonObject("thinking") { put("type", "enabled"); put("budget_tokens", budget) }
        put("messages", anthropicMessages(request.messages))
        if (request.tools.isNotEmpty()) putJsonArray("tools") {
            request.tools.forEach { tool -> add(buildJsonObject {
                put("name", tool.name)
                put("description", tool.description)
                put("input_schema", tool.parameters)
            }) }
        }
    }
}

private fun anthropicMessages(messages: List<CanonicalMessage>): JsonArray = buildJsonArray {
    val turns = mutableListOf<Pair<String, MutableList<JsonElement>>>()
    messages.forEach { message ->
        val role = if (message.role == "tool") "user" else message.role
        val blocks = mutableListOf<JsonElement>()
        if (message.role == "tool") {
            val callId = message.toolCallId.orEmpty()
            if (callId.isNotBlank()) blocks += buildJsonObject {
                put("type", "tool_result")
                put("tool_use_id", callId)
                put("content", message.content)
            }
        } else {
            if (message.role == "assistant") {
                blocks += message.replayItems
                    .filter { it.protocol == ProviderProtocol.ANTHROPIC_MESSAGES }
                    .sortedBy(ProtocolReplayItem::index)
                    .map(ProtocolReplayItem::payload)
            }
            message.contentParts().forEach { part -> when (part) {
                is CanonicalContentPart.Text -> if (part.text.isNotEmpty()) blocks += buildJsonObject {
                    put("type", "text")
                    put("text", part.text)
                }
                is CanonicalContentPart.Document -> blocks += buildJsonObject {
                        put("type", "text")
                        put("text", "[Document: ${part.fileName}]\n${part.text}")
                    }
                is CanonicalContentPart.Image -> if (message.role == "user") blocks += buildJsonObject {
                        put("type", "image")
                        putJsonObject("source") {
                            put("type", "base64")
                            put("media_type", part.mimeType)
                            put("data", part.base64)
                        }
                    }
            } }
            message.toolCalls.forEach { call -> blocks += buildJsonObject {
                    put("type", "tool_use")
                    put("id", call.id)
                    put("name", call.name)
                    put("input", runCatching { DirectApiTransport.defaultJson.parseToJsonElement(call.arguments) }
                        .getOrDefault(JsonObject(emptyMap())))
                }
            }
        }
        if (blocks.isNotEmpty()) {
            if (turns.lastOrNull()?.first == role) turns.last().second += blocks
            else turns += role to blocks
        }
    }
    turns.forEach { (role, content) ->
        add(buildJsonObject {
            put("role", role)
            putJsonArray("content") {
                content.forEach(::add)
            }
        })
    }
}

private class AnthropicReplayAccumulator(private val initial: JsonObject) {
    private val thinking = StringBuilder(initial.string("thinking"))
    private val signature = StringBuilder(initial.string("signature"))

    fun appendThinking(value: String) {
        thinking.append(value)
    }

    fun appendSignature(value: String) {
        signature.append(value)
    }

    fun payload(): JsonObject {
        if (initial.string("type") != "thinking") return initial
        return JsonObject(initial.toMutableMap().apply {
            put("thinking", JsonPrimitive(thinking.toString()))
            put("signature", JsonPrimitive(signature.toString()))
        })
    }
}

private fun openAITool(tool: ToolDefinition, nestedFunction: Boolean): JsonObject {
    val function = buildJsonObject {
        put("name", tool.name)
        put("description", tool.description)
        put("parameters", tool.parameters)
        if (!nestedFunction) put("strict", false)
    }
    return if (nestedFunction) buildJsonObject { put("type", "function"); put("function", function) }
    else buildJsonObject {
        put("type", "function")
        function.forEach { (key, value) -> put(key, value) }
    }
}

private fun flattenedContent(parts: List<CanonicalContentPart>): String = parts.joinToString("\n\n") { part ->
    when (part) {
        is CanonicalContentPart.Text -> part.text
        is CanonicalContentPart.Document -> "[Document: ${part.fileName}]\n${part.text}"
        is CanonicalContentPart.Image -> "[Image]"
    }
}

private fun thinkingBudget(effort: String, maxTokens: Int): Int {
    if (effort == "off" || maxTokens < 1025) return 0
    val requested = when (effort) { "low" -> 1024; "high" -> 3072; else -> 2048 }
    return requested.coerceAtMost(maxTokens - 1)
}

private fun openAIUsage(value: JsonObject) = Usage(
    inputTokens = value.long("prompt_tokens"),
    outputTokens = value.long("completion_tokens"),
    cacheReadTokens = value.obj("prompt_tokens_details").long("cached_tokens"),
)

private fun responsesUsage(value: JsonObject) = Usage(
    inputTokens = value.long("input_tokens"),
    outputTokens = value.long("output_tokens"),
    cacheReadTokens = value.obj("input_tokens_details").long("cached_tokens"),
)

private fun anthropicUsage(value: JsonObject) = Usage(
    inputTokens = value.long("input_tokens"),
    outputTokens = value.long("output_tokens"),
    cacheReadTokens = value.long("cache_read_input_tokens"),
    cacheCreationTokens = value.long("cache_creation_input_tokens"),
)

private fun jsonObject(raw: String, json: Json): JsonObject = runCatching {
    json.parseToJsonElement(raw).jsonObject
}.getOrElse { throw IOException("Provider returned a malformed stream event", it) }

private fun streamError(value: JsonElement): ApiException {
    val error = value as? JsonObject ?: JsonObject(emptyMap())
    return ApiException(
        status = 0,
        code = error.string("code"),
        message = error.string("message").ifBlank { "Provider response failed" },
    )
}

private fun JsonObject.string(name: String): String =
    (this[name] as? JsonPrimitive)?.contentOrNull.orEmpty()

private fun JsonObject.int(name: String): Int =
    (this[name] as? JsonPrimitive)?.intOrNull ?: 0

private fun JsonObject.long(name: String): Long =
    (this[name] as? JsonPrimitive)?.longOrNull ?: 0

private fun JsonObject.obj(name: String): JsonObject = this[name] as? JsonObject ?: JsonObject(emptyMap())

private fun JsonObject.array(name: String): JsonArray = this[name] as? JsonArray ?: JsonArray(emptyList())
