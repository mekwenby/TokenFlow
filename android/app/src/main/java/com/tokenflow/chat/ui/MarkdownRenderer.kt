package com.tokenflow.chat.ui

import android.annotation.SuppressLint
import android.graphics.Color as AndroidColor
import android.os.Build
import android.view.ViewGroup
import android.webkit.SafeBrowsingResponse
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.ClickableText
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.BaselineShift
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.unit.em
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.window.Dialog
import com.tokenflow.chat.R
import java.net.URI
import org.commonmark.ext.gfm.tables.TableBlock
import org.commonmark.ext.gfm.tables.TableCell
import org.commonmark.ext.gfm.tables.TableRow
import org.commonmark.ext.gfm.tables.TablesExtension
import org.commonmark.node.BlockQuote
import org.commonmark.node.BulletList
import org.commonmark.node.Code
import org.commonmark.node.Emphasis
import org.commonmark.node.FencedCodeBlock
import org.commonmark.node.HardLineBreak
import org.commonmark.node.Heading
import org.commonmark.node.HtmlBlock
import org.commonmark.node.HtmlInline
import org.commonmark.node.Image
import org.commonmark.node.IndentedCodeBlock
import org.commonmark.node.Link
import org.commonmark.node.ListItem
import org.commonmark.node.Node
import org.commonmark.node.OrderedList
import org.commonmark.node.Paragraph
import org.commonmark.node.SoftLineBreak
import org.commonmark.node.StrongEmphasis
import org.commonmark.node.Text as MarkdownText
import org.commonmark.node.ThematicBreak
import org.commonmark.parser.Parser

private val markdownParser: Parser = Parser.builder()
    .extensions(listOf(TablesExtension.create()))
    .build()

@Composable
fun MarkdownContent(markdown: String, modifier: Modifier = Modifier) {
    val document = remember(markdown) { markdownParser.parse(markdown) }
    SelectionContainer {
        Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(8.dp)) {
            var child = document.firstChild
            while (child != null) {
                RenderBlock(child)
                child = child.next
            }
        }
    }
}

@Composable
private fun RenderBlock(node: Node, modifier: Modifier = Modifier) {
    when (node) {
        is Paragraph -> InlineNodeText(node, modifier)
        is Heading -> InlineNodeText(
            node,
            modifier,
            MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.SemiBold),
        )
        is FencedCodeBlock -> CodeBlock(node.literal, node.info)
        is IndentedCodeBlock -> CodeBlock(node.literal, "")
        is HtmlBlock -> CodeBlock(node.literal, "html")
        is BulletList -> RenderList(node, ordered = false)
        is OrderedList -> RenderList(node, ordered = true)
        is BlockQuote -> Row(
            modifier = modifier
                .fillMaxWidth()
                .border(0.dp, Color.Transparent)
                .padding(start = 10.dp),
        ) {
            Box(Modifier.width(3.dp).fillMaxHeight().background(MaterialTheme.colorScheme.primary))
            Column(Modifier.padding(start = 10.dp)) { RenderChildren(node) }
        }
        is TableBlock -> MarkdownTable(node)
        is ThematicBreak -> Spacer(
            modifier.fillMaxWidth().height(1.dp).background(MaterialTheme.colorScheme.outline.copy(alpha = 0.35f)),
        )
        else -> RenderChildren(node)
    }
}

@Composable
private fun RenderChildren(node: Node) {
    var child = node.firstChild
    while (child != null) {
        RenderBlock(child)
        child = child.next
    }
}

@Composable
private fun RenderList(node: Node, ordered: Boolean) {
    var item = node.firstChild
    var index = if (node is OrderedList) node.markerStartNumber else 1
    Column(verticalArrangement = Arrangement.spacedBy(5.dp)) {
        while (item != null) {
            if (item is ListItem) {
                Row(Modifier.fillMaxWidth()) {
                    Text(if (ordered) "${index++}." else "•", modifier = Modifier.width(24.dp))
                    Column(Modifier.weight(1f)) { RenderChildren(item) }
                }
            }
            item = item.next
        }
    }
}

@Composable
private fun InlineNodeText(
    node: Node,
    modifier: Modifier = Modifier,
    style: androidx.compose.ui.text.TextStyle = MaterialTheme.typography.bodyMedium,
) {
    val darkTheme = isSystemInDarkTheme()
    val annotated = remember(node, darkTheme) { inlineAnnotatedString(node, darkTheme) }
    val uriHandler = LocalUriHandler.current
    @Suppress("DEPRECATION")
    ClickableText(
        text = annotated,
        modifier = modifier.fillMaxWidth(),
        style = style.copy(color = MaterialTheme.colorScheme.onSurface, lineHeight = 21.sp),
        onClick = { offset ->
            annotated.getStringAnnotations("URL", offset, offset).firstOrNull()?.item?.let { url ->
                if (isSafeHttpUrl(url)) runCatching { uriHandler.openUri(url) }
            }
        },
    )
}

