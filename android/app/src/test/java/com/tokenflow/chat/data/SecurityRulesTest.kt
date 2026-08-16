package com.tokenflow.chat.data

import java.net.InetAddress
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class SecurityRulesTest {
    @Test
    fun providerUrlsRequireCleanHttpsRoots() {
        assertEquals("https://api.example.com/v1", ProviderValidator.normalizeBaseUrl("https://api.example.com/v1/"))
        assertThrows(ConfigurationException::class.java) { ProviderValidator.normalizeBaseUrl("http://api.example.com/v1") }
        assertThrows(ConfigurationException::class.java) { ProviderValidator.normalizeBaseUrl("https://user@example.com/v1") }
        assertThrows(ConfigurationException::class.java) { ProviderValidator.normalizeBaseUrl("https://api.example.com/v1?q=1") }
        assertThrows(ConfigurationException::class.java) { ProviderValidator.normalizeBaseUrl("https://api.example.com/v1#fragment") }
    }

    @Test
    fun urlReaderRejectsPrivateReservedAndNonStandardPorts() {
        listOf("127.0.0.1", "10.0.0.1", "169.254.1.2", "192.168.1.2", "203.0.113.1", "::1", "fc00::1").forEach {
            assertFalse(it, SafeUrlValidator.isPublicAddress(InetAddress.getByName(it)))
        }
        assertTrue(SafeUrlValidator.isPublicAddress(InetAddress.getByName("8.8.8.8")))
        assertThrows(ConfigurationException::class.java) { SafeUrlValidator.parseAndResolve("https://example.com:8443/page") }
        assertThrows(ConfigurationException::class.java) { SafeUrlValidator.parseAndResolve("https://user@example.com/page") }
        assertThrows(ConfigurationException::class.java) { SafeUrlValidator.parseAndResolve("http://example.com/page") }
    }

    @Test
    fun promptLibraryContainsTwelveStableRolesAndRuntimeSafetyInstructions() {
        assertEquals(12, SystemPrompts.templates.size)
        assertEquals(12, SystemPrompts.templates.map { it.id }.distinct().size)
        val prompt = SystemPrompts.compose(
            customPrompt = "Be a developer",
            nickname = "User",
            enableSearch = true,
            enableRead = true,
            timeZone = "Asia/Shanghai",
        )
        assertTrue(prompt.contains("Asia/Shanghai"))
        assertTrue(prompt.contains("web_search"))
        assertTrue(prompt.contains("read_url"))
        assertTrue(prompt.contains("untrusted data"))
    }
}
