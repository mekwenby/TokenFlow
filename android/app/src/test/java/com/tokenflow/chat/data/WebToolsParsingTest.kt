package com.tokenflow.chat.data

import java.io.IOException
import okio.Buffer
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class WebToolsParsingTest {
    @Test
    fun extractsArticleTextAndDropsScriptsNavigationAndForms() {
        val html = """
            <html><head><title>Example</title><script>steal()</script></head>
            <body><nav>Menu</nav><main><article><h1>Heading</h1><p>${"content ".repeat(30)}</p></article></main><form>secret</form></body></html>
        """.trimIndent()

        val result = extractReadableHtml(html, "https://example.com/page")

        assertTrue(result.startsWith("Example\n\nHeading"))
        assertFalse(result.contains("steal"))
        assertFalse(result.contains("Menu"))
        assertFalse(result.contains("secret"))
        assertFalse(shouldUseRenderedFallback(true, result))
    }

    @Test
    fun onlyShortHtmlUsesIsolatedWebViewFallback() {
        assertTrue(shouldUseRenderedFallback(true, "short"))
        assertFalse(shouldUseRenderedFallback(false, "short"))
        assertFalse(shouldUseRenderedFallback(true, "x".repeat(200)))
    }

    @Test
    fun boundedUrlReadAcceptsResponsesShorterThanLimit() {
        val expected = "short response".encodeToByteArray()

        assertArrayEquals(expected, Buffer().write(expected).readUrlBytes(2 * 1024 * 1024L))
    }

    @Test
    fun boundedUrlReadRejectsResponsesLargerThanLimit() {
        val source = Buffer().write(ByteArray(33))

        assertThrows(IOException::class.java) { source.readUrlBytes(32) }
    }
}
