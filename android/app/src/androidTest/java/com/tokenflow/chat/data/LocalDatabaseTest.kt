package com.tokenflow.chat.data

import androidx.room.Room
import androidx.sqlite.db.SupportSQLiteDatabase
import androidx.sqlite.db.SupportSQLiteOpenHelper
import androidx.sqlite.db.framework.FrameworkSQLiteOpenHelperFactory
import androidx.test.platform.app.InstrumentationRegistry
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class LocalDatabaseTest {
    private lateinit var database: TokenFlowDatabase
    private lateinit var dao: LocalDao

    @Before
    fun setUp() {
        database = Room.inMemoryDatabaseBuilder(
            InstrumentationRegistry.getInstrumentation().targetContext,
            TokenFlowDatabase::class.java,
        ).build()
        dao = database.localDao()
    }

    @After
    fun tearDown() {
        database.close()
    }

    @Test
    fun conversationMessagesCascadeAndBatchDelete() = runBlocking {
        seedModel()
        val first = Conversation(id = "conversation-1", model = "model-1")
        val second = Conversation(id = "conversation-2", model = "model-1")
        dao.putConversation(first.toEntity())
        dao.putConversation(second.toEntity())
        dao.putMessages(listOf(ChatMessage(id = "message-1", conversationId = first.id, role = "user", content = "hello").toEntity()))

        dao.deleteConversations(listOf(first.id, second.id))

        assertTrue(dao.conversations().isEmpty())
        assertTrue(dao.messages(first.id).isEmpty())
    }

    @Test
    fun noteFromTheSameMessageIsStoredOnlyOnce() = runBlocking {
        val first = Note(id = "note-1", body = "first", sourceMessageId = "message-1").toEntity()
        val duplicate = Note(id = "note-2", body = "duplicate", sourceMessageId = "message-1").toEntity()

        dao.putNoteIfSourceAbsent(first)
        val stored = dao.putNoteIfSourceAbsent(duplicate)

        assertEquals("note-1", stored.id)
        assertEquals(listOf("note-1"), dao.notes().map { it.id })
    }

    @Test
    fun startupMarksGeneratingMessagesInterrupted() = runBlocking {
        seedModel()
        val conversation = Conversation(id = "conversation-1", model = "model-1", status = "generating")
        dao.putConversation(conversation.toEntity())
        dao.putMessages(listOf(ChatMessage(id = "assistant-1", conversationId = conversation.id, role = "assistant", status = "generating").toEntity()))

        val interrupted = dao.generatingMessages().map { it.copy(status = "interrupted") }
        dao.putMessages(interrupted)
        dao.interruptGeneratingConversations()

        assertEquals("interrupted", dao.message("assistant-1")?.status)
        assertEquals("idle", dao.conversation(conversation.id)?.status)
    }

    @Test
    fun deletingProviderKeepsConversationAndClearsMissingModel() = runBlocking {
        seedModel()
        val conversation = Conversation(id = "conversation-1", model = "model-1")
        dao.putConversation(conversation.toEntity())

        dao.deleteProvider("provider-1")

        assertNull(dao.conversation(conversation.id)?.modelId)
        assertTrue(dao.models().isEmpty())
    }

    @Test
    fun updatingProviderAndModelPreservesConversationModelReference() = runBlocking {
        seedModel()
        val conversation = Conversation(id = "conversation-1", model = "model-1")
        dao.putConversation(conversation.toEntity())

        dao.saveProviderWithModels(
            ProviderConfig("provider-1", "Renamed", "https://api.example.com/v1", ProviderProtocol.OPENAI_RESPONSES).toEntity(),
            listOf(ModelProfile("model-1", "provider-1", "model-a", "Renamed model").toEntity()),
        )

        assertEquals("model-1", dao.conversation(conversation.id)?.modelId)
        assertEquals("Renamed model", dao.model("model-1")?.displayName)
    }

    @Test
    fun globalSettingsAndConversationOverridesRemainIndependent() = runBlocking {
        seedModel()
        dao.putAppSettings(
            AppSettingsEntity(
                systemPrompt = "global prompt",
                urlReaderBackend = UrlReaderBackend.INFOFLOW.name,
            ),
        )
        val inherited = Conversation(id = "conversation-inherit")
        val overridden = Conversation(
            id = "conversation-override",
            model = "model-1",
            modelMode = SettingMode.OVERRIDE,
            systemPrompt = "local prompt",
            systemPromptMode = SettingMode.OVERRIDE,
            urlReaderBackend = UrlReaderBackend.BUILT_IN,
        )
        dao.putConversation(inherited.toEntity())
        dao.putConversation(overridden.toEntity())

        assertEquals("global prompt", dao.appSettings()?.systemPrompt)
        assertEquals(SettingMode.INHERIT, dao.conversation(inherited.id)?.toDomain()?.modelMode)
        assertEquals("model-1", dao.conversation(overridden.id)?.toDomain()?.model)
        assertEquals(UrlReaderBackend.BUILT_IN, dao.conversation(overridden.id)?.toDomain()?.urlReaderBackend)

        dao.deleteProvider("provider-1")
        assertEquals("model-1", dao.conversation(overridden.id)?.modelOverrideId)
        assertEquals("model-1", dao.conversation(overridden.id)?.toDomain()?.model)
    }

    @Test
    fun migrationOneToFourPreservesMessagesAndAddsMultimodalColumns() = runBlocking {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val name = "migration-${System.nanoTime()}.db"
        context.deleteDatabase(name)
        val helper = FrameworkSQLiteOpenHelperFactory().create(
            SupportSQLiteOpenHelper.Configuration.builder(context)
                .name(name)
                .callback(object : SupportSQLiteOpenHelper.Callback(1) {
                    override fun onCreate(db: SupportSQLiteDatabase) {
                        db.execSQL("CREATE TABLE providers (id TEXT NOT NULL PRIMARY KEY, name TEXT NOT NULL, baseUrl TEXT NOT NULL, protocol TEXT NOT NULL, createdAt INTEGER NOT NULL, updatedAt INTEGER NOT NULL)")
                        db.execSQL("CREATE TABLE models (id TEXT NOT NULL PRIMARY KEY, providerId TEXT NOT NULL, remoteId TEXT NOT NULL, displayName TEXT NOT NULL, maxOutputTokens INTEGER NOT NULL, isDefault INTEGER NOT NULL, createdAt INTEGER NOT NULL, updatedAt INTEGER NOT NULL, FOREIGN KEY(providerId) REFERENCES providers(id) ON UPDATE NO ACTION ON DELETE CASCADE)")
                        db.execSQL("CREATE INDEX index_models_providerId ON models(providerId)")
                        db.execSQL("CREATE INDEX index_models_providerId_remoteId ON models(providerId, remoteId)")
                        db.execSQL("CREATE TABLE conversations (id TEXT NOT NULL PRIMARY KEY, title TEXT NOT NULL, titleAutoGenerated INTEGER NOT NULL, modelId TEXT, thinkingEffort TEXT NOT NULL, systemPrompt TEXT NOT NULL, nickname TEXT NOT NULL, userAvatar TEXT NOT NULL, assistantAvatar TEXT NOT NULL, maxToolCalls INTEGER NOT NULL, status TEXT NOT NULL, statusMessage TEXT NOT NULL, createdAt INTEGER NOT NULL, updatedAt INTEGER NOT NULL, lastMessageAt INTEGER, FOREIGN KEY(modelId) REFERENCES models(id) ON UPDATE NO ACTION ON DELETE SET NULL)")
                        db.execSQL("CREATE INDEX index_conversations_modelId ON conversations(modelId)")
                        db.execSQL("CREATE INDEX index_conversations_updatedAt ON conversations(updatedAt)")
                        db.execSQL("CREATE TABLE messages (id TEXT NOT NULL PRIMARY KEY, conversationId TEXT NOT NULL, parentMessageId TEXT, requestId TEXT NOT NULL, role TEXT NOT NULL, content TEXT NOT NULL, metadata TEXT NOT NULL, status TEXT NOT NULL, createdAt INTEGER NOT NULL, FOREIGN KEY(conversationId) REFERENCES conversations(id) ON UPDATE NO ACTION ON DELETE CASCADE)")
                        db.execSQL("CREATE INDEX index_messages_conversationId ON messages(conversationId)")
                        db.execSQL("CREATE INDEX index_messages_conversationId_createdAt ON messages(conversationId, createdAt)")
                    }

                    override fun onUpgrade(db: SupportSQLiteDatabase, oldVersion: Int, newVersion: Int) = Unit
                })
                .build(),
        )
        helper.writableDatabase.apply {
            execSQL("INSERT INTO providers VALUES('p','Provider','https://api.example.com/v1','OPENAI_RESPONSES',1,1)")
            execSQL("INSERT INTO models VALUES('default','p','default','Default',4096,1,1,2)")
            execSQL("INSERT INTO models VALUES('other','p','other','Other',4096,0,1,1)")
            execSQL("INSERT INTO conversations VALUES('inherit','',0,'default','medium','','','U','AI',7,'idle','',1,1,NULL)")
            execSQL("INSERT INTO conversations VALUES('override','',0,'other','medium','custom','','U','AI',7,'idle','',1,1,NULL)")
            execSQL("INSERT INTO messages VALUES('m','override',NULL,'r','user','kept','','completed',2)")
        }
        helper.close()

        val migrated = Room.databaseBuilder(context, TokenFlowDatabase::class.java, name)
            .addMigrations(TokenFlowDatabase.MIGRATION_1_2, TokenFlowDatabase.MIGRATION_2_3, TokenFlowDatabase.MIGRATION_3_4)
            .build()
        try {
            val migratedDao = migrated.localDao()
            assertEquals(SettingMode.INHERIT, migratedDao.conversation("inherit")?.toDomain()?.modelMode)
            assertEquals(SettingMode.INHERIT, migratedDao.conversation("inherit")?.toDomain()?.systemPromptMode)
            assertEquals(SettingMode.OVERRIDE, migratedDao.conversation("override")?.toDomain()?.modelMode)
            assertEquals("other", migratedDao.conversation("override")?.modelOverrideId)
            assertEquals("kept", migratedDao.messages("override").single().content)
            assertEquals(UrlReaderBackend.BUILT_IN.name, migratedDao.appSettings()?.urlReaderBackend)
            assertTrue(migratedDao.conversation("override")?.enableSearch == true)
            assertTrue(migratedDao.conversation("override")?.enableRead == true)
            assertTrue(migratedDao.bookmarks().isEmpty())
            assertTrue(migratedDao.notes().isEmpty())
            assertTrue(migratedDao.agents().isEmpty())
            assertEquals(VisionStatus.UNKNOWN.name, migratedDao.model("default")?.visionStatus)
            assertEquals("mimo_default", migratedDao.appSettings()?.mimoTtsVoice)
            assertTrue(migratedDao.attachmentsForMessage("m").isEmpty())
        } finally {
            migrated.close()
            context.deleteDatabase(name)
        }
    }

    @Test
    fun latestAssistantSupportsRegenerationReplacement() = runBlocking {
        seedModel()
        val conversation = Conversation(id = "conversation-1", model = "model-1")
        dao.putConversation(conversation.toEntity())
        dao.putMessages(
            listOf(
                ChatMessage(id = "user-1", conversationId = conversation.id, role = "user", content = "question", createdAt = 1).toEntity(),
                ChatMessage(id = "assistant-1", conversationId = conversation.id, role = "assistant", content = "old", createdAt = 2).toEntity(),
            ),
        )

        val latest = requireNotNull(dao.latestAssistant(conversation.id))
        dao.deleteMessage(latest.id)
        dao.putMessages(listOf(ChatMessage(id = "assistant-2", conversationId = conversation.id, role = "assistant", content = "new", createdAt = 3).toEntity()))

        assertEquals(listOf("user-1", "assistant-2"), dao.messages(conversation.id).map { it.id })
    }

    @Test
    fun workspaceEntitiesSupportBookmarkNotesAgentsAndBilingualFts() = runBlocking {
        seedModel()
        val conversation = Conversation(id = "conversation-workspace", model = "model-1")
        dao.putConversation(conversation.toEntity())
        dao.putMessages(listOf(ChatMessage(id = "assistant-workspace", conversationId = conversation.id, role = "assistant", content = "Room 数据迁移").toEntity()))
        dao.putBookmark(BookmarkEntity("bookmark-1", "assistant-workspace", 1))
        dao.putNote(Note(id = "note-1", title = "Migration", body = "Room schema").toEntity())
        dao.putAgent(AgentProfile(id = "agent-1", name = "Reviewer", modelId = "model-1").toEntity())
        dao.putKnowledgeDocument(KnowledgeDocumentEntity("document-1", "guide.md", "text/markdown", "private", 10, "ready", "", 1, 1, 1))
        dao.replaceKnowledgeChunks(
            "document-1",
            listOf(KnowledgeChunkEntity(documentId = "document-1", position = 0, text = "Room 数据迁移", searchText = KnowledgeStore.searchable("Room 数据迁移"))),
        )

        assertEquals("assistant-workspace", dao.bookmarks().single().messageId)
        assertEquals("Migration", dao.notes().single().title)
        assertEquals("Reviewer", dao.agents().single().name)
        assertEquals(1, dao.searchKnowledgeChunks("\"room\"", 5).size)
        assertEquals(1, dao.searchKnowledgeChunks("\"数据\"", 5).size)

        dao.deleteMessage("assistant-workspace")
        assertTrue(dao.bookmarks().isEmpty())
    }

    private suspend fun seedModel() {
        dao.putProvider(ProviderConfig("provider-1", "Provider", "https://api.example.com/v1", ProviderProtocol.OPENAI_RESPONSES).toEntity())
        dao.putModels(listOf(ModelProfile("model-1", "provider-1", "model-a").toEntity()))
    }
}