internal fun inlineAnnotatedString(root: Node, darkTheme: Boolean = false): AnnotatedString = buildAnnotatedString {
    data class HtmlFrame(val tag: String, val popCount: Int, val suppresses: Boolean = false)
    val htmlFrames = mutableListOf<HtmlFrame>()
    var suppressed = 0

    fun applyHtmlToken(literal: String) {
        val token = parseHtmlToken(literal) ?: return
        if (token.tag in DANGEROUS_INLINE_TAGS) {
            if (token.closing) {
                val index = htmlFrames.indexOfLast { it.tag == token.tag && it.suppresses }
                if (index >= 0) {
                    while (htmlFrames.lastIndex >= index) {
                        val frame = htmlFrames.removeAt(htmlFrames.lastIndex)
                        if (frame.suppresses) suppressed = (suppressed - 1).coerceAtLeast(0)
                        repeat(frame.popCount) { pop() }
                    }
                }
            } else if (!token.selfClosing) {
                suppressed++
                htmlFrames += HtmlFrame(token.tag, 0, true)
            }
            return
        }
        if (suppressed > 0) return
        if (token.tag == "br" && !token.closing) {
            append('\n')
            return
        }
        if (token.tag !in SAFE_INLINE_TAGS) return
        if (token.closing) {
            val index = htmlFrames.indexOfLast { it.tag == token.tag }
            if (index >= 0) while (htmlFrames.lastIndex >= index) {
                val frame = htmlFrames.removeAt(htmlFrames.lastIndex)
                repeat(frame.popCount) { pop() }
            }
            return
        }
        var pops = 0
        if (token.tag == "a") {
            val href = token.attributes["href"].orEmpty()
            if (isSafeHttpUrl(href)) {
                pushStringAnnotation("URL", href); pops++
                pushStyle(SpanStyle(color = if (darkTheme) Color(0xFF7ED3C1) else Color(0xFF087F8C), textDecoration = TextDecoration.Underline)); pops++
            }
        } else {
            htmlSpanStyle(token, darkTheme)?.let { pushStyle(it); pops++ }
        }
        if (token.selfClosing) repeat(pops) { pop() } else htmlFrames += HtmlFrame(token.tag, pops)
    }

    fun appendNode(node: Node) {
        when (node) {
            is MarkdownText -> if (suppressed == 0) append(node.literal)
            is Code -> {
                pushStyle(SpanStyle(fontFamily = FontFamily.Monospace, background = Color(0x1A607D78)))
                append(node.literal)
                pop()
            }
            is StrongEmphasis -> {
                pushStyle(SpanStyle(fontWeight = FontWeight.Bold))
                appendChildren(node, ::appendNode)
                pop()
            }
            is Emphasis -> {
                pushStyle(SpanStyle(fontStyle = FontStyle.Italic))
                appendChildren(node, ::appendNode)
                pop()
            }
            is Link -> {
                val safe = isSafeHttpUrl(node.destination)
                if (safe) {
                    pushStringAnnotation("URL", node.destination)
                    pushStyle(SpanStyle(color = Color(0xFF087F8C), textDecoration = TextDecoration.Underline))
                }
                appendChildren(node, ::appendNode)
                if (safe) {
                    pop()
                    pop()
                }
            }
            is Image -> {
                if (node.firstChild != null) {
                    appendChildren(node, ::appendNode)
                } else {
                    append(node.title.orEmpty().ifBlank { node.destination.orEmpty() })
                }
            }
            is SoftLineBreak, is HardLineBreak -> append('\n')
            is HtmlInline -> applyHtmlToken(node.literal)
            else -> appendChildren(node, ::appendNode)
        }
    }
    appendChildren(root, ::appendNode)
    while (htmlFrames.isNotEmpty()) repeat(htmlFrames.removeAt(htmlFrames.lastIndex).popCount) { pop() }
}

private data class HtmlToken(
    val tag: String,
    val closing: Boolean,
    val selfClosing: Boolean,
    val attributes: Map<String, String>,
)

private fun parseHtmlToken(value: String): HtmlToken? {
    val match = Regex("^<\\s*(/)?\\s*([a-zA-Z0-9]+)([^>]*)>$").matchEntire(value.trim()) ?: return null
    val tail = match.groupValues[3]
    val attributes = Regex("([a-zA-Z_:][-a-zA-Z0-9_:.]*)\\s*=\\s*(?:\"([^\"]*)\"|'([^']*)'|([^\\s>]+))")
        .findAll(tail)
        .associate { attr ->
            attr.groupValues[1].lowercase() to attr.groupValues.drop(2).firstOrNull(String::isNotEmpty).orEmpty()
        }
        .filterKeys { !it.startsWith("on") }
    return HtmlToken(
        tag = match.groupValues[2].lowercase(),
        closing = match.groupValues[1].isNotEmpty(),
        selfClosing = tail.trimEnd().endsWith('/'),
        attributes = attributes,
    )
}

private fun htmlSpanStyle(token: HtmlToken, darkTheme: Boolean): SpanStyle? {
    var weight: FontWeight? = null
    var fontStyle: FontStyle? = null
    var decoration: TextDecoration? = null
    var foreground: Color? = null
    var background: Color? = null
    var sizeScale: Float? = null
    var baselineShift: BaselineShift? = null
    when (token.tag) {
        "strong", "b" -> weight = FontWeight.Bold
        "em", "i" -> fontStyle = FontStyle.Italic
        "u" -> decoration = TextDecoration.Underline
        "s", "del" -> decoration = TextDecoration.LineThrough
        "small" -> sizeScale = 0.85f
        "sub" -> { sizeScale = 0.75f; baselineShift = BaselineShift.Subscript }
        "sup" -> { sizeScale = 0.75f; baselineShift = BaselineShift.Superscript }
        "code" -> return SpanStyle(fontFamily = FontFamily.Monospace, background = Color(0x1A607D78))
        "mark" -> background = if (darkTheme) Color(0xFF725E16) else Color(0xFFFFE58A)
    }
    if (token.tag == "span") token.attributes["style"].orEmpty().split(';').forEach { declaration ->
        val name = declaration.substringBefore(':', "").trim().lowercase()
        val value = declaration.substringAfter(':', "").trim().lowercase()
        when (name) {
            "color" -> foreground = safeCssColor(value, darkTheme, background = false)
            "background", "background-color" -> background = safeCssColor(value, darkTheme, background = true)
            "font-weight" -> if (value == "bold" || value.toIntOrNull()?.let { it >= 600 } == true) weight = FontWeight.Bold
            "font-style" -> if (value == "italic") fontStyle = FontStyle.Italic
            "text-decoration" -> decoration = when {
                "underline" in value -> TextDecoration.Underline
                "line-through" in value -> TextDecoration.LineThrough
                else -> decoration
            }
            "font-size" -> value.removeSuffix("em").toFloatOrNull()?.coerceIn(0.75f, 1.25f)?.let { sizeScale = it }
        }
    }
    if (weight == null && fontStyle == null && decoration == null && foreground == null && background == null && sizeScale == null && baselineShift == null) return null
    return SpanStyle(
        color = foreground ?: Color.Unspecified,
        background = background ?: Color.Unspecified,
        fontWeight = weight,
        fontStyle = fontStyle,
        textDecoration = decoration,
        fontSize = sizeScale?.em ?: androidx.compose.ui.unit.TextUnit.Unspecified,
        baselineShift = baselineShift,
    )
}

