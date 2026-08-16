package com.tokenflow.chat.ui

import com.tokenflow.chat.data.ChatDataSource
import com.tokenflow.chat.data.ChatEvent
import com.tokenflow.chat.data.ChatMessage
import com.tokenflow.chat.data.Conversation
import com.tokenflow.chat.data.ConversationDetail
import com.tokenflow.chat.data.ConversationWriteRequest
import com.tokenflow.chat.data.GlobalChatSettings
import com.tokenflow.chat.data.ImportPreview
import com.tokenflow.chat.data.ModelProfile
import com.tokenflow.chat.data.PendingAttachment
import com.tokenflow.chat.data.PendingAttachmentOrigin
import com.tokenflow.chat.data.ProcessEvent
import com.tokenflow.chat.data.ProviderConfig
import com.tokenflow.chat.data.ProviderDraft
import com.tokenflow.chat.data.ProviderEditorData
import com.tokenflow.chat.data.ProviderProtocol
import com.tokenflow.chat.data.RemoteModel
import com.tokenflow.chat.data.SendMessageRequest
import com.tokenflow.chat.data.TtsAudio
import com.tokenflow.chat.data.Usage
import com.tokenflow.chat.data.VisionStatus
import com.tokenflow.chat.data.WorkspaceSnapshot
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import java.io.File
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class AppViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun setUp() {
        Dispatchers.setMain(dispatcher)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
    }

    @Test
    fun firstLaunchWithoutModelsOpensProviderSetupWithoutLogin() = runTest(dispatcher) {
        val fake = FakeChatDataSource(withModel = false)
        val viewModel = AppViewModel(fake)

        advanceUntilIdle()

        assertTrue(fake.initialized)
        assertEquals(AppPhase.SETUP, viewModel.state.value.phase)
        assertEquals(AppScreen.PROVIDERS, viewModel.state.value.screen)
        assertFalse(viewModel.state.value.hasModels)
    }

    @Test
    fun configuredWorkspaceStartsChatAndMergesStreamEvents() = runTest(dispatcher) {
        val fake = FakeChatDataSource(withModel = true)
        val viewModel = AppViewModel(fake)
        advanceUntilIdle()

        assertEquals(AppPhase.READY, viewModel.state.value.phase)
        viewModel.send("Hello locally")
        advanceUntilIdle()

        assertNotNull(fake.sentRequest?.requestId)
        assertNotNull(fake.sentRequest?.timeZone)
        assertEquals("Hello locally", fake.sentRequest?.content)
        assertEquals("Answer from provider", viewModel.state.value.activeMessages.last().content)
        assertFalse(viewModel.state.value.activeGeneration?.active ?: true)
        assertTrue(viewModel.state.value.generations.values.single().events.any { it.type == "thinking" })
    }

    @Test
    fun conversationSelectionAndBatchDeleteUseUuidIds() = runTest(dispatcher) {
        val fake = FakeChatDataSource(withModel = true).apply {
            conversations += Conversation(id = "conversation-existing", title = "Existing", model = model.id)
        }
        val viewModel = AppViewModel(fake)
        advanceUntilIdle()

        viewModel.openConversation("conversation-existing")
        advanceUntilIdle()
        assertEquals("conversation-existing", viewModel.state.value.activeConversationId)

        viewModel.deleteConversations(setOf("conversation-existing"))
        advanceUntilIdle()
        assertTrue("conversation-existing" in fake.deleted)
        assertEquals(null, viewModel.state.value.activeConversationId)
    }

    @Test
    fun cameraDraftsAreDiscardedWhenRemovedRejectedOrAbandoned() = runTest(dispatcher) {
        val fake = FakeChatDataSource(withModel = true).apply {
            conversations += Conversation(id = "conversation-existing", title = "Existing", model = model.id)
        }
        val viewModel = AppViewModel(fake)
        advanceUntilIdle()

        val removed = cameraDraft("removed")
        viewModel.addAttachments(listOf(removed))
        viewModel.removeAttachment(removed.uri)
        advanceUntilIdle()
        assertTrue(removed in fake.discardedAttachments)

        val accepted = (1..5).map { cameraDraft("accepted-$it") }
        val rejected = cameraDraft("rejected")
        viewModel.addAttachments(accepted + rejected)
        advanceUntilIdle()
        assertEquals(5, viewModel.state.value.pendingAttachments.size)
        assertTrue(rejected in fake.discardedAttachments)

        viewModel.newConversation()
        advanceUntilIdle()
        assertTrue(fake.discardedAttachments.containsAll(accepted))

        val switched = cameraDraft("switched")
        viewModel.addAttachments(listOf(switched))
        viewModel.openConversation("conversation-existing")
        advanceUntilIdle()
        assertTrue(switched in fake.discardedAttachments)
        assertTrue(viewModel.state.value.pendingAttachments.isEmpty())
    }

    @Test
    fun switchingImmediatelyAfterSendDoesNotDiscardInFlightCameraDraft() = runTest(dispatcher) {
        val fake = FakeChatDataSource(withModel = true).apply {
            conversations += Conversation(id = "conversation-source", title = "Source", model = model.id)
            conversations += Conversation(id = "conversation-other", title = "Other", model = model.id)
        }
        val viewModel = AppViewModel(fake)
        advanceUntilIdle()
        viewModel.openConversation("conversation-source")
        advanceUntilIdle()
        assertEquals("conversation-source", viewModel.state.value.activeConversationId)

        val inFlight = cameraDraft("in-flight")
        viewModel.addAttachments(listOf(inFlight))
        viewModel.send("with camera")
        assertTrue(viewModel.state.value.pendingAttachments.isEmpty())
        viewModel.openConversation("conversation-other")
        advanceUntilIdle()

        assertTrue(inFlight !in fake.discardedAttachments)
        assertEquals(listOf(inFlight), fake.sentRequest?.attachments)
    }

    @Test
    fun speechAutoPlayIsOneShotAndScopedToItsChatInstance() = runTest(dispatcher) {
        val viewModel = AppViewModel(FakeChatDataSource(withModel = true))
        advanceUntilIdle()
        val event = async(start = CoroutineStart.UNDISPATCHED) { viewModel.speechAutoPlay.first() }
        val target = SpeechAutoPlayTarget("conversation-a", "chat-instance-a")

        viewModel.synthesizeSpeech("assistant-message", autoPlayTarget = target)
        advanceUntilIdle()

        val request = event.await()
        assertEquals("assistant-message", request.messageId)
        assertEquals(target, request.target)
        assertTrue(shouldAutoPlaySpeech(request, "conversation-a", "chat-instance-a"))
        assertFalse(shouldAutoPlaySpeech(request, "conversation-a", "chat-instance-after-notes"))
        assertFalse(shouldAutoPlaySpeech(request, "conversation-b", "chat-instance-a"))
        assertTrue(viewModel.speechAutoPlay.replayCache.isEmpty())
    }

    private fun cameraDraft(id: String) = PendingAttachment(
        uri = "content://camera/$id",
        displayName = "$id.jpg",
        mimeType = "image/jpeg",
        sizeBytes = 100,
        origin = PendingAttachmentOrigin.CAMERA,
        appOwnedDraftPath = "/camera/$id.jpg",
    )
}

