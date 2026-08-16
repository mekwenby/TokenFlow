package com.tokenflow.chat.data

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Matrix
import android.net.Uri
import androidx.core.content.FileProvider
import androidx.exifinterface.media.ExifInterface
import java.io.File
import java.io.FileOutputStream
import java.io.IOException
import java.util.UUID
import kotlin.math.roundToInt
import kotlin.math.sqrt
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

data class CameraCaptureTarget(
    val uri: Uri,
    val path: String,
)

class CameraCaptureStore(context: Context) {
    private val appContext = context.applicationContext
    private val root = File(appContext.cacheDir, DIRECTORY_NAME).apply { mkdirs() }

    init {
        cleanupExpired()
    }

    fun createCapture(): CameraCaptureTarget {
        cleanupExpired()
        val file = File(root, "capture_${UUID.randomUUID()}.jpg")
        check(file.createNewFile()) { "Unable to create a camera capture file" }
        return CameraCaptureTarget(uriFor(file), file.absolutePath)
    }

    suspend fun finishCapture(path: String): PendingAttachment = withContext(Dispatchers.IO) {
        val source = ownedFile(path)
        require(source.isFile && source.length() > 0L) { "The camera did not return a photo" }
        val output = File(root, "draft_${UUID.randomUUID()}.jpg")
        val temporary = File(root, "${output.name}.tmp")
        try {
            val dimensions = normalizeJpeg(source, temporary)
            if (!temporary.renameTo(output)) {
                temporary.copyTo(output, overwrite = false)
                check(temporary.delete()) { "Unable to finish the camera photo" }
            }
            PendingAttachment(
                uri = uriFor(output).toString(),
                displayName = "photo_${System.currentTimeMillis()}.jpg",
                mimeType = "image/jpeg",
                sizeBytes = output.length(),
                origin = PendingAttachmentOrigin.CAMERA,
                appOwnedDraftPath = output.absolutePath,
            ).also {
                check(dimensions.width > 0 && dimensions.height > 0)
            }
        } catch (failure: Throwable) {
            output.delete()
            throw failure
        } finally {
            temporary.delete()
            source.delete()
        }
    }

    fun cancelCapture(path: String?) {
        if (path == null) return
        runCatching { ownedFile(path).delete() }
    }

    fun cleanupExpired(now: Long = System.currentTimeMillis()) {
        val cutoff = now - MAX_DRAFT_AGE_MS
        root.listFiles().orEmpty().filter { it.isFile && it.lastModified() < cutoff }.forEach(File::delete)
    }

    private fun normalizeJpeg(source: File, output: File): ImageDimensions {
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(source.absolutePath, bounds)
        require(bounds.outWidth > 0 && bounds.outHeight > 0) { "Unsupported camera image" }

        var sampleSize = 1
        while (maxOf(bounds.outWidth, bounds.outHeight) / sampleSize > MAX_IMAGE_EDGE) sampleSize *= 2
        var bitmap = BitmapFactory.decodeFile(
            source.absolutePath,
            BitmapFactory.Options().apply {
                inSampleSize = sampleSize
                inPreferredConfig = Bitmap.Config.ARGB_8888
            },
        ) ?: throw IOException("Unsupported camera image")

        val oriented = orient(bitmap, source)
        if (oriented !== bitmap) bitmap.recycle()
        bitmap = oriented

        val edgeScale = minOf(1f, MAX_IMAGE_EDGE.toFloat() / maxOf(bitmap.width, bitmap.height))
        if (edgeScale < 1f) {
            val scaled = Bitmap.createScaledBitmap(
                bitmap,
                (bitmap.width * edgeScale).roundToInt().coerceAtLeast(1),
                (bitmap.height * edgeScale).roundToInt().coerceAtLeast(1),
                true,
            )
            if (scaled !== bitmap) bitmap.recycle()
            bitmap = scaled
        }

        val opaque = Bitmap.createBitmap(bitmap.width, bitmap.height, Bitmap.Config.ARGB_8888)
        Canvas(opaque).apply {
            drawColor(Color.WHITE)
            drawBitmap(bitmap, 0f, 0f, null)
        }
        bitmap.recycle()
        bitmap = opaque

        try {
            while (true) {
                FileOutputStream(output, false).use { stream ->
                    check(bitmap.compress(Bitmap.CompressFormat.JPEG, JPEG_QUALITY, stream)) {
                        "Unable to encode the camera photo"
                    }
                }
                if (output.length() <= AttachmentStore.MAX_IMAGE_BYTES) {
                    return ImageDimensions(bitmap.width, bitmap.height)
                }

                val sizeRatio = sqrt(AttachmentStore.MAX_IMAGE_BYTES.toDouble() / output.length().toDouble())
                val scale = minOf(0.9, sizeRatio * 0.95).coerceAtLeast(0.5)
                val width = (bitmap.width * scale).roundToInt().coerceAtLeast(1)
                val height = (bitmap.height * scale).roundToInt().coerceAtLeast(1)
                if (width == bitmap.width && height == bitmap.height) {
                    throw IOException("Unable to reduce the camera photo below 5 MiB")
                }
                val smaller = Bitmap.createScaledBitmap(bitmap, width, height, true)
                if (smaller !== bitmap) bitmap.recycle()
                bitmap = smaller
            }
        } finally {
            bitmap.recycle()
        }
    }

    private fun orient(bitmap: Bitmap, source: File): Bitmap {
        val orientation = runCatching {
            ExifInterface(source).getAttributeInt(
                ExifInterface.TAG_ORIENTATION,
                ExifInterface.ORIENTATION_NORMAL,
            )
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

    private fun ownedFile(path: String): File {
        val file = File(path).canonicalFile
        require(file.parentFile == root.canonicalFile) { "Invalid camera capture path" }
        return file
    }

    private fun uriFor(file: File): Uri = FileProvider.getUriForFile(
        appContext,
        "${appContext.packageName}.fileprovider",
        file,
    )

    private data class ImageDimensions(val width: Int, val height: Int)

    companion object {
        const val DIRECTORY_NAME = "camera_captures"
        const val JPEG_QUALITY = 75
        const val MAX_IMAGE_EDGE = 4096
        const val MAX_DRAFT_AGE_MS = 24L * 60L * 60L * 1000L
    }
}