private fun safeCssColor(value: String, darkTheme: Boolean, background: Boolean): Color? {
    val named = mapOf(
        "black" to 0xFF000000, "white" to 0xFFFFFFFF, "red" to 0xFFD32F2F, "green" to 0xFF2E7D32,
        "blue" to 0xFF1565C0, "yellow" to 0xFFFFD54F, "gray" to 0xFF6B7471, "grey" to 0xFF6B7471,
    )
    val raw = named[value] ?: when {
        Regex("^#[0-9a-f]{6}$").matches(value) -> 0xFF000000 or value.drop(1).toLong(16)
        Regex("^#[0-9a-f]{8}$").matches(value) -> value.drop(1).toLong(16)
        else -> return null
    }
    val color = Color(raw)
    if (background) return color.copy(alpha = color.alpha.coerceAtMost(0.7f))
    val luminance = color.red * 0.2126f + color.green * 0.7152f + color.blue * 0.0722f
    return when {
        darkTheme && luminance < 0.35f -> Color(0xFFB8E4DA)
        !darkTheme && luminance > 0.82f -> Color(0xFF24534D)
        else -> color
    }
}

private val SAFE_INLINE_TAGS = setOf("span", "mark", "strong", "b", "small", "em", "i", "u", "s", "del", "sub", "sup", "code", "a")
private val DANGEROUS_INLINE_TAGS = setOf("script", "style", "iframe", "object", "embed", "form", "input", "button")

private fun appendChildren(node: Node, append: (Node) -> Unit) {
    var child = node.firstChild
    while (child != null) {
        append(child)
        child = child.next
    }
}

@Composable
private fun CodeBlock(code: String, rawLanguage: String) {
    val language = normalizeCodeLanguage(rawLanguage)
    val darkTheme = MaterialTheme.colorScheme.background.luminance() < 0.5f
    val clipboard = LocalClipboardManager.current
    var preview by remember { mutableStateOf(false) }
    var copied by remember { mutableStateOf(false) }
    Surface(
        modifier = Modifier.fillMaxWidth().border(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.35f), RoundedCornerShape(6.dp)),
        shape = RoundedCornerShape(6.dp),
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.35f),
    ) {
        Column {
            Row(
                Modifier.fillMaxWidth().padding(start = 12.dp, end = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(language.ifBlank { stringResource(R.string.code) }, style = MaterialTheme.typography.labelMedium, modifier = Modifier.weight(1f))
                if (language in setOf("html", "htm")) {
                    TextButton(onClick = { preview = true }) { Text(stringResource(R.string.preview)) }
                }
                IconButton(onClick = {
                    clipboard.setText(AnnotatedString(code))
                    copied = true
                }) {
                    Icon(Icons.Outlined.ContentCopy, contentDescription = stringResource(R.string.copy))
                }
            }
            if (copied) Text(
                stringResource(R.string.copied),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(horizontal = 12.dp),
            )
            Text(
                text = remember(code, language, darkTheme) { highlightedCode(code, language, darkTheme) },
                modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(12.dp),
                fontFamily = FontFamily.Monospace,
                fontSize = 12.sp,
                lineHeight = 18.sp,
            )
        }
    }
    if (preview) HtmlPreviewDialog(code, onDismiss = { preview = false })
}

internal enum class SyntaxKind {
    KEYWORD,
    STRING,
    COMMENT,
    NUMBER,
    CONSTANT,
    TYPE,
    FUNCTION,
    ANNOTATION,
    TAG,
    ATTRIBUTE,
    PROPERTY,
    OPERATOR,
}

internal data class SyntaxToken(
    val start: Int,
    val endExclusive: Int,
    val kind: SyntaxKind,
)

private data class CodeLanguageSpec(
    val keywords: Set<String> = emptySet(),
    val types: Set<String> = emptySet(),
    val lineComments: List<String> = emptyList(),
    val blockComments: List<Pair<String, String>> = emptyList(),
    val quoteChars: Set<Char> = setOf('"', '\''),
    val tripleQuotes: Boolean = false,
    val caseInsensitive: Boolean = false,
    val capitalizedTypes: Boolean = false,
    val propertyKeys: Boolean = false,
    val hexColors: Boolean = false,
)

private data class SyntaxPalette(
    val keyword: Color,
    val string: Color,
    val comment: Color,
    val number: Color,
    val constant: Color,
    val type: Color,
    val function: Color,
    val annotation: Color,
    val tag: Color,
    val attribute: Color,
    val property: Color,
    val operator: Color,
)

private val lightSyntaxPalette = SyntaxPalette(
    keyword = Color(0xFF7138A8),
    string = Color(0xFF26734D),
    comment = Color(0xFF687572),
    number = Color(0xFF0B6FA4),
    constant = Color(0xFFA33C00),
    type = Color(0xFF00796B),
    function = Color(0xFF955200),
    annotation = Color(0xFF9A3D87),
    tag = Color(0xFFB3261E),
    attribute = Color(0xFF725C00),
    property = Color(0xFF006A6A),
    operator = Color(0xFF555F5C),
)

private val darkSyntaxPalette = SyntaxPalette(
    keyword = Color(0xFFD0B0FF),
    string = Color(0xFF8BD5A7),
    comment = Color(0xFF9AA9A5),
    number = Color(0xFF82C7F3),
    constant = Color(0xFFFFB68A),
    type = Color(0xFF78DCC5),
    function = Color(0xFFFFB77C),
    annotation = Color(0xFFF0A8DF),
    tag = Color(0xFFFFB4AB),
    attribute = Color(0xFFEAC36A),
    property = Color(0xFF79D6D1),
    operator = Color(0xFFC6CFCC),
)

