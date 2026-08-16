package com.tokenflow.chat.data

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import androidx.test.platform.app.InstrumentationRegistry
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.util.UUID
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test

class LocalAvatarStoreTest {
    @Test
    fun savesCenterCroppedSquareGlobalImageAndRemovesItLocally() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val store = LocalAvatarStore(context)
        store.remove(LocalAvatarKind.USER)
        val source = Bitmap.createBitmap(800, 400, Bitmap.Config.ARGB_8888)
        val bytes = ByteArrayOutputStream().use { output ->
            source.compress(Bitmap.CompressFormat.PNG, 100, output)
            output.toByteArray()
        }
        source.recycle()

        val saved = store.save(LocalAvatarKind.USER, ByteArrayInputStream(bytes))
        assertNotNull(saved.user)
        val avatar = requireNotNull(saved.user)
        val decoded = requireNotNull(BitmapFactory.decodeFile(avatar.path))
        assertEquals(384, decoded.width)
        assertEquals(384, decoded.height)
        decoded.recycle()

        val removed = store.remove(LocalAvatarKind.USER)
        assertNull(removed.user)
    }

    @Test
    fun conversationAvatarsAreIsolatedAndDraftPromotes() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val store = LocalAvatarStore(context)
        val first = UUID.randomUUID().toString()
        val second = UUID.randomUUID().toString()
        val source = Bitmap.createBitmap(32, 64, Bitmap.Config.ARGB_8888)
        val bytes = ByteArrayOutputStream().use { output ->
            source.compress(Bitmap.CompressFormat.PNG, 100, output)
            output.toByteArray()
        }
        source.recycle()

        store.saveConversation(first, LocalAvatarKind.USER, ByteArrayInputStream(bytes))
        assertNotNull(store.readConversation(first).user)
        assertNull(store.readConversation(second).user)

        store.clearDraft()
        store.saveDraft(LocalAvatarKind.ASSISTANT, ByteArrayInputStream(bytes))
        assertNotNull(store.promoteDraft(second).assistant)
        assertNull(store.readDraft().assistant)

        store.deleteConversation(first)
        store.deleteConversation(second)
        assertNull(store.readConversation(first).user)
        assertNull(store.readConversation(second).assistant)
    }
}
