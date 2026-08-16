package com.tokenflow.chat.ui

import android.content.ClipboardManager
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.longClick
import androidx.test.espresso.Espresso.pressBack
import androidx.test.platform.app.InstrumentationRegistry
import com.tokenflow.chat.data.ChatDataSource
import com.tokenflow.chat.data.ChatEvent
import com.tokenflow.chat.data.ChatMessage
import com.tokenflow.chat.data.BookmarkedMessage
import com.tokenflow.chat.data.Conversation
import com.tokenflow.chat.data.ConversationDetail
import com.tokenflow.chat.data.ConversationWriteRequest
import com.tokenflow.chat.data.ImportPreview
import com.tokenflow.chat.data.GlobalChatSettings
import com.tokenflow.chat.data.ModelProfile
import com.tokenflow.chat.data.Note
import com.tokenflow.chat.data.PendingAttachment
import com.tokenflow.chat.data.ProcessEvent
import com.tokenflow.chat.data.ProviderConfig
import com.tokenflow.chat.data.ProviderDraft
import com.tokenflow.chat.data.ProviderEditorData
import com.tokenflow.chat.data.ProviderProtocol
import com.tokenflow.chat.data.RemoteModel
import com.tokenflow.chat.data.SendMessageRequest
import com.tokenflow.chat.data.Usage
import com.tokenflow.chat.data.WorkspaceSnapshot
import com.tokenflow.chat.ui.theme.TokenFlowTheme
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

class TokenFlowAppTest {
    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun firstLaunchShowsProviderSetupWithoutLogin() {
        val fake = UiFakeDataSource(withModel = false)
        val viewModel = AppViewModel(fake)
        composeRule.setContent { TokenFlowTheme { TokenFlowApp(viewModel) } }

        composeRule.waitUntil(5_000) { fake.initialized }
        composeRule.onNodeWithTag(UiTestTags.PROVIDER_GUIDE).assertIsDisplayed()
        composeRule.onNodeWithTag(UiTestTags.ADD_PROVIDER).performClick()
        composeRule.onNodeWithTag(UiTestTags.PROVIDER_NAME).assertIsDisplayed()
        composeRule.onNodeWithTag(UiTestTags.PROVIDER_BASE_URL).assertIsDisplayed()
        composeRule.onNodeWithTag(UiTestTags.PROVIDER_API_KEY).assertIsDisplayed()
    }

    @Test
    fun configuredAppSendsAndCopiesProviderResponse() {
        val fake = UiFakeDataSource(withModel = true)
        val viewModel = AppViewModel(fake)
        composeRule.setContent { TokenFlowTheme { TokenFlowApp(viewModel) } }
        composeRule.waitUntil(5_000) { fake.initialized }
        composeRule.waitForIdle()

        composeRule.onNodeWithTag(UiTestTags.MESSAGE_INPUT).performTextInput("Hello locally")
        val inputBottom = composeRule.onNodeWithTag(UiTestTags.MESSAGE_INPUT).fetchSemanticsNode().boundsInRoot.bottom
        val rootBottom = composeRule.onRoot().fetchSemanticsNode().boundsInRoot.bottom
        assertTrue(inputBottom > rootBottom * 0.5f)
        composeRule.onNodeWithTag(UiTestTags.MESSAGE_ACTION).performClick()
        composeRule.waitUntil(5_000) { fake.sentRequest != null }

        assertEquals("Hello locally", fake.sentRequest?.content)
        assertNotNull(fake.sentRequest?.requestId)
        composeRule.onNodeWithText("Answer from provider").assertIsDisplayed()
        composeRule.onNodeWithTag(UiTestTags.TOKEN_USAGE).assertIsDisplayed()
        composeRule.onNodeWithText("1.2K").assertIsDisplayed()
        composeRule.onNodeWithTag(UiTestTags.SPEECH_ACTION).assertIsDisplayed()
        val userAvatar = composeRule.onNodeWithTag(UiTestTags.USER_MESSAGE_AVATAR).fetchSemanticsNode().boundsInRoot
        assertEquals(userAvatar.width, userAvatar.height, 0.5f)
        composeRule.onNodeWithTag(UiTestTags.COPY_ASSISTANT_MESSAGE).performClick()
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val clipboard = context.getSystemService(ClipboardManager::class.java)
        assertEquals("Answer from provider", clipboard.primaryClip?.getItemAt(0)?.coerceToText(context)?.toString())
    }

