package com.tokenflow.chat.data

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Canvas
import android.graphics.Color
import androidx.exifinterface.media.ExifInterface
import androidx.room.Room
import androidx.test.platform.app.InstrumentationRegistry
import java.io.File
import java.io.FileOutputStream
import java.util.UUID
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class CameraCaptureStoreTest {
    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private lateinit var database: TokenFlowDatabase
    private val createdFiles = mutableListOf<File>()

    @Before
    fun setUp() {
        database = Room.inMemoryDatabaseBuilder(context, TokenFlowDatabase::class.java).build()
    }

    @After
    fun tearDown() {
        database.close()
        createdFiles.forEach(File::delete)
    }

    @Test
    fun capturedPhotoIsRotatedAndEncodedAsJpeg75() = runBlocking {
        val store = CameraCaptureStore(context)
        val target = store.createCapture().also { createdFiles += File(it.path) }
        writeStripedJpeg(File(target.path), 120, 60)
        ExifInterface(target.path).apply {
            setAttribute(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_ROTATE_90.toString())
            saveAttributes()
        }

        val pending = store.finishCapture(target.path)
        val output = File(requireNotNull(pending.appOwnedDraftPath)).also { createdFiles += it }
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(output.path, bounds)

        assertFalse(File(target.path).exists())
        assertEquals(PendingAttachmentOrigin.CAMERA, pending.origin)
        assertEquals("image/jpeg", pending.mimeType)
        assertEquals("image/jpeg", bounds.outMimeType)
        assertEquals(60, bounds.outWidth)
        assertEquals(120, bounds.outHeight)
        assertEquals(75, CameraCaptureStore.JPEG_QUALITY)
        assertTrue(output.length() in 1L..AttachmentStore.MAX_IMAGE_BYTES)
    }

    @Test
    fun capturedPhotoAppliesExifMirrorAndLimitsLongestEdge() = runBlocking {
        val store = CameraCaptureStore(context)
        val mirrorTarget = store.createCapture().also { createdFiles += File(it.path) }
        writeStripedJpeg(File(mirrorTarget.path), 120, 60)
        ExifInterface(mirrorTarget.path).apply {
            setAttribute(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_FLIP_HORIZONTAL.toString())
            saveAttributes()
        }

        val mirrored = store.finishCapture(mirrorTarget.path)
        val mirroredFile = File(requireNotNull(mirrored.appOwnedDraftPath)).also { createdFiles += it }
        val bitmap = requireNotNull(BitmapFactory.decodeFile(mirroredFile.path))
        assertTrue(Color.blue(bitmap.getPixel(10, 30)) > Color.red(bitmap.getPixel(10, 30)))
        assertTrue(Color.red(bitmap.getPixel(110, 30)) > Color.blue(bitmap.getPixel(110, 30)))
        bitmap.recycle()

        val largeTarget = store.createCapture().also { createdFiles += File(it.path) }
        writeStripedJpeg(File(largeTarget.path), 5000, 80)
        val scaled = store.finishCapture(largeTarget.path)
        val scaledFile = File(requireNotNull(scaled.appOwnedDraftPath)).also { createdFiles += it }
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(scaledFile.path, bounds)
        assertTrue(maxOf(bounds.outWidth, bounds.outHeight) <= CameraCaptureStore.MAX_IMAGE_EDGE)
    }

    @Test
    fun normalizedCameraDraftIsCopiedWithoutSecondEncoding() = runBlocking {
        val cameraStore = CameraCaptureStore(context)
        val target = cameraStore.createCapture().also { createdFiles += File(it.path) }
        writeStripedJpeg(File(target.path), 160, 90)
        val pending = cameraStore.finishCapture(target.path)
        val draft = File(requireNotNull(pending.appOwnedDraftPath)).also { createdFiles += it }
        val originalBytes = draft.readBytes()

        val conversation = Conversation(id = UUID.randomUUID().toString())
        val message = ChatMessage(
            id = UUID.randomUUID().toString(),
            conversationId = conversation.id,
            role = "user",
        )
        database.localDao().putConversation(conversation.toEntity())
        database.localDao().putMessages(listOf(message.toEntity()))
        val attachmentStore = AttachmentStore(context, database.localDao())
        val stored = attachmentStore.persist(message.id, listOf(pending)).single()
        createdFiles += File(stored.storedPath)

        assertArrayEquals(originalBytes, File(stored.storedPath).readBytes())
        assertTrue(draft.exists())
        attachmentStore.discardPendingDrafts(listOf(pending))
        assertFalse(draft.exists())
    }

    @Test
    fun cancellationAndAgeCleanupRemoveOnlyCameraDrafts() {
        val store = CameraCaptureStore(context)
        val cancelled = store.createCapture().also { createdFiles += File(it.path) }
        store.cancelCapture(cancelled.path)
        assertFalse(File(cancelled.path).exists())

        val expired = store.createCapture().also { createdFiles += File(it.path) }
        val expiredFile = File(expired.path)
        assertTrue(expiredFile.setLastModified(System.currentTimeMillis() - CameraCaptureStore.MAX_DRAFT_AGE_MS - 1))
        store.cleanupExpired()
        assertFalse(expiredFile.exists())
    }

    private fun writeStripedJpeg(file: File, width: Int, height: Int) {
        val bitmap = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888)
        Canvas(bitmap).apply {
            drawColor(Color.RED)
            clipRect(width / 2, 0, width, height)
            drawColor(Color.BLUE)
        }
        FileOutputStream(file).use { output ->
            assertTrue(bitmap.compress(Bitmap.CompressFormat.JPEG, 100, output))
        }
        bitmap.recycle()
    }
}