internal fun normalizeCodeLanguage(rawLanguage: String): String {
    val raw = rawLanguage.trim()
        .substringBefore(' ')
        .substringBefore('\t')
        .lowercase()
        .trim('{', '}', '.')
    return when (raw) {
        "kt", "kts" -> "kotlin"
        "js", "jsx", "mjs", "cjs", "node" -> "javascript"
        "ts", "tsx" -> "typescript"
        "py", "py3", "python3" -> "python"
        "golang" -> "go"
        "rs" -> "rust"
        "c++", "cc", "cxx", "hpp", "hxx" -> "cpp"
        "h" -> "c"
        "c#", "cs", "csharp" -> "csharp"
        "sh", "bash", "zsh", "shell", "console" -> "shell"
        "ps1", "pwsh", "powershell" -> "powershell"
        "htm" -> "html"
        "svg" -> "xml"
        "scss", "sass", "less" -> "css"
        "yml" -> "yaml"
        "md", "mdx" -> "markdown"
        "postgres", "postgresql", "mysql", "sqlite" -> "sql"
        "rb" -> "ruby"
        "text", "txt", "plaintext", "plain", "none" -> "plain"
        else -> raw
    }
}

internal fun highlightedCode(
    code: String,
    rawLanguage: String,
    darkTheme: Boolean = false,
): AnnotatedString = buildAnnotatedString {
    append(code)
    val palette = if (darkTheme) darkSyntaxPalette else lightSyntaxPalette
    syntaxTokens(code, rawLanguage).forEach { token ->
        val color = when (token.kind) {
            SyntaxKind.KEYWORD -> palette.keyword
            SyntaxKind.STRING -> palette.string
            SyntaxKind.COMMENT -> palette.comment
            SyntaxKind.NUMBER -> palette.number
            SyntaxKind.CONSTANT -> palette.constant
            SyntaxKind.TYPE -> palette.type
            SyntaxKind.FUNCTION -> palette.function
            SyntaxKind.ANNOTATION -> palette.annotation
            SyntaxKind.TAG -> palette.tag
            SyntaxKind.ATTRIBUTE -> palette.attribute
            SyntaxKind.PROPERTY -> palette.property
            SyntaxKind.OPERATOR -> palette.operator
        }
        addStyle(
            SpanStyle(
                color = color,
                fontWeight = when (token.kind) {
                    SyntaxKind.KEYWORD, SyntaxKind.TYPE, SyntaxKind.TAG -> FontWeight.SemiBold
                    else -> null
                },
                fontStyle = if (token.kind == SyntaxKind.COMMENT) FontStyle.Italic else null,
            ),
            token.start,
            token.endExclusive,
        )
    }
}

internal fun syntaxTokens(code: String, rawLanguage: String): List<SyntaxToken> {
    if (code.isEmpty()) return emptyList()
    return when (val language = normalizeCodeLanguage(rawLanguage)) {
        "plain", "" -> emptyList()
        "html", "xml" -> tokenizeMarkup(code)
        "markdown" -> tokenizeMarkdown(code)
        else -> tokenizeCode(code, languageSpec(language))
    }
}

private fun tokenizeCode(code: String, spec: CodeLanguageSpec): List<SyntaxToken> {
    val result = mutableListOf<SyntaxToken>()
    var cursor = 0
    while (cursor < code.length) {
        val blockComment = spec.blockComments.firstOrNull { code.startsWith(it.first, cursor) }
        if (blockComment != null) {
            val end = code.indexOf(blockComment.second, cursor + blockComment.first.length)
                .let { if (it < 0) code.length else it + blockComment.second.length }
            result += SyntaxToken(cursor, end, SyntaxKind.COMMENT)
            cursor = end
            continue
        }

        val lineComment = spec.lineComments.firstOrNull { marker ->
            code.startsWith(marker, cursor) && (marker != "#" || cursor == 0 || code[cursor - 1] != '$')
        }
        if (lineComment != null) {
            val end = code.indexOf('\n', cursor).let { if (it < 0) code.length else it }
            result += SyntaxToken(cursor, end, SyntaxKind.COMMENT)
            cursor = end
            continue
        }

        val quoteLength = when {
            spec.tripleQuotes && code.startsWith("\"\"\"", cursor) -> 3
            spec.tripleQuotes && code.startsWith("'''", cursor) -> 3
            code[cursor] in spec.quoteChars -> 1
            else -> 0
        }
        if (quoteLength > 0) {
            val end = scanQuotedValue(code, cursor, quoteLength)
            val kind = if (spec.propertyKeys && nextNonWhitespace(code, end) == ':') SyntaxKind.PROPERTY else SyntaxKind.STRING
            result += SyntaxToken(cursor, end, kind)
            cursor = end
            continue
        }

        if (spec.hexColors && code[cursor] == '#') {
            val end = (cursor + 1 until code.length)
                .takeWhile { code[it].isDigit() || code[it].lowercaseChar() in 'a'..'f' }
                .lastOrNull()?.plus(1) ?: cursor + 1
            if (end - cursor in setOf(4, 5, 7, 9)) {
                result += SyntaxToken(cursor, end, SyntaxKind.CONSTANT)
                cursor = end
                continue
            }
        }

        if (code[cursor].isDigit() || (code[cursor] == '.' && code.getOrNull(cursor + 1)?.isDigit() == true)) {
            val end = scanNumber(code, cursor)
            result += SyntaxToken(cursor, end, SyntaxKind.NUMBER)
            cursor = end
            continue
        }

        if (code[cursor] == '@' && code.getOrNull(cursor + 1)?.let(::isIdentifierStart) == true) {
            val end = scanIdentifier(code, cursor + 1)
            result += SyntaxToken(cursor, end, SyntaxKind.ANNOTATION)
            cursor = end
            continue
        }

        if (isIdentifierStart(code[cursor])) {
            val end = scanIdentifier(code, cursor)
            val word = code.substring(cursor, end)
            val comparable = if (spec.caseInsensitive) word.lowercase() else word
            val kind = when {
                comparable in spec.keywords -> SyntaxKind.KEYWORD
                comparable in LANGUAGE_CONSTANTS -> SyntaxKind.CONSTANT
                comparable in spec.types -> SyntaxKind.TYPE
                spec.propertyKeys && nextNonWhitespace(code, end) == ':' -> SyntaxKind.PROPERTY
                spec.capitalizedTypes && word.firstOrNull()?.isUpperCase() == true && word.any(Char::isLowerCase) -> SyntaxKind.TYPE
                nextNonWhitespace(code, end) == '(' -> SyntaxKind.FUNCTION
                else -> null
            }
            if (kind != null) result += SyntaxToken(cursor, end, kind)
            cursor = end
            continue
        }

        if (code[cursor] in CODE_OPERATORS) {
            val start = cursor
            while (cursor < code.length && code[cursor] in CODE_OPERATORS) cursor++
            result += SyntaxToken(start, cursor, SyntaxKind.OPERATOR)
            continue
        }
        cursor++
    }
    return result
}

