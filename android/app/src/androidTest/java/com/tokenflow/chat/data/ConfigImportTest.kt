package com.tokenflow.chat.data

import androidx.room.Room
import androidx.test.platform.app.InstrumentationRegistry
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ConfigImportTest {
    @Test
    fun failedDatabaseMergeRestoresSecretsAndWritesNoPartialRows() = runBlocking {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val database = Room.inMemoryDatabaseBuilder(context, TokenFlowDatabase::class.java).build()
        val dao = database.localDao()
        val secrets = SecretStore(context)
        val providerId = "import-rollback-${System.nanoTime()}"
        val keyName = secrets.providerKeyName(providerId)
        val original = ProviderConfig(providerId, "Original", "https://api.example.com/v1", ProviderProtocol.OPENAI_RESPONSES)
        dao.putProvider(original.toEntity())
        secrets.write(keyName, "old-key")
        val gateway = ModelGateway()
        val webTools = WebToolExecutor(secrets, ExaClient(), UrlReader(context))
        val repository = ChatRepository(
            dao,
            secrets,
            gateway,
            DirectChatEngine(gateway, webTools),
            ConfigArchiveCodec(),
        )
        val payload = ConfigArchivePayload(
            createdAt = 1,
            providers = listOf(ConfigProviderRecord(original.copy(name = "Imported"), "new-key")),
            models = listOf(ModelProfile("invalid-model", "missing-provider", "model-a")),
        )
        val preview = ImportPreview(
            payload = payload,
            newProviders = 0,
            updatedProviders = 1,
            newModels = 1,
            updatedModels = 0,
            replacesExaKey = false,
        )

        var failed = false
        try {
            repository.applyImport(preview)
        } catch (_: Throwable) {
            failed = true
        }

        assertTrue(failed)
        assertEquals("old-key", secrets.read(keyName))
        assertEquals("Original", dao.provider(providerId)?.name)
        assertEquals(null, dao.model("invalid-model"))
        secrets.remove(keyName)
        database.close()
    }
}