    @Test
    fun attachmentMenuShowsCameraFirstAndPendingAttachmentCanBeRemoved() {
        val fake = UiFakeDataSource(withModel = true)
        val viewModel = AppViewModel(fake)
        composeRule.setContent { TokenFlowTheme { TokenFlowApp(viewModel) } }
        composeRule.waitUntil(5_000) { fake.initialized }
        val context = InstrumentationRegistry.getInstrumentation().targetContext

        composeRule.onNodeWithContentDescription(
            context.getString(com.tokenflow.chat.R.string.add_attachment),
        ).performClick()
        val cameraTop = composeRule.onNodeWithText(
            context.getString(com.tokenflow.chat.R.string.take_photo),
        ).fetchSemanticsNode().boundsInRoot.top
        val imagesTop = composeRule.onNodeWithText(
            context.getString(com.tokenflow.chat.R.string.choose_images),
        ).fetchSemanticsNode().boundsInRoot.top
        val filesTop = composeRule.onNodeWithText(
            context.getString(com.tokenflow.chat.R.string.choose_files),
        ).fetchSemanticsNode().boundsInRoot.top
        assertTrue(cameraTop < imagesTop && imagesTop < filesTop)
        composeRule.onNodeWithText(context.getString(com.tokenflow.chat.R.string.cancel)).performClick()

        viewModel.addAttachments(listOf(PendingAttachment(
            uri = "content://documents/draft.txt",
            displayName = "draft.txt",
            mimeType = "text/plain",
            sizeBytes = 12,
        )))
        composeRule.onNodeWithText("draft.txt").assertIsDisplayed()
        composeRule.onNodeWithContentDescription(
            context.getString(com.tokenflow.chat.R.string.remove),
        ).performClick()
        composeRule.waitUntil(5_000) { viewModel.state.value.pendingAttachments.isEmpty() }
        composeRule.onAllNodesWithText("draft.txt").assertCountEquals(0)
    }

    @Test
    fun longPressConversationEntersUuidSelectionMode() {
        val fake = UiFakeDataSource(withModel = true).apply {
            conversations += Conversation(id = "conversation-existing", title = "Existing", model = model.id)
        }
        val viewModel = AppViewModel(fake)
        composeRule.setContent { TokenFlowTheme { TokenFlowApp(viewModel) } }
        composeRule.waitUntil(5_000) { fake.initialized }

        composeRule.onNodeWithTag(UiTestTags.OPEN_CONVERSATIONS).performClick()
        composeRule.onNodeWithTag(UiTestTags.conversationItem("conversation-existing")).performTouchInput { longClick() }
        composeRule.onNodeWithTag(UiTestTags.RENAME_SELECTED).assertIsDisplayed()
        composeRule.onNodeWithTag(UiTestTags.DELETE_SELECTED).assertIsDisplayed()
    }

    @Test
    fun globalSettingsExposeAnonymousInfoFlowAndDefaultModel() {
        val fake = UiFakeDataSource(withModel = true)
        val viewModel = AppViewModel(fake)
        composeRule.setContent { TokenFlowTheme { TokenFlowApp(viewModel) } }
        composeRule.waitUntil(5_000) { fake.initialized }
        val context = InstrumentationRegistry.getInstrumentation().targetContext

        composeRule.onNodeWithTag(UiTestTags.OPEN_CONVERSATIONS).performClick()
        composeRule.onNodeWithTag(UiTestTags.EXPAND_DESTINATIONS).performClick()
        composeRule.onNodeWithText(context.getString(com.tokenflow.chat.R.string.global_settings)).performClick()
        composeRule.onNodeWithText("InfoFlow").assertIsDisplayed()
        composeRule.onAllNodesWithText(context.getString(com.tokenflow.chat.R.string.infoflow_api_key)).assertCountEquals(0)
        composeRule.onNodeWithText("Model A").assertIsDisplayed()
    }