private fun tokenizeMarkup(code: String): List<SyntaxToken> {
    val result = mutableListOf<SyntaxToken>()
    var cursor = 0
    while (cursor < code.length) {
        if (code.startsWith("<!--", cursor)) {
            val end = code.indexOf("-->", cursor + 4).let { if (it < 0) code.length else it + 3 }
            result += SyntaxToken(cursor, end, SyntaxKind.COMMENT)
            cursor = end
            continue
        }
        if (code[cursor] != '<') {
            if (code[cursor] == '&') {
                val end = code.indexOf(';', cursor + 1)
                if (end in (cursor + 2)..(cursor + 12)) {
                    result += SyntaxToken(cursor, end + 1, SyntaxKind.CONSTANT)
                    cursor = end + 1
                    continue
                }
            }
            cursor++
            continue
        }

        val openingEnd = when {
            code.startsWith("</", cursor) -> cursor + 2
            code.startsWith("<!", cursor) || code.startsWith("<?", cursor) -> cursor + 2
            else -> cursor + 1
        }
        result += SyntaxToken(cursor, openingEnd, SyntaxKind.OPERATOR)
        cursor = openingEnd
        while (cursor < code.length && code[cursor].isWhitespace()) cursor++
        val tagEnd = scanMarkupName(code, cursor)
        if (tagEnd > cursor) {
            result += SyntaxToken(cursor, tagEnd, SyntaxKind.TAG)
            cursor = tagEnd
        }
        while (cursor < code.length && code[cursor] != '>') {
            if (code.startsWith("/>", cursor) || code.startsWith("?>", cursor)) {
                result += SyntaxToken(cursor, cursor + 2, SyntaxKind.OPERATOR)
                cursor += 2
                break
            }
            if (code[cursor].isWhitespace()) {
                cursor++
                continue
            }
            if (code[cursor] == '"' || code[cursor] == '\'') {
                val end = scanQuotedValue(code, cursor, 1)
                result += SyntaxToken(cursor, end, SyntaxKind.STRING)
                cursor = end
                continue
            }
            if (code[cursor] == '=') {
                result += SyntaxToken(cursor, cursor + 1, SyntaxKind.OPERATOR)
                cursor++
                continue
            }
            val attributeEnd = scanMarkupName(code, cursor)
            if (attributeEnd > cursor) {
                result += SyntaxToken(cursor, attributeEnd, SyntaxKind.ATTRIBUTE)
                cursor = attributeEnd
            } else {
                cursor++
            }
        }
        if (cursor < code.length && code[cursor] == '>') {
            result += SyntaxToken(cursor, cursor + 1, SyntaxKind.OPERATOR)
            cursor++
        }
    }
    return result
}

private fun tokenizeMarkdown(code: String): List<SyntaxToken> {
    val result = mutableListOf<SyntaxToken>()
    var cursor = 0
    var lineStart = true
    while (cursor < code.length) {
        if (lineStart) {
            val markerEnd = when {
                code[cursor] == '#' -> (cursor until code.length).takeWhile { code[it] == '#' }.lastOrNull()?.plus(1)
                code.startsWith("> ", cursor) -> cursor + 1
                code.startsWith("- ", cursor) || code.startsWith("* ", cursor) -> cursor + 1
                else -> null
            }
            if (markerEnd != null) {
                result += SyntaxToken(cursor, markerEnd, SyntaxKind.KEYWORD)
                cursor = markerEnd
                lineStart = false
                continue
            }
        }
        if (code[cursor] == '`') {
            val tickCount = (cursor until code.length).takeWhile { code[it] == '`' }.count().coerceAtLeast(1)
            val marker = "`".repeat(tickCount)
            val closing = code.indexOf(marker, cursor + tickCount)
            val end = if (closing < 0) code.length else closing + tickCount
            result += SyntaxToken(cursor, end, SyntaxKind.STRING)
            lineStart = code.substring(cursor, end).endsWith('\n')
            cursor = end
            continue
        }
        if (code[cursor] in setOf('*', '_', '~') || code.startsWith("[", cursor) || code.startsWith("](", cursor)) {
            val end = if (code.startsWith("](", cursor)) cursor + 2 else cursor + 1
            result += SyntaxToken(cursor, end, SyntaxKind.OPERATOR)
            cursor = end
            lineStart = false
            continue
        }
        lineStart = code[cursor] == '\n'
        cursor++
    }
    return result
}

