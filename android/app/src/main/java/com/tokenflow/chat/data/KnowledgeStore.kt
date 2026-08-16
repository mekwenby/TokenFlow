package com.tokenflow.chat.data

import android.content.Context
import android.net.Uri
import com.tom_roush.pdfbox.android.PDFBoxResourceLoader
import com.tom_roush.pdfbox.pdmodel.PDDocument
import com.tom_roush.pdfbox.text.PDFTextStripper
import java.io.File
import java.util.Locale
import java.util.UUID
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class KnowledgeStore(
    context: Context,
    private val dao: LocalDao,
) {
    private val appContext = context.applicationContext
    private val directory = File(appContext.filesDir, "knowledge").also(File::mkdirs)

    init {
        PDFBoxResourceLoader.init(appContext)
    }

    suspend fun import(source: KnowledgeImportSource): KnowledgeDocument = withContext(Dispatchers.IO) {
        val name = source.displayName.trim().ifBlank { "document" }
        val extension = name.substringAfterLast('.', "").lowercase(Locale.ROOT)
        require(extension in SUPPORTED_EXTENSIONS) { "Supported files: TXT, Markdown, JSON, CSV and PDF" }
        require(source.sizeBytes < 0 || source.sizeBytes <= MAX_FILE_BYTES) { "File exceeds the 20 MiB limit" }

        val now = System.currentTimeMillis()
        val id = UUID.randomUUID().toString()
        val destination = File(directory, "$id.${extension.ifBlank { "txt" }}")
        var entity = KnowledgeDocumentEntity(
            id = id,
            name = name,
            mimeType = source.mimeType,
            storedPath = destination.absolutePath,
            sizeBytes = source.sizeBytes.coerceAtLeast(0),
            status = "indexing",
            error = "",
            chunkCount = 0,
            createdAt = now,
            updatedAt = now,
        )
        dao.putKnowledgeDocument(entity)
        try {
            val copied = appContext.contentResolver.openInputStream(Uri.parse(source.uri))?.use { input ->
                destination.outputStream().use { output ->
                    val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                    var total = 0L
                    while (true) {
                        val count = input.read(buffer)
                        if (count < 0) break
                        total += count
                        require(total <= MAX_FILE_BYTES) { "File exceeds the 20 MiB limit" }
                        output.write(buffer, 0, count)
                    }
                    total
                }
            } ?: error("Unable to read the selected file")
            val text = extract(destination, extension).take(MAX_TEXT_CHARS)
            require(text.isNotBlank()) { "No readable text was found" }
            val pieces = chunk(text)
            dao.replaceKnowledgeChunks(
                id,
                pieces.mapIndexed { index, value ->
                    KnowledgeChunkEntity(documentId = id, position = index, text = value, searchText = searchable(value))
                },
            )
            entity = entity.copy(
                sizeBytes = copied,
                status = "ready",
                chunkCount = pieces.size,
                updatedAt = System.currentTimeMillis(),
            )
            dao.putKnowledgeDocument(entity)
            entity.toDomain()
        } catch (error: Throwable) {
            entity = entity.copy(
                status = "error",
                error = error.message.orEmpty().ifBlank { "Indexing failed" }.take(500),
                updatedAt = System.currentTimeMillis(),
            )
            dao.putKnowledgeDocument(entity)
            entity.toDomain()
        }
    }

    suspend fun delete(id: String) = withContext(Dispatchers.IO) {
        val entity = dao.knowledgeDocument(id) ?: return@withContext
        dao.replaceKnowledgeChunks(id, emptyList())
        dao.deleteKnowledgeDocument(id)
        File(entity.storedPath).takeIf(File::exists)?.delete()
    }

    suspend fun snippets(ids: List<Long>): List<KnowledgeSnippet> {
        if (ids.isEmpty()) return emptyList()
        return dao.knowledgeChunks(ids.distinct()).mapNotNull { chunk ->
            dao.knowledgeDocument(chunk.documentId)?.let { document -> chunk.toSnippet(document, 0) }
        }.sortedBy { ids.indexOf(it.chunkId) }
    }

    suspend fun search(query: String, limit: Int = 5): List<KnowledgeSnippet> {
        val terms = tokenize(query).distinct().take(12)
        if (terms.isEmpty()) return emptyList()
        val fts = terms.joinToString(" OR ") { "\"${it.replace("\"", "\"\"")}\"" }
        return dao.searchKnowledgeChunks(fts, 40).mapNotNull { chunk ->
            val document = dao.knowledgeDocument(chunk.documentId) ?: return@mapNotNull null
            val normalized = chunk.text.lowercase(Locale.ROOT)
            val score = terms.sumOf { term ->
                var index = normalized.indexOf(term)
                var count = 0
                while (index >= 0) {
                    count += 1
                    index = normalized.indexOf(term, index + term.length)
                }
                count
            } + if (normalized.contains(query.trim().lowercase(Locale.ROOT))) 8 else 0
            chunk.toSnippet(document, score)
        }.sortedWith(compareByDescending<KnowledgeSnippet> { it.score }.thenBy { it.documentName }.thenBy { it.position })
            .take(limit.coerceIn(1, 5))
    }

    private fun extract(file: File, extension: String): String = if (extension == "pdf") {
        PDDocument.load(file).use { document ->
            require(document.numberOfPages <= MAX_PDF_PAGES) { "PDF exceeds the 500 page limit" }
            PDFTextStripper().getText(document)
        }
    } else {
        file.inputStream().bufferedReader(Charsets.UTF_8).use { it.readText() }
    }

    companion object {
        const val MAX_FILE_BYTES = 20L * 1024 * 1024
        const val MAX_PDF_PAGES = 500
        const val MAX_TEXT_CHARS = 2_000_000
        private const val CHUNK_SIZE = 1_200
        private const val CHUNK_OVERLAP = 200
        private val SUPPORTED_EXTENSIONS = setOf("txt", "md", "markdown", "json", "csv", "pdf")

        internal fun chunk(raw: String): List<String> {
            val normalized = raw.replace("\r\n", "\n").replace('\r', '\n').trim()
            if (normalized.isEmpty()) return emptyList()
            val result = mutableListOf<String>()
            var start = 0
            while (start < normalized.length) {
                var end = minOf(start + CHUNK_SIZE, normalized.length)
                if (end < normalized.length) {
                    val paragraph = normalized.lastIndexOf("\n\n", end)
                    if (paragraph > start + CHUNK_SIZE / 2) end = paragraph
                }
                result += normalized.substring(start, end).trim()
                if (end == normalized.length) break
                start = (end - CHUNK_OVERLAP).coerceAtLeast(start + 1)
            }
            return result.filter(String::isNotBlank)
        }

        internal fun searchable(value: String): String = tokenize(value).distinct().joinToString(" ")

        internal fun tokenize(value: String): List<String> {
            val lowered = value.lowercase(Locale.ROOT)
            val latin = Regex("[\\p{L}\\p{N}_-]{2,}").findAll(lowered)
                .map(MatchResult::value)
                .filter { token -> token.none(::isCjk) }
                .toList()
            val cjkRuns = Regex("[\\u3400-\\u4dbf\\u4e00-\\u9fff]+").findAll(lowered).map(MatchResult::value)
            val bigrams = cjkRuns.flatMap { run ->
                when (run.length) {
                    0 -> emptySequence()
                    1 -> sequenceOf(run)
                    else -> (0 until run.length - 1).asSequence().map { run.substring(it, it + 2) }
                }
            }.toList()
            return latin + bigrams
        }

        private fun isCjk(char: Char) = char.code in 0x3400..0x4DBF || char.code in 0x4E00..0x9FFF
    }
}

private fun KnowledgeChunkEntity.toSnippet(document: KnowledgeDocumentEntity, score: Int) = KnowledgeSnippet(
    chunkId = id,
    documentId = documentId,
    documentName = document.name,
    position = position,
    text = text,
    score = score,
)
