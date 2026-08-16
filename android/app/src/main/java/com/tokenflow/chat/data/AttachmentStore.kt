package com.tokenflow.chat.data

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Canvas
import android.graphics.Matrix
import android.graphics.Paint
import android.net.Uri
import androidx.exifinterface.media.ExifInterface
import com.tom_roush.pdfbox.pdmodel.PDDocument
import com.tom_roush.pdfbox.text.PDFTextStripper
import com.tom_roush.pdfbox.android.PDFBoxResourceLoader
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.util.Base64
import java.util.UUID
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.apache.poi.hwpf.HWPFDocument
import org.apache.poi.hwpf.extractor.WordExtractor
import org.apache.poi.ss.usermodel.DataFormatter
import org.apache.poi.ss.usermodel.WorkbookFactory
import org.apache.poi.xwpf.usermodel.XWPFDocument

class AttachmentStore(
    context: Context,
    private val dao: LocalDao,
) {
    private val appContext = context.applicationContext
    private val root = File(appContext.filesDir, "chat_attachments").apply { mkdirs() }
    private val cameraDraftRoot = File(appContext.cacheDir, CameraCaptureStore.DIRECTORY_NAME).apply { mkdirs() }

    init {
        PDFBoxResourceLoader.init(appContext)
    }

    suspend fun persist(messageId: String, pending: List<PendingAttachment>): List<MessageAttachment> =
        withContext(Dispatchers.IO) {
            validateSelection(pending)
            val created = mutableListOf<MessageAttachment>()
            try {
                pending.forEachIndexed { index, source ->
                    created += importOne(messageId, source, System.currentTimeMillis() + index)
                    require(created.sumOf(MessageAttachment::sizeBytes) <= MAX_TOTAL_BYTES) {
                        "Attachments exceed the 20 MiB total limit"
                    }
                }
                dao.putAttachments(created.map(MessageAttachment::toEntity))
                created
            } catch (failure: Throwable) {
                created.forEach { File(it.storedPath).delete() }
                throw failure
            }
        }

    suspend fun forMessages(messageIds: List<String>): List<MessageAttachment> =
        if (messageIds.isEmpty()) emptyList() else dao.attachmentsForMessages(messageIds).map(MessageAttachmentEntity::toDomain)

    suspend fun canonicalParts(message: ChatMessage, descriptions: List<String> = emptyList()): List<CanonicalContentPart> =
        withContext(Dispatchers.IO) {
            val attachments = dao.attachmentsForMessage(message.id).map(MessageAttachmentEntity::toDomain)
            buildList {
                if (message.content.isNotBlank()) add(CanonicalContentPart.Text(message.content))
                attachments.filter { it.status == AttachmentStatus.READY }.forEach { attachment ->
                    when (attachment.kind) {
                        AttachmentKind.IMAGE -> if (descriptions.isEmpty()) {
                            val bytes = File(attachment.storedPath).readBytes()
                            add(CanonicalContentPart.Image(attachment.mimeType, Base64.getEncoder().encodeToString(bytes)))
                        }
                        AttachmentKind.DOCUMENT -> if (attachment.extractedText.isNotBlank()) {
                            add(CanonicalContentPart.Document(attachment.fileName, attachment.extractedText))
                        }
                    }
                }
                descriptions.forEachIndexed { index, description ->
                    add(CanonicalContentPart.Text("[Image ${index + 1} description]\n$description"))
                }
            }
        }

    suspend fun hasImages(messageId: String): Boolean =
        dao.attachmentsForMessage(messageId).any { it.kind == AttachmentKind.IMAGE.name && it.status == AttachmentStatus.READY.name }

    suspend fun imageParts(messageId: String): List<CanonicalContentPart.Image> = withContext(Dispatchers.IO) {
        dao.attachmentsForMessage(messageId).filter {
            it.kind == AttachmentKind.IMAGE.name && it.status == AttachmentStatus.READY.name
        }.map { attachment ->
            CanonicalContentPart.Image(
                attachment.mimeType,
                Base64.getEncoder().encodeToString(File(attachment.storedPath).readBytes()),
            )
        }
    }

    fun visionTestPart(): CanonicalContentPart.Image {
        val bitmap = Bitmap.createBitmap(640, 240, Bitmap.Config.ARGB_8888)
        Canvas(bitmap).apply {
            drawColor(android.graphics.Color.WHITE)
            drawText("TOKENFLOW 73", 48f, 145f, Paint(Paint.ANTI_ALIAS_FLAG).apply {
                color = android.graphics.Color.BLACK
                textSize = 64f
                typeface = android.graphics.Typeface.DEFAULT_BOLD
            })
        }
        val bytes = ByteArrayOutputStream().use { output ->
            bitmap.compress(Bitmap.CompressFormat.PNG, 100, output)
            output.toByteArray()
        }
        bitmap.recycle()
        return CanonicalContentPart.Image("image/png", Base64.getEncoder().encodeToString(bytes))
    }

    suspend fun copyForBranch(sourceMessageIds: List<String>, messageIdMap: Map<String, String>): List<MessageAttachment> =
        withContext(Dispatchers.IO) {
            val sources = forMessages(sourceMessageIds)
            val copies = mutableListOf<MessageAttachment>()
            try {
                sources.forEach { source ->
                    val targetMessageId = messageIdMap[source.messageId] ?: return@forEach
                    val sourceFile = File(source.storedPath)
                    val extension = sourceFile.extension.takeIf(String::isNotBlank)?.let { ".$it" }.orEmpty()
                    val targetFile = File(root, "${UUID.randomUUID()}$extension")
                    sourceFile.copyTo(targetFile, overwrite = false)
                    copies += source.copy(
                        id = UUID.randomUUID().toString(),
                        messageId = targetMessageId,
                        storedPath = targetFile.absolutePath,
                        createdAt = System.currentTimeMillis(),
                    )
                }
                copies
            } catch (failure: Throwable) {
                copies.forEach { File(it.storedPath).delete() }
                throw failure
            }
        }

    fun deleteFiles(attachments: List<MessageAttachment>) {
        attachments.forEach { attachment ->
            val file = File(attachment.storedPath)
            if (file.parentFile?.canonicalFile == root.canonicalFile) file.delete()
        }
    }

    suspend fun discardPendingDrafts(attachments: List<PendingAttachment>) = withContext(Dispatchers.IO) {
        attachments.filter { it.origin == PendingAttachmentOrigin.CAMERA }.forEach { attachment ->
            runCatching { ownedCameraDraft(attachment).delete() }
        }
    }

    private fun validateSelection(items: List<PendingAttachment>) {
        require(items.size <= MAX_ATTACHMENTS) { "A message can contain at most $MAX_ATTACHMENTS attachments" }
        var total = 0L
        items.forEach { source ->
            val image = isImage(source.mimeType, source.displayName)
            val limit = if (image) MAX_IMAGE_BYTES else MAX_DOCUMENT_BYTES
            require(source.sizeBytes < 0 || source.sizeBytes <= limit) { "${source.displayName} is too large" }
            if (source.sizeBytes > 0) total += source.sizeBytes
        }
        require(total <= MAX_TOTAL_BYTES) { "Attachments exceed the 20 MiB total limit" }
    }

    private fun importOne(messageId: String, source: PendingAttachment, createdAt: Long): MessageAttachment {
        if (source.origin == PendingAttachmentOrigin.CAMERA) {
            return importNormalizedCamera(messageId, source, createdAt)
        }
        val uri = Uri.parse(source.uri)
        val isImage = isImage(source.mimeType, source.displayName)
        val limit = if (isImage) MAX_IMAGE_BYTES else MAX_DOCUMENT_BYTES
        val inputFile = File(root, "${UUID.randomUUID()}.incoming")
        try {
            appContext.contentResolver.openInputStream(uri)?.use { input ->
                FileOutputStream(inputFile).use { output ->
                    val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                    var total = 0L
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) break
                        total += read
                        if (total > limit) throw IOException("${source.displayName} is too large")
                        output.write(buffer, 0, read)
                    }
                }
            } ?: throw IOException("Unable to open ${source.displayName}")
            return if (isImage) importImage(messageId, source, inputFile, createdAt)
            else importDocument(messageId, source, inputFile, createdAt)
        } finally {
            inputFile.delete()
        }
    }

    private fun importNormalizedCamera(
        messageId: String,
        source: PendingAttachment,
        createdAt: Long,
    ): MessageAttachment {
        val input = ownedCameraDraft(source)
        require(input.isFile && input.length() in 1L..MAX_IMAGE_BYTES) { "Camera photo is unavailable or too large" }
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(input.absolutePath, bounds)
        require(bounds.outWidth > 0 && bounds.outHeight > 0 && bounds.outMimeType == "image/jpeg") {
            "Camera photo is not a valid JPEG"
        }
        require(maxOf(bounds.outWidth, bounds.outHeight) <= CameraCaptureStore.MAX_IMAGE_EDGE) {
            "Camera photo dimensions are too large"
        }
        val output = File(root, "${UUID.randomUUID()}.jpg")
        return try {
            input.copyTo(output, overwrite = false)
            MessageAttachment(
                messageId = messageId,
                fileName = source.displayName.substringBeforeLast('.', source.displayName) + ".jpg",
                mimeType = "image/jpeg",
                kind = AttachmentKind.IMAGE,
                storedPath = output.absolutePath,
                sizeBytes = output.length(),
                width = bounds.outWidth,
                height = bounds.outHeight,
                createdAt = createdAt,
            )
        } catch (failure: Throwable) {
            output.delete()
            throw failure
        }
    }

    private fun importImage(messageId: String, source: PendingAttachment, input: File, createdAt: Long): MessageAttachment {
        val raw = BitmapFactory.decodeFile(input.absolutePath) ?: throw IOException("Unsupported image: ${source.displayName}")
        val orientation = runCatching { ExifInterface(input).rotationDegrees }.getOrDefault(0)
        val rotated = if (orientation == 0) raw else Bitmap.createBitmap(
            raw, 0, 0, raw.width, raw.height, Matrix().apply { postRotate(orientation.toFloat()) }, true,
        ).also { if (it !== raw) raw.recycle() }
        val scale = minOf(1f, MAX_IMAGE_EDGE.toFloat() / maxOf(rotated.width, rotated.height))
        val normalized = if (scale < 1f) Bitmap.createScaledBitmap(
            rotated, (rotated.width * scale).toInt(), (rotated.height * scale).toInt(), true,
        ).also { if (it !== rotated) rotated.recycle() } else rotated
        val usePng = normalized.hasAlpha()
        val output = File(root, "${UUID.randomUUID()}.${if (usePng) "png" else "jpg"}")
        FileOutputStream(output).use { stream ->
            check(normalized.compress(if (usePng) Bitmap.CompressFormat.PNG else Bitmap.CompressFormat.JPEG, 90, stream))
        }
        val result = MessageAttachment(
            messageId = messageId,
            fileName = source.displayName,
            mimeType = if (usePng) "image/png" else "image/jpeg",
            kind = AttachmentKind.IMAGE,
            storedPath = output.absolutePath,
            sizeBytes = output.length(),
            width = normalized.width,
            height = normalized.height,
            createdAt = createdAt,
        )
        normalized.recycle()
        return result
    }

    private fun importDocument(messageId: String, source: PendingAttachment, input: File, createdAt: Long): MessageAttachment {
        val extension = source.displayName.substringAfterLast('.', "bin").lowercase()
        require(extension in DOCUMENT_EXTENSIONS) { "Unsupported document type: .$extension" }
        val output = File(root, "${UUID.randomUUID()}.$extension")
        return try {
            input.copyTo(output, overwrite = false)
            val text = extractText(output, extension).take(MAX_DOCUMENT_CHARS)
            MessageAttachment(
                messageId = messageId,
                fileName = source.displayName,
                mimeType = source.mimeType,
                kind = AttachmentKind.DOCUMENT,
                storedPath = output.absolutePath,
                sizeBytes = output.length(),
                extractedText = text,
                status = if (text.isBlank()) AttachmentStatus.FAILED else AttachmentStatus.READY,
                createdAt = createdAt,
            )
        } catch (failure: Throwable) {
            output.delete()
            throw failure
        }
    }

    private fun extractText(file: File, extension: String): String = when (extension) {
        "pdf" -> PDDocument.load(file).use { document ->
            PDFTextStripper().apply { endPage = minOf(document.numberOfPages, MAX_PDF_PAGES) }.getText(document)
        }
        "doc" -> FileInputStream(file).use { input ->
            HWPFDocument(input).use { document -> WordExtractor(document).use { extractor -> extractor.text } }
        }
        "docx" -> FileInputStream(file).use { input -> XWPFDocument(input).use { document ->
            document.paragraphs.joinToString("\n") { it.text }
        } }
        "xls", "xlsx" -> FileInputStream(file).use { input -> WorkbookFactory.create(input).use { workbook ->
            val formatter = DataFormatter()
            workbook.joinToString("\n") { sheet ->
                sheet.joinToString("\n") { row -> row.joinToString("\t") { cell -> formatter.formatCellValue(cell) } }
            }
        } }
        else -> file.readText(Charsets.UTF_8)
    }.trim()

    private fun isImage(mime: String, name: String): Boolean =
        mime.lowercase().startsWith("image/") || name.substringAfterLast('.', "").lowercase() in IMAGE_EXTENSIONS

    private fun ownedCameraDraft(source: PendingAttachment): File {
        val path = requireNotNull(source.appOwnedDraftPath) { "Camera photo draft is unavailable" }
        val file = File(path).canonicalFile
        require(file.parentFile == cameraDraftRoot.canonicalFile) { "Invalid camera photo draft" }
        return file
    }

    companion object {
        const val MAX_ATTACHMENTS = 5
        const val MAX_IMAGE_BYTES = 5L * 1024 * 1024
        const val MAX_DOCUMENT_BYTES = 2L * 1024 * 1024
        const val MAX_TOTAL_BYTES = 20L * 1024 * 1024
        const val MAX_DOCUMENT_CHARS = 100_000
        const val MAX_MESSAGE_DOCUMENT_CHARS = 200_000
        const val MAX_PDF_PAGES = 100
        private const val MAX_IMAGE_EDGE = 4096
        private val IMAGE_EXTENSIONS = setOf("jpg", "jpeg", "png", "webp", "heic", "heif", "gif")
        private val DOCUMENT_EXTENSIONS = setOf(
            "txt", "md", "json", "csv", "xml", "yaml", "yml", "log", "kt", "kts", "java", "go", "py",
            "js", "ts", "tsx", "jsx", "c", "cc", "cpp", "h", "hpp", "cs", "rs", "swift", "sql", "sh",
            "ps1", "html", "css", "toml", "ini", "properties", "gradle", "doc", "docx", "xls", "xlsx", "pdf",
        )
    }
}