private fun scanQuotedValue(code: String, start: Int, delimiterLength: Int): Int {
    val delimiter = code.substring(start, (start + delimiterLength).coerceAtMost(code.length))
    var cursor = start + delimiterLength
    while (cursor < code.length) {
        if (code.startsWith(delimiter, cursor)) {
            if (delimiterLength == 1 && cursor + 1 < code.length && code[cursor + 1] == delimiter[0]) {
                cursor += 2
                continue
            }
            return cursor + delimiterLength
        }
        if (code[cursor] == '\\' && cursor + 1 < code.length) cursor += 2 else cursor++
    }
    return code.length
}

private fun scanNumber(code: String, start: Int): Int {
    var cursor = start
    if (code.startsWith("0x", cursor, ignoreCase = true) || code.startsWith("0b", cursor, ignoreCase = true) || code.startsWith("0o", cursor, ignoreCase = true)) {
        cursor += 2
        while (cursor < code.length && (code[cursor].isLetterOrDigit() || code[cursor] == '_')) cursor++
        return cursor
    }
    while (cursor < code.length && (code[cursor].isDigit() || code[cursor] == '_')) cursor++
    if (cursor < code.length && code[cursor] == '.') {
        cursor++
        while (cursor < code.length && (code[cursor].isDigit() || code[cursor] == '_')) cursor++
    }
    if (cursor < code.length && code[cursor].lowercaseChar() == 'e') {
        cursor++
        if (cursor < code.length && code[cursor] in setOf('+', '-')) cursor++
        while (cursor < code.length && (code[cursor].isDigit() || code[cursor] == '_')) cursor++
    }
    while (cursor < code.length && code[cursor].lowercaseChar() in setOf('f', 'd', 'l', 'u')) cursor++
    return cursor
}

private fun scanIdentifier(code: String, start: Int): Int {
    var cursor = start
    while (cursor < code.length && isIdentifierPart(code[cursor])) cursor++
    return cursor
}

private fun scanMarkupName(code: String, start: Int): Int {
    var cursor = start
    while (cursor < code.length && (code[cursor].isLetterOrDigit() || code[cursor] in setOf('_', '-', ':', '.', '@'))) cursor++
    return cursor
}

private fun nextNonWhitespace(code: String, start: Int): Char? {
    var cursor = start
    while (cursor < code.length && code[cursor].isWhitespace()) cursor++
    return code.getOrNull(cursor)
}

private fun isIdentifierStart(char: Char): Boolean = char.isLetter() || char == '_' || char == '$'

private fun isIdentifierPart(char: Char): Boolean = char.isLetterOrDigit() || char == '_' || char == '$'