    @Test
    fun bookmarksAndNotesSupportSelectAllAndBatchDelete() {
        val fake = UiFakeDataSource(withModel = true).apply {
            bookmarks += BookmarkedMessage(messageId = "message-1", conversationId = "conversation-1", content = "First saved answer")
            bookmarks += BookmarkedMessage(messageId = "message-2", conversationId = "conversation-1", content = "Second saved answer")
            notes += Note(id = "note-1", title = "First note", body = "Body one")
            notes += Note(id = "note-2", title = "Second note", body = "Body two")
        }
        val viewModel = AppViewModel(fake)
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        composeRule.setContent { TokenFlowTheme { TokenFlowApp(viewModel) } }
        composeRule.waitUntil(5_000) { fake.initialized }

        viewModel.openScreen(AppScreen.BOOKMARKS)
        composeRule.onNodeWithTag(UiTestTags.bookmarkItem("message-1")).performTouchInput { longClick() }
        composeRule.onNodeWithTag(UiTestTags.WORKSPACE_SELECT_ALL).performClick()
        composeRule.onNodeWithTag(UiTestTags.WORKSPACE_DELETE_SELECTED).performClick()
        composeRule.onNodeWithText(context.getString(com.tokenflow.chat.R.string.delete)).performClick()
        composeRule.waitUntil(5_000) { fake.bookmarks.isEmpty() }

        viewModel.openScreen(AppScreen.NOTES)
        composeRule.onNodeWithTag(UiTestTags.noteItem("note-1")).performTouchInput { longClick() }
        composeRule.onNodeWithTag(UiTestTags.WORKSPACE_SELECT_ALL).performClick()
        composeRule.onNodeWithTag(UiTestTags.WORKSPACE_DELETE_SELECTED).performClick()
        composeRule.onNodeWithText(context.getString(com.tokenflow.chat.R.string.delete)).performClick()
        composeRule.waitUntil(5_000) { fake.notes.isEmpty() }
    }

    @Test
    fun existingNoteOpensRenderedReaderAndEditIsExplicit() {
        val fake = UiFakeDataSource(withModel = true).apply {
            notes += Note(
                id = "note-rich",
                title = "Rich note",
                body = "# Rendered heading\n\n<mark>Jade highlight</mark> and **strong text**",
            )
        }
        val viewModel = AppViewModel(fake)
        composeRule.setContent { TokenFlowTheme { TokenFlowApp(viewModel) } }
        composeRule.waitUntil(5_000) { fake.initialized }

        viewModel.openScreen(AppScreen.NOTES)
        composeRule.onNodeWithTag(UiTestTags.noteItem("note-rich")).performClick()

        composeRule.onNodeWithTag("note_reader").assertIsDisplayed()
        composeRule.onNodeWithText("Rendered heading").assertIsDisplayed()
        composeRule.onNodeWithText("Jade highlight and strong text").assertIsDisplayed()
        composeRule.onAllNodesWithTag("note_editor").assertCountEquals(0)

        composeRule.onNodeWithTag("note_reader_edit").performClick()
        composeRule.onNodeWithTag("note_editor").assertIsDisplayed()
        composeRule.onAllNodesWithTag("note_reader").assertCountEquals(0)
    }

    @Test
    fun systemBackLeavesNoteReaderBeforeReturningToChat() {
        val fake = UiFakeDataSource(withModel = true).apply {
            notes += Note(id = "note-back", title = "Back navigation", body = "Rendered body")
        }
        val viewModel = AppViewModel(fake)
        composeRule.setContent { TokenFlowTheme { TokenFlowApp(viewModel) } }
        composeRule.waitUntil(5_000) { fake.initialized }

        viewModel.openScreen(AppScreen.NOTES)
        composeRule.onNodeWithTag(UiTestTags.noteItem("note-back")).performClick()
        composeRule.onNodeWithTag("note_reader").assertIsDisplayed()

        pressBack()
        composeRule.onNodeWithTag(UiTestTags.noteItem("note-back")).assertIsDisplayed()
        assertEquals(AppScreen.NOTES, viewModel.state.value.screen)

        pressBack()
        composeRule.waitUntil(5_000) { viewModel.state.value.screen == AppScreen.CHAT }
        composeRule.onNodeWithTag(UiTestTags.MESSAGE_INPUT).assertIsDisplayed()
    }

    @Test
    fun newNoteOpensEditorDirectly() {
        val fake = UiFakeDataSource(withModel = true)
        val viewModel = AppViewModel(fake)
        composeRule.setContent { TokenFlowTheme { TokenFlowApp(viewModel) } }
        composeRule.waitUntil(5_000) { fake.initialized }

        viewModel.openScreen(AppScreen.NOTES)
        composeRule.onNodeWithTag("note_create").performClick()

        composeRule.onNodeWithTag("note_editor").assertIsDisplayed()
        composeRule.onAllNodesWithTag("note_reader").assertCountEquals(0)
    }
}

