package com.tokenflow.chat.data

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Matrix
import android.net.Uri
import androidx.exifinterface.media.ExifInterface
import java.io.File
import java.io.InputStream
import java.security.MessageDigest
import java.util.UUID
import kotlin.math.min

enum class LocalAvatarKind(internal val prefix: String) {
    USER("user"),
    ASSISTANT("assistant"),
}

data class LocalAvatarFile(
    val path: String,
    val lastModified: Long,
)

data class LocalAvatarImages(
    val user: LocalAvatarFile? = null,
    val assistant: LocalAvatarFile? = null,
)

class LocalAvatarStore(context: Context) {
    private val appContext = context.applicationContext

    init {
        migrateLegacyAccountAvatars()
    }

    fun read(): LocalAvatarImages {
        return readDirectory(globalDirectory())
    }

    fun readConversation(conversationId: String): LocalAvatarImages =
        readDirectory(conversationDirectory(conversationId))

    fun readDraft(): LocalAvatarImages = readDirectory(draftDirectory())

    private fun readDirectory(directory: File): LocalAvatarImages {
        return LocalAvatarImages(
            user = newest(directory, LocalAvatarKind.USER),
            assistant = newest(directory, LocalAvatarKind.ASSISTANT),
        )
    }

    fun save(kind: LocalAvatarKind, uri: Uri): LocalAvatarImages {
        val input = requireNotNull(appContext.contentResolver.openInputStream(uri))
        return input.use { saveTo(globalDirectory(), kind, it) }
    }

    fun saveConversation(conversationId: String, kind: LocalAvatarKind, uri: Uri): LocalAvatarImages {
        val input = requireNotNull(appContext.contentResolver.openInputStream(uri))
        return input.use { saveTo(conversationDirectory(conversationId), kind, it) }
    }

    internal fun saveConversation(conversationId: String, kind: LocalAvatarKind, input: InputStream): LocalAvatarImages =
        saveTo(conversationDirectory(conversationId), kind, input)

    fun saveDraft(kind: LocalAvatarKind, uri: Uri): LocalAvatarImages {
        val input = requireNotNull(appContext.contentResolver.openInputStream(uri))
        return input.use { saveTo(draftDirectory(), kind, it) }
    }

    internal fun saveDraft(kind: LocalAvatarKind, input: InputStream): LocalAvatarImages =
        saveTo(draftDirectory(), kind, input)

    internal fun save(kind: LocalAvatarKind, input: InputStream): LocalAvatarImages {
        return saveTo(globalDirectory(), kind, input)
    }

    private fun saveTo(directory: File, kind: LocalAvatarKind, input: InputStream): LocalAvatarImages {
        val imported = File.createTempFile("avatar-import-", ".image", appContext.cacheDir)
        try {
            imported.outputStream().buffered().use { output ->
                val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                var total = 0L
                while (true) {
                    val count = input.read(buffer)
                    if (count < 0) break
                    total += count
                    require(total <= MAX_SOURCE_BYTES)
                    output.write(buffer, 0, count)
                }
            }

            val normalized = normalize(imported)
            directory.apply { check(mkdirs() || isDirectory) }
            val temporary = File.createTempFile("${kind.prefix}-", ".tmp", directory)
            try {
                temporary.outputStream().buffered().use { output ->
                    check(normalized.compress(Bitmap.CompressFormat.PNG, 100, output))
                }
                val target = File(directory, "${kind.prefix}-${System.currentTimeMillis()}-${UUID.randomUUID()}.png")
                check(temporary.renameTo(target))
                directory.listFiles().orEmpty()
                    .filter { it != target && it.name.startsWith("${kind.prefix}-") }
                    .forEach(File::delete)
            } finally {
                temporary.delete()
                normalized.recycle()
            }
        } finally {
            imported.delete()
        }
        return readDirectory(directory)
    }

    fun remove(kind: LocalAvatarKind): LocalAvatarImages {
        return removeFrom(globalDirectory(), kind)
    }

    fun removeConversation(conversationId: String, kind: LocalAvatarKind): LocalAvatarImages =
        removeFrom(conversationDirectory(conversationId), kind)

    fun removeDraft(kind: LocalAvatarKind): LocalAvatarImages = removeFrom(draftDirectory(), kind)

    private fun removeFrom(directory: File, kind: LocalAvatarKind): LocalAvatarImages {
        directory.listFiles().orEmpty()
            .filter { it.name.startsWith("${kind.prefix}-") }
            .forEach(File::delete)
        if (directory.listFiles().isNullOrEmpty()) directory.delete()
        return readDirectory(directory)
    }

    fun promoteDraft(conversationId: String): LocalAvatarImages {
        val source = draftDirectory()
        val target = conversationDirectory(conversationId)
        if (!source.exists()) return readConversation(conversationId)
        check(target.mkdirs() || target.isDirectory)
        source.listFiles().orEmpty().filter(File::isFile).forEach { file ->
            val destination = File(target, file.name)
            if (!file.renameTo(destination)) {
                file.copyTo(destination, overwrite = true)
                file.delete()
            }
        }
        source.delete()
        return readConversation(conversationId)
    }