private fun languageSpec(language: String): CodeLanguageSpec = when (language) {
    "kotlin" -> cStyleSpec(
        keywords = words("as break by catch class companion constructor continue data delegate do dynamic else enum expect external false field file finally for fun get if import in infix init inline inner interface internal is lateinit noinline null object open operator out override package private protected public reified return sealed set suspend super tailrec this throw true try typealias typeof val var vararg when where while actual value"),
        types = words("Any Boolean Byte Char Double Float Int Long Nothing Number Short String Unit UInt ULong UByte UShort List MutableList Map MutableMap Set MutableSet Array Sequence Flow"),
        capitalizedTypes = true,
        tripleQuotes = true,
    )
    "java" -> cStyleSpec(
        keywords = words("abstract assert boolean break byte case catch char class const continue default do double else enum exports extends final finally float for goto if implements import instanceof int interface long module native new non-sealed open opens package permits private protected provides public record requires return sealed short static strictfp super switch synchronized this throw throws to transient transitive try uses var void volatile while with yield"),
        types = words("Boolean Byte Character Class Double Exception Float Integer Long Number Object Short String StringBuilder Throwable Void List Map Set Optional Stream"),
        capitalizedTypes = true,
    )
    "javascript" -> cStyleSpec(
        keywords = words("as async await break case catch class const continue debugger default delete do else export extends finally for from function get if implements import in instanceof interface let new of package private protected public return set static super switch this throw try typeof undefined var void while with yield"),
        types = words("Array BigInt Boolean Date Error Function Map Number Object Promise RegExp Set String Symbol WeakMap WeakSet"),
        quoteChars = setOf('"', '\'', '`'),
        capitalizedTypes = true,
    )
    "typescript" -> cStyleSpec(
        keywords = words("abstract any as asserts async await bigint boolean break case catch class const constructor continue debugger declare default delete do else enum export extends false finally for from function get if implements import in infer instanceof interface is keyof let module namespace never new null number object of package private protected public readonly require return set static string super switch symbol this throw true try type typeof undefined unique unknown var void while with yield"),
        types = words("Array Date Error Function Map Promise Record Set String Number Boolean Partial Pick Omit Readonly Required"),
        quoteChars = setOf('"', '\'', '`'),
        capitalizedTypes = true,
    )
    "python" -> CodeLanguageSpec(
        keywords = words("and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield match case"),
        types = words("bool bytes bytearray complex dict float frozenset int list memoryview object range set str tuple type Exception None"),
        lineComments = listOf("#"),
        quoteChars = setOf('"', '\''),
        tripleQuotes = true,
        capitalizedTypes = true,
    )
    "go" -> cStyleSpec(
        keywords = words("break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var"),
        types = words("any bool byte complex64 complex128 error float32 float64 int int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr"),
        quoteChars = setOf('"', '\'', '`'),
        capitalizedTypes = true,
    )
    "rust" -> cStyleSpec(
        keywords = words("as async await break const continue crate dyn else enum extern false fn for if impl in let loop match mod move mut pub ref return self Self static struct super trait true type unsafe use where while abstract become box do final macro override priv typeof unsized virtual yield"),
        types = words("bool char f32 f64 i8 i16 i32 i64 i128 isize str String u8 u16 u32 u64 u128 usize Vec Option Result Box Arc Rc"),
        capitalizedTypes = true,
    )
    "c", "cpp" -> cStyleSpec(
        keywords = words("alignas alignof asm auto break case catch class const constexpr consteval constinit const_cast continue co_await co_return co_yield decltype default delete do dynamic_cast else enum explicit export extern false for friend goto if inline mutable namespace new noexcept nullptr operator private protected public register reinterpret_cast requires return sizeof static static_assert static_cast struct switch template this thread_local throw true try typedef typeid typename union unsigned using virtual void volatile while"),
        types = words("bool char char8_t char16_t char32_t double float int int8_t int16_t int32_t int64_t long short signed size_t string uint8_t uint16_t uint32_t uint64_t vector map set optional variant"),
        capitalizedTypes = true,
    )
    "csharp" -> cStyleSpec(
        keywords = words("abstract as async await base break case catch checked class const continue decimal default delegate do else enum event explicit extern false finally fixed for foreach from get global goto if implicit in init interface internal into is join let lock namespace new not null object on operator orderby out override params partial private protected public readonly record ref remove required return sealed select set sizeof stackalloc static string struct switch this throw true try typeof unchecked unmanaged unsafe using value var virtual void volatile when where while with yield"),
        types = words("bool byte char DateTime decimal double dynamic Exception float Guid int long object sbyte short string uint ulong ushort Task List Dictionary HashSet IEnumerable"),
        capitalizedTypes = true,
    )
    "swift" -> cStyleSpec(
        keywords = words("associatedtype break case catch class continue convenience default defer deinit didSet do dynamic else enum extension fallthrough false fileprivate final for func get guard if import in indirect infix init inout internal is lazy let mutating nil nonmutating open operator optional override postfix precedence prefix private protocol public repeat required rethrows return self Self set static struct subscript super switch throw throws true try typealias unowned var weak where while willSet"),
        types = words("Any Array Bool Character Data Dictionary Double Error Float Int Optional Result Set String UInt URL UUID Void"),
        capitalizedTypes = true,
    )
    "dart" -> cStyleSpec(
        keywords = words("abstract as assert async await break case catch class const continue covariant default deferred do dynamic else enum export extends extension external factory false final finally for Function get hide if implements import in interface is late library mixin new null on operator part required rethrow return set show static super switch sync this throw true try typedef var void while with yield"),
        types = words("bool double Future int Iterable List Map Never Null num Object Record Set Stream String Symbol Type Uri"),
        capitalizedTypes = true,
    )
    "json" -> CodeLanguageSpec(propertyKeys = true)
    "yaml" -> CodeLanguageSpec(
        keywords = words("true false null yes no on off anchor alias"),
        lineComments = listOf("#"),
        propertyKeys = true,
        caseInsensitive = true,
    )
    "toml" -> CodeLanguageSpec(
        lineComments = listOf("#"),
        propertyKeys = true,
        tripleQuotes = true,
        caseInsensitive = true,
    )
    "css" -> CodeLanguageSpec(
        keywords = words("important inherit initial revert unset auto none normal block inline flex grid relative absolute fixed sticky solid dashed transparent currentcolor media supports keyframes import font-face"),
        blockComments = listOf("/*" to "*/"),
        quoteChars = setOf('"', '\''),
        propertyKeys = true,
        caseInsensitive = true,
        hexColors = true,
    )
    "sql" -> CodeLanguageSpec(
        keywords = words("add all alter analyze and any as asc begin between by case check column commit constraint create cross database default delete desc distinct drop else end except exists explain false fetch foreign from full grant group having if in index inner insert intersect into is join key left like limit merge natural not null offset on or order outer primary references returning revoke right rollback row select set table then true truncate union unique update using values view when where with"),
        types = words("bigint binary bit blob boolean char date datetime decimal double float int integer json numeric real smallint text time timestamp uuid varchar"),
        lineComments = listOf("--"),
        blockComments = listOf("/*" to "*/"),
        quoteChars = setOf('"', '\'', '`'),
        caseInsensitive = true,
    )
    "shell" -> CodeLanguageSpec(
        keywords = words("break case continue do done elif else esac eval exec exit export fi for function if in local readonly return set shift source then time trap typeset until while"),
        types = emptySet(),
        lineComments = listOf("#"),
        quoteChars = setOf('"', '\'', '`'),
    )
    "powershell" -> CodeLanguageSpec(
        keywords = words("begin break catch class continue data define do dynamicparam else elseif end enum exit filter finally for foreach from function hidden if in param process return static switch throw trap try until using var while workflow"),
        lineComments = listOf("#"),
        blockComments = listOf("<#" to "#>"),
        quoteChars = setOf('"', '\''),
        caseInsensitive = true,
        capitalizedTypes = true,
    )
    "ruby" -> CodeLanguageSpec(
        keywords = words("alias and begin break case class def defined do else elsif end ensure false for if in module next nil not or redo rescue retry return self super then true undef unless until when while yield"),
        lineComments = listOf("#"),
        quoteChars = setOf('"', '\'', '`'),
        capitalizedTypes = true,
    )
    "php" -> cStyleSpec(
        keywords = words("abstract and array as break callable case catch class clone const continue declare default do echo else elseif empty enddeclare endfor endforeach endif endswitch endwhile eval exit extends final finally fn for foreach function global goto if implements include include_once instanceof insteadof interface isset list match namespace new or print private protected public readonly require require_once return static switch throw trait try unset use var while xor yield"),
        types = words("bool boolean float int integer mixed never null object string void iterable self parent static"),
        capitalizedTypes = true,
    )
    else -> cStyleSpec(
        keywords = words("break case catch class const continue default else enum false for function if import in interface let new null package return struct switch this throw true try type val var while"),
        quoteChars = setOf('"', '\'', '`'),
        capitalizedTypes = true,
    )
}