private class UiFakeDataSource(withModel: Boolean) : ChatDataSource {
    val provider = ProviderConfig("provider-1", "Provider", "https://api.example.com/v1", ProviderProtocol.OPENAI_RESPONSES, true)
    val model = ModelProfile("model-1", provider.id, "model-a", "Model A", 4096, true)
    val conversations = mutableListOf<Conversation>()
    val bookmarks = mutableListOf<BookmarkedMessage>()
    val notes = mutableListOf<Note>()
    private val messages = mutableMapOf<String, List<ChatMessage>>()
    private var models = if (withModel) listOf(model) else emptyList()
    @Volatile var initialized = false
    @Volatile var sentRequest: SendMessageRequest? = null

    override suspend fun initialize() { initialized = true }
    override suspend fun workspace() = WorkspaceSnapshot(
        providers = if (models.isEmpty()) emptyList() else listOf(provider),
        models = models,
        conversations = conversations.toList(),
        exaConfigured = false,
        globalSettings = GlobalChatSettings(defaultModelId = models.firstOrNull()?.id),
        bookmarks = bookmarks.toList(),
        notes = notes.toList(),
    )
    override suspend fun provider(id: String) = ProviderEditorData(ProviderDraft(provider.id, provider.name, provider.baseUrl, provider.protocol, "secret"), models)
    override suspend fun fetchModels(draft: ProviderDraft) = listOf(RemoteModel("model-a"))
    override suspend fun saveProvider(draft: ProviderDraft, models: List<ModelProfile>): ProviderConfig { this.models = models; return provider }
    override suspend fun deleteProvider(id: String) { models = emptyList() }
    override suspend fun setDefaultModel(id: String) = Unit
    override fun exaConfigured() = false
    override fun saveExaKey(value: String) = Unit
    override suspend fun conversations() = conversations.toList()
    override suspend fun conversation(id: String) = ConversationDetail(conversations.first { it.id == id }, messages[id].orEmpty())

    override suspend fun createConversation(request: ConversationWriteRequest): Conversation {
        val conversation = Conversation(id = "conversation-created", model = request.model ?: model.id)
        conversations += conversation
        return conversation
    }

    override suspend fun updateConversation(id: String, request: ConversationWriteRequest): Conversation {
        val index = conversations.indexOfFirst { it.id == id }
        val updated = conversations[index].copy(title = request.title ?: conversations[index].title)
        conversations[index] = updated
        return updated
    }

    override suspend fun deleteConversations(ids: Set<String>) { conversations.removeAll { it.id in ids } }
    override suspend fun deleteBookmarks(messageIds: Set<String>) { bookmarks.removeAll { it.messageId in messageIds } }
    override suspend fun deleteNotes(ids: Set<String>) { notes.removeAll { it.id in ids } }
    override suspend fun generateTitle(id: String, force: Boolean) = updateConversation(id, ConversationWriteRequest(title = "Title"))

    override fun sendMessage(id: String, request: SendMessageRequest): Flow<ChatEvent> = flow {
        sentRequest = request
        val user = ChatMessage("user", id, requestId = request.requestId, role = "user", content = request.content)
        val assistant = ChatMessage("assistant", id, requestId = request.requestId, role = "assistant", status = "generating")
        emit(ChatEvent.UserMessage(user))
        emit(ChatEvent.AssistantMessage(assistant))
        emit(ChatEvent.Process(ProcessEvent(type = "thinking", id = "thinking", content = "summary")))
        emit(ChatEvent.Delta("Answer from provider"))
        messages[id] = listOf(user, assistant.copy(content = "Answer from provider", status = "completed"))
        emit(ChatEvent.Done(Usage(600, 600), false))
    }

    override fun regenerate(id: String, request: SendMessageRequest): Flow<ChatEvent> = flow { }
    override suspend fun exportConfiguration(password: CharArray) = "archive"
    override suspend fun previewImport(raw: String, password: CharArray): ImportPreview = error("not used")
    override suspend fun applyImport(preview: ImportPreview) = Unit
}