    fun deleteConversation(conversationId: String) {
        val directory = conversationDirectory(conversationId)
        directory.listFiles().orEmpty().forEach(File::delete)
        directory.delete()
    }

    fun clearDraft() {
        val directory = draftDirectory()
        directory.listFiles().orEmpty().forEach(File::delete)
        directory.delete()
    }

    private fun normalize(source: File): Bitmap {
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(source.path, bounds)
        require(bounds.outWidth > 0 && bounds.outHeight > 0)

        var sampleSize = 1
        while (bounds.outWidth / sampleSize > MAX_DECODE_SIZE || bounds.outHeight / sampleSize > MAX_DECODE_SIZE) {
            sampleSize *= 2
        }
        val decoded = requireNotNull(
            BitmapFactory.decodeFile(
                source.path,
                BitmapFactory.Options().apply {
                    inSampleSize = sampleSize
                    inPreferredConfig = Bitmap.Config.ARGB_8888
                },
            ),
        )
        val oriented = orient(decoded, source)
        val side = min(oriented.width, oriented.height)
        val cropped = Bitmap.createBitmap(
            oriented,
            (oriented.width - side) / 2,
            (oriented.height - side) / 2,
            side,
            side,
        )
        val scaled = Bitmap.createScaledBitmap(cropped, AVATAR_SIZE, AVATAR_SIZE, true)
        listOf(decoded, oriented, cropped).filter { it !== scaled }.distinct().forEach(Bitmap::recycle)
        return scaled
    }

    private fun orient(bitmap: Bitmap, source: File): Bitmap {
        val orientation = runCatching {
            ExifInterface(source.path).getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL)
        }.getOrDefault(ExifInterface.ORIENTATION_NORMAL)
        val matrix = Matrix()
        when (orientation) {
            ExifInterface.ORIENTATION_FLIP_HORIZONTAL -> matrix.setScale(-1f, 1f)
            ExifInterface.ORIENTATION_ROTATE_180 -> matrix.setRotate(180f)
            ExifInterface.ORIENTATION_FLIP_VERTICAL -> matrix.setScale(1f, -1f)
            ExifInterface.ORIENTATION_TRANSPOSE -> {
                matrix.setRotate(90f)
                matrix.postScale(-1f, 1f)
            }
            ExifInterface.ORIENTATION_ROTATE_90 -> matrix.setRotate(90f)
            ExifInterface.ORIENTATION_TRANSVERSE -> {
                matrix.setRotate(-90f)
                matrix.postScale(-1f, 1f)
            }
            ExifInterface.ORIENTATION_ROTATE_270 -> matrix.setRotate(-90f)
            else -> return bitmap
        }
        return Bitmap.createBitmap(bitmap, 0, 0, bitmap.width, bitmap.height, matrix, true)
    }

    private fun globalDirectory() = File(appContext.filesDir, "avatars/global")

    private fun conversationDirectory(conversationId: String): File {
        require(conversationId.isNotBlank()) { "Invalid conversation ID" }
        val directoryName = runCatching { UUID.fromString(conversationId).toString() }.getOrElse {
            MessageDigest.getInstance("SHA-256").digest(conversationId.encodeToByteArray())
                .joinToString("") { byte -> "%02x".format(byte) }
        }
        return File(appContext.filesDir, "avatars/conversations/$directoryName")
    }

    private fun draftDirectory() = File(appContext.cacheDir, "avatar-drafts/current")

    private fun migrateLegacyAccountAvatars() {
        val target = globalDirectory()
        if (target.listFiles().orEmpty().any { it.isFile }) return
        val root = File(appContext.filesDir, "avatars")
        val source = root.listFiles().orEmpty()
            .filter { it.isDirectory && it.name != "global" && it.name != "conversations" }
            .maxByOrNull { directory -> directory.listFiles().orEmpty().maxOfOrNull(File::lastModified) ?: 0L }
            ?: return
        LocalAvatarKind.entries.forEach { kind ->
            newest(source, kind)?.let { avatar ->
                check(target.mkdirs() || target.isDirectory)
                File(avatar.path).copyTo(
                    File(target, "${kind.prefix}-migrated-${UUID.randomUUID()}.png"),
                    overwrite = false,
                )
            }
        }
    }

    private fun newest(directory: File, kind: LocalAvatarKind): LocalAvatarFile? = directory
        .listFiles()
        .orEmpty()
        .filter { it.isFile && it.name.startsWith("${kind.prefix}-") && it.extension == "png" }
        .maxByOrNull(File::lastModified)
        ?.let { LocalAvatarFile(it.path, it.lastModified()) }

    private companion object {
        const val AVATAR_SIZE = 384
        const val MAX_DECODE_SIZE = 1536
        const val MAX_SOURCE_BYTES = 20L * 1024 * 1024
    }
}