private class FakeChatDataSource(withModel: Boolean) : ChatDataSource {
    val provider = ProviderConfig("provider-1", "Provider", "https://api.example.com/v1", ProviderProtocol.OPENAI_RESPONSES, true)
    val model = ModelProfile(
        "model-1",
        provider.id,
        "model-a",
        "Model A",
        4096,
        true,
        visionStatus = VisionStatus.SUPPORTED,
    )
    val conversations = mutableListOf<Conversation>()
    val messageMap = mutableMapOf<String, List<ChatMessage>>()
    val deleted = mutableSetOf<String>()
    val discardedAttachments = mutableListOf<PendingAttachment>()
    var initialized = false
    var sentRequest: SendMessageRequest? = null
    private var models = if (withModel) listOf(model) else emptyList()

    override suspend fun initialize() {
        initialized = true
    }

    override suspend fun workspace() = WorkspaceSnapshot(
        providers = if (models.isEmpty()) emptyList() else listOf(provider),
        models = models,
        conversations = conversations.toList(),
        exaConfigured = false,
        globalSettings = GlobalChatSettings(defaultModelId = models.firstOrNull()?.id),
    )

    override suspend fun provider(id: String) = ProviderEditorData(
        ProviderDraft(provider.id, provider.name, provider.baseUrl, provider.protocol, "secret"),
        models,
    )

