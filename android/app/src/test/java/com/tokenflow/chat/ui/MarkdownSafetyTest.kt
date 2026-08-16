package com.tokenflow.chat.ui

import org.junit.Assert.assertFalse
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.commonmark.parser.Parser

class MarkdownSafetyTest {
    private val parser = Parser.builder().build()

    @Test
    fun linksOnlyAllowAbsoluteHttpAndHttpsUrls() {
        assertTrue(isSafeHttpUrl("https://example.com/path?q=1"))
        assertTrue(isSafeHttpUrl("http://localhost:8019"))
        assertFalse(isSafeHttpUrl("javascript:alert(1)"))
        assertFalse(isSafeHttpUrl("file:///data/user/0/token"))
        assertFalse(isSafeHttpUrl("//example.com/path"))
        assertFalse(isSafeHttpUrl("https:///missing-host"))
    }

    @Test
    fun previewDocumentInjectsRestrictivePolicyIntoCompleteDocuments() {
        val result = previewDocument("<!doctype html><html><head><title>Demo</title></head><body>ok</body></html>")

        assertTrue(result.contains("<head><meta http-equiv=\"Content-Security-Policy\""))
        assertTrue(result.contains("connect-src 'none'"))
        assertTrue(result.contains("frame-src 'none'"))
        assertTrue(result.contains("form-action 'none'"))
        assertTrue(result.contains("<title>Demo</title>"))
    }

    @Test
    fun previewDocumentWrapsFragmentsAndKeepsScriptsUnderCsp() {
        val result = previewDocument("<script>document.body.textContent='ok'</script>")

        assertTrue(result.startsWith("<!doctype html><html><head>"))
        assertTrue(result.contains(HTML_PREVIEW_CSP))
        assertTrue(result.contains("<body><script>"))
    }

    @Test
    fun imageWithoutOptionalTitleUsesAltTextWithoutCrashing() {
        val document = parser.parse("![Architecture diagram](https://example.com/diagram.png)")

        assertEquals("Architecture diagram", inlineAnnotatedString(document).text)
    }

    @Test
    fun imageWithoutTitleOrAltFallsBackToDestination() {
        val document = parser.parse("![](https://example.com/diagram.png)")

        assertEquals("https://example.com/diagram.png", inlineAnnotatedString(document).text)
    }

    @Test
    fun safeInlineHtmlRendersTextAndFiltersDangerousContent() {
        val document = parser.parse("<mark>marked <strong>bold</strong></mark> <span style=\"color:#ff0000;font-size:3em;position:fixed\">safe</span><script>secret()</script>")

        val rendered = inlineAnnotatedString(document)

        assertEquals("marked bold safe", rendered.text.trim())
        assertFalse(rendered.text.contains("secret"))
        assertTrue(rendered.spanStyles.isNotEmpty())
    }

    @Test
    fun htmlLinksOnlyAnnotateSafeHttpDestinations() {
        val document = parser.parse("<a href=\"https://example.com\">safe</a> <a href=\"javascript:alert(1)\" onclick=\"bad()\">blocked</a>")

        val rendered = inlineAnnotatedString(document)

        assertEquals("safe blocked", rendered.text)
        assertEquals(listOf("https://example.com"), rendered.getStringAnnotations("URL", 0, rendered.length).map { it.item })
    }

    @Test
    fun fencedLanguageAliasesAreNormalized() {
        assertEquals("kotlin", normalizeCodeLanguage("{.KTS}"))
        assertEquals("javascript", normalizeCodeLanguage("jsx title=demo"))
        assertEquals("python", normalizeCodeLanguage("python3"))
        assertEquals("cpp", normalizeCodeLanguage("C++"))
        assertEquals("powershell", normalizeCodeLanguage("pwsh"))
        assertEquals("yaml", normalizeCodeLanguage("yml"))
    }

    @Test
    fun kotlinLexerDoesNotTreatCommentMarkersInsideStringsAsComments() {
        val code = "val endpoint = \"https://example.com/#part\" // request URL"

        val tokens = syntaxTokens(code, "kt")

        assertToken(code, tokens, "val", SyntaxKind.KEYWORD)
        assertToken(code, tokens, "\"https://example.com/#part\"", SyntaxKind.STRING)
        assertToken(code, tokens, "// request URL", SyntaxKind.COMMENT)
        assertEquals(1, tokens.count { it.kind == SyntaxKind.COMMENT })
    }

    @Test
    fun commonLanguageFamiliesProduceSemanticTokens() {
        val python = "def parse(value: str):\n    return value or None # fallback"
        val sql = "SELECT id, name FROM users WHERE id = 42 AND name = 'Ada'; -- one row"
        val json = "{\"enabled\": true, \"limit\": 1200}"

        assertToken(python, syntaxTokens(python, "py"), "def", SyntaxKind.KEYWORD)
        assertToken(python, syntaxTokens(python, "py"), "parse", SyntaxKind.FUNCTION)
        assertToken(python, syntaxTokens(python, "py"), "# fallback", SyntaxKind.COMMENT)
        assertToken(sql, syntaxTokens(sql, "postgresql"), "SELECT", SyntaxKind.KEYWORD)
        assertToken(sql, syntaxTokens(sql, "postgresql"), "42", SyntaxKind.NUMBER)
        assertToken(json, syntaxTokens(json, "json"), "\"enabled\"", SyntaxKind.PROPERTY)
        assertToken(json, syntaxTokens(json, "json"), "true", SyntaxKind.CONSTANT)
    }

    @Test
    fun markupLexerHighlightsTagsAttributesValuesAndComments() {
        val code = "<!-- title --><section data-kind=\"note\"><strong>Hi</strong></section>"
        val tokens = syntaxTokens(code, "html")

        assertToken(code, tokens, "<!-- title -->", SyntaxKind.COMMENT)
        assertToken(code, tokens, "section", SyntaxKind.TAG)
        assertToken(code, tokens, "data-kind", SyntaxKind.ATTRIBUTE)
        assertToken(code, tokens, "\"note\"", SyntaxKind.STRING)
        assertToken(code, tokens, "strong", SyntaxKind.TAG)
    }

    @Test
    fun syntaxPaletteChangesBetweenLightAndDarkThemes() {
        val light = highlightedCode("fun answer() = 42", "kotlin", darkTheme = false)
        val dark = highlightedCode("fun answer() = 42", "kotlin", darkTheme = true)

        assertEquals(light.text, dark.text)
        assertTrue(light.spanStyles.isNotEmpty())
        assertEquals(light.spanStyles.map { it.start to it.end }, dark.spanStyles.map { it.start to it.end })
        assertNotEquals(light.spanStyles.first().item.color, dark.spanStyles.first().item.color)
    }

    @Test
    fun explicitlyPlainCodeRemainsUnstyled() {
        assertTrue(syntaxTokens("val example = 12", "text").isEmpty())
    }

    private fun assertToken(
        code: String,
        tokens: List<SyntaxToken>,
        expectedText: String,
        expectedKind: SyntaxKind,
    ) {
        assertTrue(
            "Expected $expectedKind token for '$expectedText' in $tokens",
            tokens.any { token ->
                token.kind == expectedKind && code.substring(token.start, token.endExclusive) == expectedText
            },
        )
    }
}
