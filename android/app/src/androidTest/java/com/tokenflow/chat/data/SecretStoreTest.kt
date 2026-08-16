package com.tokenflow.chat.data

import android.content.Context
import androidx.test.platform.app.InstrumentationRegistry
import java.util.UUID
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

class SecretStoreTest {
    @Test
    fun apiKeyIsEncryptedAtRestAndCanBeRemoved() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val store = SecretStore(context)
        val name = store.providerKeyName(UUID.randomUUID().toString())
        val secret = "sk-test-${UUID.randomUUID()}"

        store.write(name, secret)

        assertEquals(secret, store.read(name))
        val stored = context.getSharedPreferences("tokenflow_secrets_v2", Context.MODE_PRIVATE).getString(name, null).orEmpty()
        assertFalse(stored.contains(secret))
        store.remove(name)
        assertEquals(null, store.read(name))
    }
}