    override suspend fun fetchModels(draft: ProviderDraft) = listOf(RemoteModel("model-a"))

    override suspend fun saveProvider(draft: ProviderDraft, models: List<ModelProfile>): ProviderConfig {
        this.models = models
        return provider
    }

    override suspend fun deleteProvider(id: String) {
        models = emptyList()
    }

    override suspend fun setDefaultModel(id: String) = Unit
    override suspend fun synthesizeSpeech(messageId: String, force: Boolean) =
        TtsAudio(File("$messageId.wav"), fromCache = false)
    override fun exaConfigured() = false
    override fun saveExaKey(value: String) = Unit
    override suspend fun conversations() = conversations.toList()

    override suspend fun conversation(id: String): ConversationDetail {
        val conversation = conversations.first { it.id == id }
        return ConversationDetail(conversation, messageMap[id].orEmpty())
    }

    override suspend fun createConversation(request: ConversationWriteRequest): Conversation {
        val conversation = Conversation(
            id = "conversation-${conversations.size + 1}",
            model = request.model ?: model.id,
            thinkingEffort = request.thinkingEffort ?: "medium",
            systemPrompt = request.systemPrompt.orEmpty(),
            nickname = request.nickname.orEmpty(),
            maxToolCalls = request.maxToolCalls ?: 7,
        )
        conversations += conversation
        return conversation
    }

    override suspend fun updateConversation(id: String, request: ConversationWriteRequest): Conversation {
        val index = conversations.indexOfFirst { it.id == id }
        val current = conversations[index]
        val updated = current.copy(
            title = request.title ?: current.title,
            model = request.model ?: current.model,
            systemPrompt = request.systemPrompt ?: current.systemPrompt,
        )
        conversations[index] = updated
        return updated
    }

    override suspend fun deleteConversations(ids: Set<String>) {
        deleted += ids
        conversations.removeAll { it.id in ids }
        ids.forEach(messageMap::remove)
    }

    override suspend fun discardPendingAttachments(attachments: List<PendingAttachment>) {
        discardedAttachments += attachments
    }

    override suspend fun generateTitle(id: String, force: Boolean): Conversation =
        updateConversation(id, ConversationWriteRequest(title = "Generated title"))

    override fun sendMessage(id: String, request: SendMessageRequest): Flow<ChatEvent> = flow {
        sentRequest = request
        val user = ChatMessage("user-1", id, requestId = request.requestId, role = "user", content = request.content)
        val initial = ChatMessage("assistant-1", id, requestId = request.requestId, role = "assistant", status = "generating")
        val final = initial.copy(content = "Answer from provider", status = "completed")
        emit(ChatEvent.UserMessage(user))
        emit(ChatEvent.AssistantMessage(initial))
        emit(ChatEvent.Process(ProcessEvent(type = "thinking", id = "thinking-1", content = "summary")))
        emit(ChatEvent.Delta("Answer from provider"))
        messageMap[id] = listOf(user, final)
        emit(ChatEvent.Done(Usage(3, 4), false))
    }

    override fun regenerate(id: String, request: SendMessageRequest): Flow<ChatEvent> = flow { }
    override suspend fun exportConfiguration(password: CharArray) = "archive"
    override suspend fun previewImport(raw: String, password: CharArray): ImportPreview = error("not used")
    override suspend fun applyImport(preview: ImportPreview) = Unit
}
