package com.tokenflow.chat.data

import java.security.GeneralSecurityException
import java.security.SecureRandom
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.SecretKeyFactory
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.PBEKeySpec
import javax.crypto.spec.SecretKeySpec
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

@Serializable
data class ConfigArchivePayload(
    val format: String = PAYLOAD_FORMAT,
    val version: Int = ARCHIVE_VERSION,
    val createdAt: Long,
    val providers: List<ConfigProviderRecord>,
    val models: List<ModelProfile>,
    val defaultModelId: String? = null,
    val exaApiKey: String? = null,
    val mimoTtsApiKey: String? = null,
    val mimoTtsVoice: String? = null,
    val visionFallbackModelId: String? = null,
    val infoFlowApiKey: String? = null,
    val globalSystemPrompt: String? = null,
    val globalUrlReaderBackend: UrlReaderBackend? = null,
    val agents: List<AgentProfile> = emptyList(),
) {
    companion object {
        const val PAYLOAD_FORMAT = "tokenflow-local-config"
        const val ARCHIVE_VERSION = 1
    }
}

@Serializable
data class ConfigProviderRecord(val provider: ProviderConfig, val apiKey: String)

@Serializable
private data class ConfigArchiveEnvelope(
    val format: String = "tokenflow-encrypted-config",
    val version: Int = 1,
    val kdf: String = "PBKDF2-HMAC-SHA256",
    val iterations: Int,
    val salt: String,
    val cipher: String = "AES-256-GCM",
    val iv: String,
    val ciphertext: String,
)

class ConfigArchiveException(message: String, cause: Throwable? = null) : Exception(message, cause)

class ConfigArchiveCodec(private val json: Json = defaultJson) {
    fun encode(payload: ConfigArchivePayload, password: CharArray): String {
        require(password.size >= MIN_PASSWORD_LENGTH) { "Password must contain at least $MIN_PASSWORD_LENGTH characters" }
        val salt = ByteArray(SALT_SIZE).also(random::nextBytes)
        val iv = ByteArray(IV_SIZE).also(random::nextBytes)
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, deriveKey(password, salt, ITERATIONS), GCMParameterSpec(128, iv))
        cipher.updateAAD(AAD)
        val encrypted = cipher.doFinal(json.encodeToString(payload).encodeToByteArray())
        return json.encodeToString(
            ConfigArchiveEnvelope(
                iterations = ITERATIONS,
                salt = encoder.encodeToString(salt),
                iv = encoder.encodeToString(iv),
                ciphertext = encoder.encodeToString(encrypted),
            ),
        )
    }

    fun decode(raw: String, password: CharArray): ConfigArchivePayload {
        try {
            val envelope = json.decodeFromString<ConfigArchiveEnvelope>(raw)
            require(envelope.format == "tokenflow-encrypted-config" && envelope.version == 1)
            require(envelope.kdf == "PBKDF2-HMAC-SHA256" && envelope.cipher == "AES-256-GCM")
            require(envelope.iterations in 100_000..2_000_000)
            val salt = decoder.decode(envelope.salt)
            val iv = decoder.decode(envelope.iv)
            require(salt.size == SALT_SIZE && iv.size == IV_SIZE)
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.DECRYPT_MODE, deriveKey(password, salt, envelope.iterations), GCMParameterSpec(128, iv))
            cipher.updateAAD(AAD)
            val payload = json.decodeFromString<ConfigArchivePayload>(
                cipher.doFinal(decoder.decode(envelope.ciphertext)).decodeToString(),
            )
            require(payload.format == ConfigArchivePayload.PAYLOAD_FORMAT)
            require(payload.version == ConfigArchivePayload.ARCHIVE_VERSION)
            return payload
        } catch (error: Throwable) {
            if (error is ConfigArchiveException) throw error
            throw ConfigArchiveException("The configuration file or password is invalid", error)
        }
    }

    private fun deriveKey(password: CharArray, salt: ByteArray, iterations: Int): SecretKeySpec {
        val spec = PBEKeySpec(password, salt, iterations, 256)
        return try {
            SecretKeySpec(SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256").generateSecret(spec).encoded, "AES")
        } catch (error: GeneralSecurityException) {
            throw ConfigArchiveException("Unable to derive the archive key", error)
        } finally {
            spec.clearPassword()
        }
    }

    companion object {
        const val MIN_PASSWORD_LENGTH = 10
        const val ITERATIONS = 600_000
        private const val SALT_SIZE = 16
        private const val IV_SIZE = 12
        private const val TRANSFORMATION = "AES/GCM/NoPadding"
        private val AAD = "tokenflow-encrypted-config:1".encodeToByteArray()
        private val random = SecureRandom()
        private val encoder = Base64.getEncoder()
        private val decoder = Base64.getDecoder()
        val defaultJson = Json { ignoreUnknownKeys = true; explicitNulls = false; encodeDefaults = true }
    }
}
