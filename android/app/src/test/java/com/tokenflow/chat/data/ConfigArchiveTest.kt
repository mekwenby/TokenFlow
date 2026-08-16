package com.tokenflow.chat.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class ConfigArchiveTest {
    private val codec = ConfigArchiveCodec()
    private val payload = ConfigArchivePayload(
        createdAt = 1234L,
        providers = listOf(
            ConfigProviderRecord(
                ProviderConfig("provider-1", "OpenAI", "https://api.openai.com/v1", ProviderProtocol.OPENAI_RESPONSES, true),
                "sk-secret",
            ),
        ),
        models = listOf(ModelProfile("model-1", "provider-1", "gpt-test", "Test", 4096, true)),
        defaultModelId = "model-1",
        exaApiKey = "exa-secret",
        infoFlowApiKey = "infoflow-secret",
        globalSystemPrompt = "global prompt",
        globalUrlReaderBackend = UrlReaderBackend.INFOFLOW,
        agents = listOf(AgentProfile(id = "agent-1", name = "Reviewer", modelId = "model-1", systemPrompt = "Review code")),
    )

    @Test
    fun encryptedArchiveRoundTripsWithoutPlaintextSecrets() {
        val encoded = codec.encode(payload, "correct horse battery".toCharArray())

        assertFalse(encoded.contains("sk-secret"))
        assertFalse(encoded.contains("exa-secret"))
        assertFalse(encoded.contains("infoflow-secret"))
        assertTrue(encoded.contains("PBKDF2-HMAC-SHA256"))
        assertEquals(payload, codec.decode(encoded, "correct horse battery".toCharArray()))
    }

    @Test
    fun wrongPasswordAndTamperingFailBeforeReturningPayload() {
        val encoded = codec.encode(payload, "correct horse battery".toCharArray())

        assertThrows(ConfigArchiveException::class.java) {
            codec.decode(encoded, "incorrect password".toCharArray())
        }
        val tampered = encoded.replace(Regex("\"ciphertext\":\"."), "\"ciphertext\":\"A")
        assertThrows(ConfigArchiveException::class.java) {
            codec.decode(tampered, "correct horse battery".toCharArray())
        }
    }

    @Test
    fun passwordMustContainAtLeastTenCharacters() {
        assertThrows(IllegalArgumentException::class.java) {
            codec.encode(payload, "short".toCharArray())
        }
    }
}