private fun cStyleSpec(
    keywords: Set<String>,
    types: Set<String> = emptySet(),
    quoteChars: Set<Char> = setOf('"', '\''),
    capitalizedTypes: Boolean = false,
    tripleQuotes: Boolean = false,
): CodeLanguageSpec = CodeLanguageSpec(
    keywords = keywords,
    types = types,
    lineComments = listOf("//"),
    blockComments = listOf("/*" to "*/"),
    quoteChars = quoteChars,
    tripleQuotes = tripleQuotes,
    capitalizedTypes = capitalizedTypes,
)

private fun words(value: String): Set<String> = value.split(' ').filter(String::isNotBlank).toSet()

private val LANGUAGE_CONSTANTS = words("true false null nil none undefined nan infinity")

private val CODE_OPERATORS = setOf(
    '+', '-', '*', '/', '%', '=', '!', '<', '>', '&', '|', '^', '~', '?', ':', '.', ',', ';',
    '(', ')', '[', ']', '{', '}',
)

@Composable
private fun MarkdownTable(table: TableBlock) {
    val rows = remember(table) { tableRows(table) }
    Column(
        Modifier.fillMaxWidth().horizontalScroll(rememberScrollState())
            .border(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.35f)),
    ) {
        rows.forEachIndexed { rowIndex, row ->
            Row {
                children<TableCell>(row).forEach { cell ->
                    Box(
                        Modifier.width(160.dp)
                            .border(0.5.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.25f))
                            .padding(8.dp),
                    ) {
                        InlineNodeText(
                            cell,
                            style = MaterialTheme.typography.bodySmall.copy(
                                fontWeight = if (rowIndex == 0) FontWeight.SemiBold else FontWeight.Normal,
                            ),
                        )
                    }
                }
            }
        }
    }
}

private inline fun <reified T : Node> children(node: Node): List<T> {
    val result = mutableListOf<T>()
    var child = node.firstChild
    while (child != null) {
        if (child is T) result += child
        child = child.next
    }
    return result
}

private fun tableRows(node: Node): List<TableRow> {
    val result = mutableListOf<TableRow>()
    fun visit(current: Node) {
        var child = current.firstChild
        while (child != null) {
            if (child is TableRow) result += child
            visit(child)
            child = child.next
        }
    }
    visit(node)
    return result
}

internal fun isSafeHttpUrl(value: String): Boolean = runCatching {
    val uri = URI(value)
    (uri.scheme == "https" || uri.scheme == "http") && !uri.host.isNullOrBlank()
}.getOrDefault(false)

@SuppressLint("SetJavaScriptEnabled")
@Composable
private fun HtmlPreviewDialog(source: String, onDismiss: () -> Unit) {
    val context = LocalContext.current
    var reloadKey by remember { mutableIntStateOf(0) }
    val document = remember(source, reloadKey) { previewDocument(source) }
    val webView = remember {
        WebView(context).apply {
            setBackgroundColor(AndroidColor.TRANSPARENT)
            layoutParams = ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT)
            settings.javaScriptEnabled = true
            settings.domStorageEnabled = false
            settings.allowFileAccess = false
            settings.allowContentAccess = false
            settings.allowFileAccessFromFileURLs = false
            settings.allowUniversalAccessFromFileURLs = false
            settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
            webViewClient = object : WebViewClient() {
                override fun shouldOverrideUrlLoading(view: WebView?, request: WebResourceRequest?) = true
                @Deprecated("Deprecated in Android")
                override fun shouldOverrideUrlLoading(view: WebView?, url: String?) = true
                override fun onSafeBrowsingHit(view: WebView?, request: WebResourceRequest?, threatType: Int, callback: SafeBrowsingResponse?) {
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O_MR1) {
                        callback?.backToSafety(true)
                    }
                }
                override fun shouldInterceptRequest(view: WebView?, request: WebResourceRequest?): WebResourceResponse? {
                    if (request?.url?.scheme in setOf("file", "content")) {
                        return WebResourceResponse("text/plain", "UTF-8", null)
                    }
                    return super.shouldInterceptRequest(view, request)
                }
            }
        }
    }
    DisposableEffect(webView) {
        onDispose {
            webView.stopLoading()
            webView.loadUrl("about:blank")
            webView.destroy()
        }
    }
    Dialog(onDismissRequest = onDismiss) {
        Surface(
            modifier = Modifier.fillMaxWidth().fillMaxHeight(0.92f),
            shape = RoundedCornerShape(8.dp),
        ) {
            Column {
                Row(
                    Modifier.fillMaxWidth().padding(start = 16.dp, end = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(stringResource(R.string.html_preview), style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f))
                    IconButton(onClick = { reloadKey++ }) { Icon(Icons.Outlined.Refresh, stringResource(R.string.reload)) }
                    IconButton(onClick = onDismiss) { Icon(Icons.Outlined.Close, stringResource(R.string.close)) }
                }
                AndroidView(
                    factory = { webView },
                    modifier = Modifier.fillMaxSize(),
                    update = { it.loadDataWithBaseURL("about:blank", document, "text/html", "UTF-8", null) },
                )
            }
        }
    }
}

internal const val HTML_PREVIEW_CSP = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline' https:; img-src https: data: blob:; font-src https: data:; media-src https: data: blob:; connect-src 'none'; frame-src 'none'; worker-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"

internal fun previewDocument(source: String): String {
    val metadata = "<meta http-equiv=\"Content-Security-Policy\" content=\"$HTML_PREVIEW_CSP\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">"
    val head = Regex("<head(?:\\s[^>]*)?>", RegexOption.IGNORE_CASE)
    val html = Regex("<html(?:\\s[^>]*)?>", RegexOption.IGNORE_CASE)
    return when {
        head.containsMatchIn(source) -> head.find(source)!!.let { source.replaceRange(it.range, "${it.value}$metadata") }
        html.containsMatchIn(source) -> html.find(source)!!.let { source.replaceRange(it.range, "${it.value}<head>$metadata</head>") }
        else -> "<!doctype html><html><head>$metadata</head><body>${source.replace(Regex("^\\s*<!doctype[^>]*>", RegexOption.IGNORE_CASE), "")}</body></html>"
    }
}
