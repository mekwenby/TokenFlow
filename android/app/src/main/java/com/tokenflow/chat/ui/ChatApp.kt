package com.tokenflow.chat.ui

import android.graphics.BitmapFactory
import android.net.Uri
import android.provider.OpenableColumns
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.horizontalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.automirrored.outlined.Send
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.AddAPhoto
import androidx.compose.material.icons.outlined.AttachFile
import androidx.compose.material.icons.outlined.AutoAwesome
import androidx.compose.material.icons.outlined.Archive
import androidx.compose.material.icons.outlined.Bookmarks
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.CallSplit
import androidx.compose.material.icons.outlined.DeleteOutline
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.ExpandLess
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material.icons.outlined.FileDownload
import androidx.compose.material.icons.outlined.FileUpload
import androidx.compose.material.icons.outlined.Hub
import androidx.compose.material.icons.outlined.Key
import androidx.compose.material.icons.outlined.Link
import androidx.compose.material.icons.outlined.FolderOpen
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.Menu
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.KeyboardArrowDown
import androidx.compose.material.icons.outlined.NoteAlt
import androidx.compose.material.icons.outlined.PushPin
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Pause
import androidx.compose.material.icons.outlined.PhotoCamera
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.SelectAll
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.SmartToy
import androidx.compose.material.icons.outlined.Stop
import androidx.compose.material.icons.outlined.Tune
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material.icons.outlined.VisibilityOff
import androidx.compose.material.icons.outlined.VolumeUp
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.FilledTonalIconButton
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalDrawerSheet
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.rememberDrawerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.snapshotFlow
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.intl.Locale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tokenflow.chat.R
import com.tokenflow.chat.data.ChatDisplayPreferences
import com.tokenflow.chat.data.ChatMessage
import com.tokenflow.chat.data.CameraCaptureStore
import com.tokenflow.chat.data.Conversation
import com.tokenflow.chat.data.GlobalChatSettings
import com.tokenflow.chat.data.LocalAvatarFile
import com.tokenflow.chat.data.LocalAvatarImages
import com.tokenflow.chat.data.LocalAvatarKind
import com.tokenflow.chat.data.LocalAvatarStore
import com.tokenflow.chat.data.ModelProfile
import com.tokenflow.chat.data.MessageAttachment
import com.tokenflow.chat.data.PendingAttachment
import com.tokenflow.chat.data.MimoTtsClient
import com.tokenflow.chat.data.ProcessEvent
import com.tokenflow.chat.data.PromptTemplate
import com.tokenflow.chat.data.ProviderDraft
import com.tokenflow.chat.data.ProviderProtocol
import com.tokenflow.chat.data.RemoteModel
import com.tokenflow.chat.data.SettingMode
import com.tokenflow.chat.data.SystemPrompts
import com.tokenflow.chat.data.UrlReaderBackend
import com.tokenflow.chat.data.VisionStatus
import com.tokenflow.chat.data.assistantMetadata
import java.util.UUID
import java.io.File
import kotlin.math.roundToInt
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

object UiTestTags {
    const val MESSAGE_INPUT = "message_input"
    const val MESSAGE_ACTION = "message_action"
    const val TAKE_PHOTO = "take_photo"
    const val PROCESS_DETAILS = "process_details"
    const val SETTINGS = "settings"
    const val OPEN_CONVERSATIONS = "open_conversations"
    const val DELETE_SELECTED = "delete_selected"
    const val RENAME_SELECTED = "rename_selected"
    const val USER_MESSAGE_AVATAR = "user_message_avatar"
    const val ASSISTANT_MESSAGE_AVATAR = "assistant_message_avatar"
    const val COPY_ASSISTANT_MESSAGE = "copy_assistant_message"
    const val SPEECH_ACTION = "speech_action"
    const val SPEECH_CONTROLS = "speech_controls"
    const val TOKEN_USAGE = "token_usage"
    const val CHAT_FONT_SIZE = "chat_font_size"
    const val USER_AVATAR_PICKER = "user_avatar_picker"
    const val ASSISTANT_AVATAR_PICKER = "assistant_avatar_picker"
    const val PROVIDER_NAME = "provider_name"
    const val PROVIDER_BASE_URL = "provider_base_url"
    const val PROVIDER_API_KEY = "provider_api_key"
    const val PROVIDER_SAVE = "provider_save"
    const val PROVIDER_GUIDE = "provider_guide"
    const val ADD_PROVIDER = "add_provider"
    const val EXPAND_DESTINATIONS = "expand_destinations"
    const val WORKSPACE_SELECT_ALL = "workspace_select_all"
    const val WORKSPACE_DELETE_SELECTED = "workspace_delete_selected"

    fun conversationItem(id: String) = "conversation_item_$id"
    fun bookmarkItem(id: String) = "bookmark_item_$id"
    fun noteItem(id: String) = "note_item_$id"
}

@Composable
fun TokenFlowApp(viewModel: AppViewModel) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val snackbar = remember { SnackbarHostState() }
    val scope = rememberCoroutineScope()
    val avatarStore = remember(context.applicationContext) { LocalAvatarStore(context.applicationContext) }
    val displayPreferences = remember(context.applicationContext) { ChatDisplayPreferences(context.applicationContext) }
    var globalAvatars by remember { mutableStateOf(avatarStore.read()) }
    var scopedAvatars by remember { mutableStateOf(LocalAvatarImages()) }
    var chatFontScale by remember { mutableFloatStateOf(displayPreferences.readFontScale()) }
    val effectiveAvatars = LocalAvatarImages(
        user = if (state.config.userAvatarMode == SettingMode.INHERIT) globalAvatars.user else scopedAvatars.user,
        assistant = if (state.config.assistantAvatarMode == SettingMode.INHERIT) globalAvatars.assistant else scopedAvatars.assistant,
    )
    val userAvatarImage = rememberLocalAvatarImage(effectiveAvatars.user)
    val assistantAvatarImage = rememberLocalAvatarImage(effectiveAvatars.assistant)
    var showExportPassword by rememberSaveable { mutableStateOf(false) }
    var pendingImport by remember { mutableStateOf<String?>(null) }
    val avatarError = stringResource(R.string.avatar_save_failed)
    val fileError = stringResource(R.string.file_operation_failed)
    val exportContent = state.transfer.exportContent

    val createConfig = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/json")) { uri ->
        val content = exportContent
        if (uri == null || content == null) {
            viewModel.consumeExport()
        } else scope.launch {
            runCatching {
                withContext(Dispatchers.IO) {
                    context.contentResolver.openOutputStream(uri)?.bufferedWriter()?.use { it.write(content) }
                        ?: error("Unable to open output file")
                }
            }.onFailure { snackbar.showSnackbar(fileError) }
            viewModel.consumeExport()
        }
    }
    val openConfig = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        uri ?: return@rememberLauncherForActivityResult
        scope.launch {
            runCatching {
                withContext(Dispatchers.IO) {
                    context.contentResolver.openInputStream(uri)?.bufferedReader()?.use { it.readText() }
                        ?: error("Unable to open configuration")
                }
            }.onSuccess { pendingImport = it }
                .onFailure { snackbar.showSnackbar(fileError) }
        }
    }

    LaunchedEffect(exportContent) {
        if (exportContent != null) createConfig.launch("tokenflow-${System.currentTimeMillis()}.tfcfg")
    }
    LaunchedEffect(state.activeConversationId) {
        scopedAvatars = withContext(Dispatchers.IO) {
            state.activeConversationId?.let { id ->
                val draft = avatarStore.readDraft()
                if (draft.user != null || draft.assistant != null) avatarStore.promoteDraft(id)
                else avatarStore.readConversation(id)
            } ?: avatarStore.readDraft()
        }
    }
    val notice = state.notice?.resolve()
    LaunchedEffect(notice) {
        notice?.let { snackbar.showSnackbar(it); viewModel.clearNotice() }
    }

    Box(Modifier.fillMaxSize()) {
        if (state.phase == AppPhase.LOADING) {
            LoadingScreen()
        } else {
            when (state.screen) {
                AppScreen.CHAT -> ChatWorkspace(
                    state = state,
                    viewModel = viewModel,
                    avatars = effectiveAvatars,
                    userAvatarImage = userAvatarImage,
                    assistantAvatarImage = assistantAvatarImage,
                    chatFontScale = chatFontScale,
                    onFontScale = { value -> displayPreferences.writeFontScale(value); chatFontScale = value },
                    onAvatarSelected = { kind, uri ->
                        scope.launch {
                            runCatching {
                                withContext(Dispatchers.IO) {
                                    state.activeConversationId?.let { avatarStore.saveConversation(it, kind, uri) }
                                        ?: avatarStore.saveDraft(kind, uri)
                                }
                            }.onSuccess { scopedAvatars = it }
                                .onFailure { snackbar.showSnackbar(avatarError) }
                        }
                    },
                    onAvatarRemoved = { kind ->
                        scope.launch {
                            scopedAvatars = withContext(Dispatchers.IO) {
                                state.activeConversationId?.let { avatarStore.removeConversation(it, kind) }
                                    ?: avatarStore.removeDraft(kind)
                            }
                        }
                    },
                    onDiscardDraft = {
                        withContext(Dispatchers.IO) { avatarStore.clearDraft() }
                        scopedAvatars = LocalAvatarImages()
                    },
                )
                AppScreen.GLOBAL_SETTINGS -> AdaptiveConfigurationShell(state, viewModel) { GlobalSettingsScreen(
                    state = state,
                    viewModel = viewModel,
                    avatars = globalAvatars,
                    onAvatarSelected = { kind, uri ->
                        scope.launch {
                            runCatching { withContext(Dispatchers.IO) { avatarStore.save(kind, uri) } }
                                .onSuccess { globalAvatars = it }
                                .onFailure { snackbar.showSnackbar(avatarError) }
                        }
                    },
                    onAvatarRemoved = { kind ->
                        scope.launch { globalAvatars = withContext(Dispatchers.IO) { avatarStore.remove(kind) } }
                    },
                ) }
                AppScreen.PROVIDERS -> AdaptiveConfigurationShell(state, viewModel) { ProvidersScreen(state, viewModel) }
                AppScreen.EXA -> AdaptiveConfigurationShell(state, viewModel) { ExaScreen(state, viewModel) }
                AppScreen.TRANSFER -> AdaptiveConfigurationShell(state, viewModel) { TransferScreen(
                    state = state,
                    onBack = { viewModel.openScreen(AppScreen.CHAT) },
                    onExport = { showExportPassword = true },
                    onImport = { openConfig.launch(arrayOf("application/json", "application/octet-stream", "text/plain")) },
                ) }
                AppScreen.BOOKMARKS, AppScreen.NOTES, AppScreen.AGENTS, AppScreen.KNOWLEDGE, AppScreen.ABOUT ->
                    WorkspaceScreen(state, viewModel)
            }
        }
        SnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter).navigationBarsPadding().padding(12.dp))
    }

    if (showExportPassword) PasswordDialog(
        title = stringResource(R.string.export_configuration),
        onDismiss = { showExportPassword = false },
        onConfirm = { password -> showExportPassword = false; viewModel.exportConfiguration(password.toCharArray()) },
    )
    pendingImport?.let { raw ->
        PasswordDialog(
            title = stringResource(R.string.import_configuration),
            onDismiss = { pendingImport = null },
            onConfirm = { password -> pendingImport = null; viewModel.previewImport(raw, password.toCharArray()) },
        )
    }
    state.transfer.importPreview?.let { preview ->
        AlertDialog(
            onDismissRequest = viewModel::cancelImportPreview,
            title = { Text(stringResource(R.string.import_preview)) },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(stringResource(R.string.import_summary, preview.newProviders, preview.updatedProviders, preview.newModels, preview.updatedModels))
                    Text(stringResource(R.string.import_agents_summary, preview.newAgents, preview.updatedAgents))
                    if (preview.replacesExaKey) Text(stringResource(R.string.import_replaces_exa))
                    if (preview.replacesMimoKey) Text(stringResource(R.string.import_replaces_mimo))
                    if (preview.replacesInfoFlowKey) Text(stringResource(R.string.import_replaces_infoflow))
                    if (preview.conflicts.isNotEmpty()) {
                        Text(stringResource(R.string.import_conflicts, preview.conflicts.size), fontWeight = FontWeight.SemiBold)
                        preview.conflicts.take(5).forEach { Text("• $it", style = MaterialTheme.typography.bodySmall) }
                    }
                }
            },
            confirmButton = { Button(onClick = viewModel::applyImport, enabled = !state.transfer.busy) { Text(stringResource(R.string.import_action)) } },
            dismissButton = { TextButton(onClick = viewModel::cancelImportPreview) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

@Composable
private fun rememberLocalAvatarImage(file: LocalAvatarFile?): ImageBitmap? {
    val image by produceState<ImageBitmap?>(null, file?.path, file?.lastModified) {
        value = file?.let { withContext(Dispatchers.IO) { BitmapFactory.decodeFile(it.path)?.asImageBitmap() } }
    }
    return image
}

@Composable
private fun LoadingScreen() = Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
    CircularProgressIndicator()
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ProvidersScreen(state: AppUiState, viewModel: AppViewModel) {
    val canBack = state.models.isNotEmpty()
    BackHandler(enabled = canBack && state.providerEditor == null) { viewModel.openScreen(AppScreen.CHAT) }
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(if (state.providerEditor == null) stringResource(R.string.providers_models) else stringResource(R.string.edit_provider)) },
                navigationIcon = {
                    if (state.providerEditor != null) IconButton(onClick = viewModel::closeProviderEditor) {
                        Icon(Icons.AutoMirrored.Outlined.ArrowBack, stringResource(R.string.back))
                    } else if (canBack) IconButton(onClick = { viewModel.openScreen(AppScreen.CHAT) }) {
                        Icon(Icons.AutoMirrored.Outlined.ArrowBack, stringResource(R.string.back))
                    }
                },
            )
        },
        floatingActionButton = {
            if (state.providerEditor == null && state.providers.isNotEmpty()) FloatingActionButton(onClick = viewModel::beginNewProvider, modifier = Modifier.testTag(UiTestTags.ADD_PROVIDER)) {
                Icon(Icons.Outlined.Add, stringResource(R.string.add_provider))
            }
        },
        contentWindowInsets = WindowInsets.safeDrawing,
    ) { padding ->
        if (state.providerEditor != null) ProviderEditor(state, viewModel, Modifier.padding(padding))
        else ProviderList(state, viewModel, Modifier.padding(padding))
    }
}

@Composable
private fun ProviderList(state: AppUiState, viewModel: AppViewModel, modifier: Modifier) {
    LazyColumn(
        modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp, 12.dp, 16.dp, 96.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        if (state.models.isEmpty()) item {
            Surface(color = MaterialTheme.colorScheme.secondaryContainer, shape = RoundedCornerShape(8.dp), modifier = Modifier.testTag(UiTestTags.PROVIDER_GUIDE)) {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(stringResource(R.string.first_provider_title), style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                    Text(stringResource(R.string.first_provider_detail))
                }
            }
        }
        items(state.providers, key = { it.id }) { provider ->
            Surface(shape = RoundedCornerShape(8.dp), tonalElevation = 1.dp) {
                ListItem(
                    headlineContent = { Text(provider.name, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                    supportingContent = {
                        val count = state.models.count { it.providerId == provider.id }
                        Text("${protocolLabel(provider.protocol)} · ${stringResource(R.string.model_count, count)}")
                    },
                    leadingContent = { Icon(Icons.Outlined.Hub, null) },
                    trailingContent = {
                        Row {
                            IconButton(onClick = { viewModel.editProvider(provider.id) }) { Icon(Icons.Outlined.Edit, stringResource(R.string.edit)) }
                            IconButton(onClick = { viewModel.deleteProvider(provider.id) }) { Icon(Icons.Outlined.DeleteOutline, stringResource(R.string.delete)) }
                        }
                    },
                )
            }
        }
        if (state.providers.isEmpty()) item {
            OutlinedButton(onClick = viewModel::beginNewProvider, shape = RoundedCornerShape(8.dp), modifier = Modifier.fillMaxWidth().testTag(UiTestTags.ADD_PROVIDER)) {
                Icon(Icons.Outlined.Add, null); Spacer(Modifier.width(8.dp)); Text(stringResource(R.string.add_provider))
            }
        }
    }
}

@Composable
private fun ProviderEditor(state: AppUiState, viewModel: AppViewModel, modifier: Modifier) {
    val editor = state.providerEditor ?: return
    var showKey by rememberSaveable(editor.draft.id) { mutableStateOf(false) }
    var manualModel by rememberSaveable(editor.draft.id) { mutableStateOf("") }
    Column(
        modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        OutlinedTextField(
            value = editor.draft.name,
            onValueChange = { viewModel.updateProviderDraft(editor.draft.copy(name = it)) },
            label = { Text(stringResource(R.string.provider_name)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth().testTag(UiTestTags.PROVIDER_NAME),
        )
        Text(stringResource(R.string.protocol), style = MaterialTheme.typography.labelLarge)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            ProviderProtocol.entries.forEach { protocol ->
                FilterChip(
                    selected = editor.draft.protocol == protocol,
                    onClick = { viewModel.updateProviderDraft(editor.draft.copy(protocol = protocol)) },
                    label = { Text(protocolShortLabel(protocol)) },
                )
            }
        }
        OutlinedTextField(
            value = editor.draft.baseUrl,
            onValueChange = { viewModel.updateProviderDraft(editor.draft.copy(baseUrl = it)) },
            label = { Text(stringResource(R.string.api_base_url)) },
            supportingText = { Text(stringResource(R.string.api_base_url_hint)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth().testTag(UiTestTags.PROVIDER_BASE_URL),
        )
        OutlinedTextField(
            value = editor.draft.apiKey,
            onValueChange = { viewModel.updateProviderDraft(editor.draft.copy(apiKey = it)) },
            label = { Text(stringResource(R.string.api_key)) },
            singleLine = true,
            visualTransformation = if (showKey) VisualTransformation.None else PasswordVisualTransformation(),
            trailingIcon = {
                IconButton(onClick = { showKey = !showKey }) {
                    Icon(if (showKey) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility, stringResource(R.string.toggle_key_visibility))
                }
            },
            modifier = Modifier.fillMaxWidth().testTag(UiTestTags.PROVIDER_API_KEY),
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = viewModel::fetchProviderModels, enabled = !editor.busy) {
                if (editor.busy) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                else Icon(Icons.Outlined.Refresh, null)
                Spacer(Modifier.width(8.dp)); Text(stringResource(R.string.test_and_fetch_models))
            }
        }
        editor.error?.let { Text(it.resolve(), color = MaterialTheme.colorScheme.error) }

        if (editor.remoteModels.isNotEmpty()) {
            Text(stringResource(R.string.available_models), style = MaterialTheme.typography.titleSmall)
            OutlinedTextField(
                value = editor.modelSearch,
                onValueChange = viewModel::setProviderModelSearch,
                label = { Text(stringResource(R.string.search_models)) },
                leadingIcon = { Icon(Icons.Outlined.Search, null) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            editor.remoteModels.filter {
                editor.modelSearch.isBlank() || it.id.contains(editor.modelSearch, true) || it.displayName.contains(editor.modelSearch, true)
            }.take(100).forEach { remote ->
                val selected = editor.selectedModels.any { it.remoteId == remote.id }
                Row(
                    Modifier.fillMaxWidth().clickable { viewModel.toggleProviderModel(remote) }.padding(vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Checkbox(selected, onCheckedChange = { viewModel.toggleProviderModel(remote) })
                    Column { Text(remote.displayName); if (remote.displayName != remote.id) Text(remote.id, style = MaterialTheme.typography.bodySmall) }
                }
            }
        }

        Text(stringResource(R.string.manual_model), style = MaterialTheme.typography.titleSmall)
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedTextField(
                value = manualModel,
                onValueChange = { manualModel = it },
                label = { Text(stringResource(R.string.model_id)) },
                singleLine = true,
                modifier = Modifier.weight(1f),
                keyboardActions = KeyboardActions(onDone = { viewModel.addManualModel(manualModel); manualModel = "" }),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
            )
            IconButton(onClick = { viewModel.addManualModel(manualModel); manualModel = "" }) { Icon(Icons.Outlined.Add, stringResource(R.string.add)) }
        }

        if (editor.selectedModels.isNotEmpty()) {
            Text(stringResource(R.string.added_models), style = MaterialTheme.typography.titleSmall)
            editor.selectedModels.forEach { model ->
                SelectedModelEditor(
                    model,
                    visionTesting = state.visionTestingModelId == model.id,
                    canTestVision = state.models.any { it.id == model.id },
                    viewModel = viewModel,
                )
            }
        }
        Spacer(Modifier.height(4.dp))
        Button(
            onClick = viewModel::saveProvider,
            enabled = !editor.busy,
            modifier = Modifier.fillMaxWidth().testTag(UiTestTags.PROVIDER_SAVE),
        ) { Icon(Icons.Outlined.Check, null); Spacer(Modifier.width(8.dp)); Text(stringResource(R.string.save_provider)) }
        Spacer(Modifier.navigationBarsPadding())
    }
}

@Composable
private fun SelectedModelEditor(model: ModelProfile, visionTesting: Boolean, canTestVision: Boolean, viewModel: AppViewModel) {
    var alias by remember(model.id, model.displayName) { mutableStateOf(model.displayName) }
    var maxTokens by remember(model.id, model.maxOutputTokens) { mutableStateOf(model.maxOutputTokens.toString()) }
    Surface(shape = RoundedCornerShape(8.dp), tonalElevation = 1.dp) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                RadioButton(
                    selected = model.isDefault,
                    onClick = { viewModel.updateSelectedModel(model.id, alias, maxTokens.toIntOrNull() ?: 4096, true) },
                )
                Column(Modifier.weight(1f)) {
                    Text(model.remoteId, fontWeight = FontWeight.Medium)
                    Text(stringResource(R.string.default_model), style = MaterialTheme.typography.bodySmall)
                }
                IconButton(onClick = { viewModel.removeSelectedModel(model.id) }) { Icon(Icons.Outlined.Close, stringResource(R.string.remove)) }
            }
            OutlinedTextField(
                value = alias,
                onValueChange = { alias = it; viewModel.updateSelectedModel(model.id, it, maxTokens.toIntOrNull() ?: 4096, model.isDefault) },
                label = { Text(stringResource(R.string.model_alias)) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = maxTokens,
                onValueChange = { value ->
                    maxTokens = value.filter(Char::isDigit)
                    viewModel.updateSelectedModel(model.id, alias, maxTokens.toIntOrNull() ?: 4096, model.isDefault)
                },
                label = { Text(stringResource(R.string.max_output_tokens)) },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedButton(
                onClick = { viewModel.testModelVision(model.id) },
                enabled = !visionTesting && canTestVision,
                modifier = Modifier.fillMaxWidth(),
            ) {
                if (visionTesting) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                else Icon(Icons.Outlined.Visibility, null, Modifier.size(18.dp))
                Spacer(Modifier.width(8.dp))
                Text("${stringResource(R.string.vision_test)} · ${visionStatusLabel(model.visionStatus)}")
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ExaScreen(state: AppUiState, viewModel: AppViewModel) {
    var key by rememberSaveable { mutableStateOf("") }
    var visible by rememberSaveable { mutableStateOf(false) }
    var query by rememberSaveable { mutableStateOf("") }
    Scaffold(
        topBar = { TopAppBar(
            title = { Text(stringResource(R.string.exa_search)) },
            navigationIcon = { IconButton(onClick = { viewModel.openScreen(AppScreen.CHAT) }) { Icon(Icons.AutoMirrored.Outlined.ArrowBack, stringResource(R.string.back)) } },
        ) },
        contentWindowInsets = WindowInsets.safeDrawing,
    ) { padding ->
        Column(Modifier.padding(padding).fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
            Text(stringResource(if (state.exaConfigured) R.string.exa_configured else R.string.exa_not_configured))
            OutlinedTextField(
                value = key,
                onValueChange = { key = it },
                label = { Text(stringResource(R.string.exa_api_key)) },
                visualTransformation = if (visible) VisualTransformation.None else PasswordVisualTransformation(),
                trailingIcon = { IconButton(onClick = { visible = !visible }) { Icon(if (visible) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility, null) } },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            Button(onClick = { viewModel.saveExaKey(key); key = "" }, modifier = Modifier.fillMaxWidth()) { Text(stringResource(R.string.save)) }
            if (state.exaConfigured) OutlinedButton(onClick = { viewModel.saveExaKey("") }, modifier = Modifier.fillMaxWidth()) {
                Text(stringResource(R.string.remove_exa_key))
            }
            if (state.exaConfigured) {
                HorizontalDivider()
                OutlinedTextField(query, { query = it }, label = { Text(stringResource(R.string.search_web)) }, leadingIcon = { Icon(Icons.Outlined.Search, null) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                Button(onClick = { viewModel.testExa(query) }, enabled = query.isNotBlank(), shape = RoundedCornerShape(8.dp), modifier = Modifier.fillMaxWidth()) { Text(stringResource(R.string.test_search)) }
                if (state.exaTestResult.isNotBlank()) Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = RoundedCornerShape(8.dp)) {
                    SelectionContainer { Text(state.exaTestResult, Modifier.padding(12.dp), style = MaterialTheme.typography.bodySmall) }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun GlobalSettingsScreen(
    state: AppUiState,
    viewModel: AppViewModel,
    avatars: LocalAvatarImages,
    onAvatarSelected: (LocalAvatarKind, Uri) -> Unit,
    onAvatarRemoved: (LocalAvatarKind) -> Unit,
) {
    var draft by remember(state.globalSettings) { mutableStateOf(state.globalSettings) }
    var mimoKey by rememberSaveable { mutableStateOf("") }
    var mimoKeyVisible by rememberSaveable { mutableStateOf(false) }
    var urlTest by rememberSaveable { mutableStateOf("https://example.com") }
    var modelMenu by remember { mutableStateOf(false) }
    var fallbackMenu by remember { mutableStateOf(false) }
    var voiceMenu by remember { mutableStateOf(false) }
    var templateMenu by remember { mutableStateOf(false) }
    var pendingTemplate by remember { mutableStateOf<PromptTemplate?>(null) }
    val selectedModel = state.models.firstOrNull { it.id == draft.defaultModelId }
    val selectedFallback = state.models.firstOrNull { it.id == draft.visionFallbackModelId }
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.global_settings)) },
                navigationIcon = {
                    IconButton(onClick = { viewModel.openScreen(AppScreen.CHAT) }) {
                        Icon(Icons.AutoMirrored.Outlined.ArrowBack, stringResource(R.string.back))
                    }
                },
            )
        },
        contentWindowInsets = WindowInsets.safeDrawing,
    ) { padding ->
        Column(
            Modifier.padding(padding).fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Text(stringResource(R.string.default_model), style = MaterialTheme.typography.labelLarge)
            Box {
                OutlinedButton(onClick = { modelMenu = true }, modifier = Modifier.fillMaxWidth()) {
                    Text(selectedModel?.displayName ?: stringResource(R.string.select_model), Modifier.weight(1f))
                }
                DropdownMenu(expanded = modelMenu, onDismissRequest = { modelMenu = false }) {
                    state.models.forEach { model -> DropdownMenuItem(
                        text = { Text(model.displayName) },
                        onClick = { draft = draft.copy(defaultModelId = model.id); modelMenu = false },
                    ) }
                }
            }
            Text(stringResource(R.string.vision_fallback_model), style = MaterialTheme.typography.labelLarge)
            Box {
                OutlinedButton(onClick = { fallbackMenu = true }, modifier = Modifier.fillMaxWidth()) {
                    Text(selectedFallback?.displayName ?: stringResource(R.string.not_configured), Modifier.weight(1f))
                }
                DropdownMenu(expanded = fallbackMenu, onDismissRequest = { fallbackMenu = false }) {
                    DropdownMenuItem(text = { Text(stringResource(R.string.not_configured)) }, onClick = {
                        draft = draft.copy(visionFallbackModelId = null); fallbackMenu = false
                    })
                    state.models.filter { it.visionStatus == VisionStatus.SUPPORTED }.forEach { model ->
                        DropdownMenuItem(text = { Text(model.displayName) }, onClick = {
                            draft = draft.copy(visionFallbackModelId = model.id); fallbackMenu = false
                        })
                    }
                }
            }
            Text(stringResource(R.string.prompt_template), style = MaterialTheme.typography.labelLarge)
            Box {
                OutlinedButton(onClick = { templateMenu = true }, modifier = Modifier.fillMaxWidth()) {
                    Text(stringResource(R.string.choose_template), Modifier.weight(1f))
                }
                DropdownMenu(expanded = templateMenu, onDismissRequest = { templateMenu = false }) {
                    SystemPrompts.templates.forEach { template -> DropdownMenuItem(
                        text = { Text(if (Locale.current.language == "zh") template.titleZh else template.titleEn) },
                        onClick = {
                            templateMenu = false
                            if (draft.systemPrompt.isNotBlank() && draft.systemPrompt != template.content) pendingTemplate = template
                            else draft = draft.copy(systemPrompt = template.content)
                        },
                    ) }
                }
            }
            OutlinedTextField(
                value = draft.systemPrompt,
                onValueChange = { draft = draft.copy(systemPrompt = it) },
                label = { Text(stringResource(R.string.global_system_prompt)) },
                minLines = 5,
                modifier = Modifier.fillMaxWidth(),
            )
            LocalAvatarSetting(LocalAvatarKind.USER, stringResource(R.string.global_user_avatar), avatars.user, onAvatarSelected, onAvatarRemoved, UiTestTags.USER_AVATAR_PICKER)
            LocalAvatarSetting(LocalAvatarKind.ASSISTANT, stringResource(R.string.global_assistant_avatar), avatars.assistant, onAvatarSelected, onAvatarRemoved, UiTestTags.ASSISTANT_AVATAR_PICKER)
            Text(stringResource(R.string.url_reader), style = MaterialTheme.typography.labelLarge)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                FilterChip(
                    selected = draft.urlReaderBackend == UrlReaderBackend.BUILT_IN,
                    onClick = { draft = draft.copy(urlReaderBackend = UrlReaderBackend.BUILT_IN) },
                    label = { Text(stringResource(R.string.url_reader_builtin)) },
                )
                FilterChip(
                    selected = draft.urlReaderBackend == UrlReaderBackend.INFOFLOW,
                    onClick = { draft = draft.copy(urlReaderBackend = UrlReaderBackend.INFOFLOW) },
                    label = { Text("InfoFlow") },
                )
            }
            OutlinedTextField(
                value = urlTest,
                onValueChange = { urlTest = it },
                label = { Text(stringResource(R.string.url_test)) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedButton(
                onClick = { viewModel.testUrl(urlTest) },
                enabled = urlTest.isNotBlank() && !state.urlTestBusy,
                modifier = Modifier.fillMaxWidth(),
            ) {
                if (state.urlTestBusy) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                else Icon(Icons.Outlined.Link, null, Modifier.size(18.dp))
                Spacer(Modifier.width(8.dp)); Text(stringResource(R.string.test_url_reader))
            }
            state.urlTestResult?.let { result ->
                Surface(shape = RoundedCornerShape(8.dp), color = if (result.success) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.errorContainer) {
                    Column(Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text("${result.source} · ${result.elapsedMs} ms", fontWeight = FontWeight.SemiBold)
                        Text(result.finalUrl, style = MaterialTheme.typography.bodySmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
                        Text(result.detail, style = MaterialTheme.typography.bodySmall)
                    }
                }
            }
            HorizontalDivider()
            Text(stringResource(R.string.mimo_tts), style = MaterialTheme.typography.labelLarge)
            OutlinedTextField(
                value = mimoKey,
                onValueChange = { mimoKey = it },
                label = { Text(stringResource(R.string.mimo_api_key)) },
                visualTransformation = if (mimoKeyVisible) VisualTransformation.None else PasswordVisualTransformation(),
                trailingIcon = { IconButton(onClick = { mimoKeyVisible = !mimoKeyVisible }) {
                    Icon(if (mimoKeyVisible) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility, null)
                } },
                supportingText = { Text(stringResource(if (state.globalSettings.mimoTtsConfigured) R.string.mimo_configured else R.string.mimo_not_configured)) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            Box {
                OutlinedButton(onClick = { voiceMenu = true }, modifier = Modifier.fillMaxWidth()) {
                    Text(draft.mimoTtsVoice, Modifier.weight(1f))
                }
                DropdownMenu(expanded = voiceMenu, onDismissRequest = { voiceMenu = false }) {
                    MimoTtsClient.VOICES.forEach { voice -> DropdownMenuItem(text = { Text(voice) }, onClick = {
                        draft = draft.copy(mimoTtsVoice = voice); voiceMenu = false
                    }) }
                }
            }
            if (state.globalSettings.mimoTtsConfigured) OutlinedButton(
                onClick = { viewModel.saveGlobalSettings(draft, "") },
                modifier = Modifier.fillMaxWidth(),
            ) { Text(stringResource(R.string.remove_mimo_key)) }
            Button(
                onClick = { viewModel.saveGlobalSettings(draft, mimoKey.takeIf(String::isNotBlank)); mimoKey = "" },
                enabled = draft.defaultModelId != null,
                modifier = Modifier.fillMaxWidth(),
            ) { Text(stringResource(R.string.save)) }
            Spacer(Modifier.navigationBarsPadding())
        }
    }
    pendingTemplate?.let { template ->
        AlertDialog(
            onDismissRequest = { pendingTemplate = null },
            title = { Text(stringResource(R.string.replace_prompt_title)) },
            text = { Text(stringResource(R.string.replace_prompt_detail)) },
            confirmButton = { Button(onClick = { draft = draft.copy(systemPrompt = template.content); pendingTemplate = null }) { Text(stringResource(R.string.replace)) } },
            dismissButton = { TextButton(onClick = { pendingTemplate = null }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun TransferScreen(state: AppUiState, onBack: () -> Unit, onExport: () -> Unit, onImport: () -> Unit) {
    Scaffold(
        topBar = { TopAppBar(
            title = { Text(stringResource(R.string.import_export)) },
            navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Outlined.ArrowBack, stringResource(R.string.back)) } },
        ) },
        contentWindowInsets = WindowInsets.safeDrawing,
    ) { padding ->
        Column(Modifier.padding(padding).fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(stringResource(R.string.transfer_detail))
            Button(onClick = onExport, enabled = !state.transfer.busy, modifier = Modifier.fillMaxWidth()) {
                Icon(Icons.Outlined.FileDownload, null); Spacer(Modifier.width(8.dp)); Text(stringResource(R.string.export_configuration))
            }
            OutlinedButton(onClick = onImport, enabled = !state.transfer.busy, modifier = Modifier.fillMaxWidth()) {
                Icon(Icons.Outlined.FileUpload, null); Spacer(Modifier.width(8.dp)); Text(stringResource(R.string.import_configuration))
            }
            state.transfer.error?.let { Text(it.resolve(), color = MaterialTheme.colorScheme.error) }
            if (state.transfer.busy) CircularProgressIndicator(Modifier.align(Alignment.CenterHorizontally))
        }
    }
}

@Composable
private fun ChatWorkspace(
    state: AppUiState,
    viewModel: AppViewModel,
    avatars: LocalAvatarImages,
    userAvatarImage: ImageBitmap?,
    assistantAvatarImage: ImageBitmap?,
    chatFontScale: Float,
    onFontScale: (Float) -> Unit,
    onAvatarSelected: (LocalAvatarKind, Uri) -> Unit,
    onAvatarRemoved: (LocalAvatarKind) -> Unit,
    onDiscardDraft: suspend () -> Unit,
) {
    BoxWithConstraints(Modifier.fillMaxSize()) {
        val wide = maxWidth >= 600.dp
        val inspector = maxWidth >= 1000.dp
        val drawer = rememberDrawerState(DrawerValue.Closed)
        val scope = rememberCoroutineScope()
        val sidebar: @Composable () -> Unit = {
            ConversationSidebar(
                state = state,
                viewModel = viewModel,
                onNewConversation = {
                    scope.launch { onDiscardDraft(); viewModel.newConversation(); drawer.close() }
                },
                onConversation = { id ->
                    scope.launch { onDiscardDraft(); viewModel.openConversation(id); drawer.close() }
                },
            )
        }
        val chat: @Composable () -> Unit = {
            ChatPane(
                state,
                viewModel,
                showMenu = !wide,
                onMenu = { scope.launch { drawer.open() } },
                avatars = avatars,
                userAvatarImage = userAvatarImage,
                assistantAvatarImage = assistantAvatarImage,
                chatFontScale = chatFontScale,
                onFontScale = onFontScale,
                onAvatarSelected = onAvatarSelected,
                onAvatarRemoved = onAvatarRemoved,
            )
        }
        if (wide) Row(Modifier.fillMaxSize()) {
            Box(Modifier.width(if (inspector) 280.dp else 264.dp).fillMaxHeight()) { sidebar() }
            Box(Modifier.width(1.dp).fillMaxHeight().background(MaterialTheme.colorScheme.outlineVariant))
            Box(Modifier.weight(1f).fillMaxHeight()) { chat() }
            if (inspector) {
                Box(Modifier.width(1.dp).fillMaxHeight().background(MaterialTheme.colorScheme.outlineVariant))
                JadeInspector(state, viewModel, Modifier.width(340.dp).fillMaxHeight())
            }
        } else ModalNavigationDrawer(
            drawerState = drawer,
            drawerContent = { ModalDrawerSheet(Modifier.width(304.dp), drawerContainerColor = MaterialTheme.colorScheme.surface) { sidebar() } },
        ) { chat() }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun ConversationSidebar(
    state: AppUiState,
    viewModel: AppViewModel,
    onNewConversation: () -> Unit,
    onConversation: (String) -> Unit,
) {
    var selected by remember { mutableStateOf(setOf<String>()) }
    var destinationsExpanded by rememberSaveable { mutableStateOf(false) }
    var renameId by remember { mutableStateOf<String?>(null) }
    var deleteIds by remember { mutableStateOf<Set<String>?>(null) }
    val filtered = state.conversations.filter { it.archivedAt == null }.filter {
        state.conversationSearch.isBlank() || displayTitle(it, stringResource(R.string.new_chat)).contains(state.conversationSearch, true)
    }
    Column(Modifier.fillMaxSize().statusBarsPadding()) {
        Row(Modifier.fillMaxWidth().padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
            AppLogo(34.dp)
            Text(stringResource(R.string.app_name), Modifier.padding(start = 10.dp).weight(1f), style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
            IconButton(onClick = onNewConversation) { Icon(Icons.Outlined.Add, stringResource(R.string.new_chat)) }
        }
        OutlinedTextField(
            value = state.conversationSearch,
            onValueChange = viewModel::setConversationSearch,
            placeholder = { Text(stringResource(R.string.search_conversations)) },
            leadingIcon = { Icon(Icons.Outlined.Search, null) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 4.dp),
        )
        if (selected.isNotEmpty()) Row(Modifier.fillMaxWidth().padding(horizontal = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(stringResource(R.string.selected_conversations, selected.size), Modifier.weight(1f), style = MaterialTheme.typography.labelLarge)
            if (selected.size == 1) IconButton(onClick = { renameId = selected.first() }, modifier = Modifier.testTag(UiTestTags.RENAME_SELECTED)) {
                Icon(Icons.Outlined.Edit, stringResource(R.string.rename))
            }
            IconButton(onClick = { selected = filtered.map(Conversation::id).toSet() }) { Icon(Icons.Outlined.SelectAll, stringResource(R.string.select_all)) }
            IconButton(onClick = { deleteIds = selected }, modifier = Modifier.testTag(UiTestTags.DELETE_SELECTED)) { Icon(Icons.Outlined.DeleteOutline, stringResource(R.string.delete)) }
            IconButton(onClick = { selected = emptySet() }) { Icon(Icons.Outlined.Close, stringResource(R.string.clear_selection)) }
        }
        LazyColumn(Modifier.weight(1f), contentPadding = PaddingValues(vertical = 8.dp)) {
            items(filtered, key = Conversation::id) { conversation ->
                val isSelected = conversation.id in selected
                val generating = state.generations[conversation.id]?.active == true
                Surface(
                    color = when {
                        isSelected -> MaterialTheme.colorScheme.secondaryContainer
                        state.activeConversationId == conversation.id -> MaterialTheme.colorScheme.surfaceVariant
                        else -> MaterialTheme.colorScheme.surface
                    },
                    shape = RoundedCornerShape(6.dp),
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp).fillMaxWidth()
                        .combinedClickable(
                            onClick = {
                                if (selected.isNotEmpty()) selected = if (isSelected) selected - conversation.id else selected + conversation.id
                                else onConversation(conversation.id)
                            },
                            onLongClick = { selected = if (isSelected) selected - conversation.id else selected + conversation.id },
                        ).testTag(UiTestTags.conversationItem(conversation.id)),
                ) {
                    Row(Modifier.padding(horizontal = 10.dp, vertical = 7.dp), verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text(
                                displayTitle(conversation, stringResource(R.string.new_chat)),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                style = MaterialTheme.typography.bodyMedium,
                            )
                            val effectiveModelId = if (conversation.modelMode == SettingMode.INHERIT) {
                                state.globalSettings.defaultModelId
                            } else conversation.model
                            state.models.firstOrNull { it.id == effectiveModelId }?.let {
                                Text(
                                    it.displayName,
                                    style = MaterialTheme.typography.labelSmall,
                                    maxLines = 1,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                        }
                        if (generating) CircularProgressIndicator(Modifier.size(16.dp), strokeWidth = 2.dp)
                        else if (conversation.pinnedAt != null) Icon(Icons.Outlined.PushPin, null, Modifier.size(16.dp), tint = MaterialTheme.colorScheme.primary)
                    }
                }
            }
            val archived = state.conversations.filter { it.archivedAt != null }
            if (archived.isNotEmpty()) {
                item { Text(stringResource(R.string.archived), Modifier.padding(horizontal = 16.dp, vertical = 10.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                items(archived, key = { "archived-${it.id}" }) { conversation ->
                    Row(Modifier.fillMaxWidth().clickable { onConversation(conversation.id) }.padding(horizontal = 16.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(displayTitle(conversation, stringResource(R.string.new_chat)), Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        IconButton(onClick = { viewModel.archiveConversation(conversation.id, false) }, modifier = Modifier.size(32.dp)) { Icon(Icons.Outlined.Refresh, stringResource(R.string.restore), Modifier.size(17.dp)) }
                    }
                }
            }
        }
        HorizontalDivider()
        TextButton(
            onClick = { destinationsExpanded = !destinationsExpanded },
            modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp).testTag(UiTestTags.EXPAND_DESTINATIONS),
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp),
        ) {
            Text(stringResource(R.string.workspace_and_settings), Modifier.weight(1f), style = MaterialTheme.typography.labelLarge)
            Icon(
                if (destinationsExpanded) Icons.Outlined.ExpandLess else Icons.Outlined.ExpandMore,
                stringResource(if (destinationsExpanded) R.string.collapse else R.string.expand),
                Modifier.size(20.dp),
            )
        }
        if (destinationsExpanded) {
            SidebarDestination(Icons.Outlined.Bookmarks, stringResource(R.string.bookmarks)) { viewModel.openScreen(AppScreen.BOOKMARKS) }
            SidebarDestination(Icons.Outlined.NoteAlt, stringResource(R.string.notes)) { viewModel.openScreen(AppScreen.NOTES) }
            SidebarDestination(Icons.Outlined.SmartToy, stringResource(R.string.agents)) { viewModel.openScreen(AppScreen.AGENTS) }
            SidebarDestination(Icons.Outlined.FolderOpen, stringResource(R.string.knowledge)) { viewModel.openScreen(AppScreen.KNOWLEDGE) }
            SidebarDestination(Icons.Outlined.Settings, stringResource(R.string.global_settings)) { viewModel.openScreen(AppScreen.GLOBAL_SETTINGS) }
            SidebarDestination(Icons.Outlined.Hub, stringResource(R.string.providers_models)) { viewModel.openScreen(AppScreen.PROVIDERS) }
            SidebarDestination(Icons.Outlined.Search, stringResource(R.string.exa_search)) { viewModel.openScreen(AppScreen.EXA) }
            SidebarDestination(Icons.Outlined.FileUpload, stringResource(R.string.import_export)) { viewModel.openScreen(AppScreen.TRANSFER) }
            SidebarDestination(Icons.Outlined.Info, stringResource(R.string.about)) { viewModel.openScreen(AppScreen.ABOUT) }
        }
        Spacer(Modifier.navigationBarsPadding())
    }
    renameId?.let { id ->
        val current = state.conversations.firstOrNull { it.id == id }?.title.orEmpty()
        TextInputDialog(stringResource(R.string.rename_conversation), current, { renameId = null }) { title ->
            viewModel.renameConversation(id, title); renameId = null; selected = emptySet()
        }
    }
    deleteIds?.let { ids ->
        AlertDialog(
            onDismissRequest = { deleteIds = null },
            title = { Text(stringResource(if (ids.size == 1) R.string.confirm_delete else R.string.confirm_batch_delete)) },
            text = { Text(pluralStringResource(R.plurals.batch_delete_detail, ids.size, ids.size)) },
            confirmButton = { Button(onClick = { viewModel.deleteConversations(ids); deleteIds = null; selected = emptySet() }) { Text(stringResource(R.string.delete)) } },
            dismissButton = { TextButton(onClick = { deleteIds = null }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

@Composable
private fun SidebarDestination(icon: androidx.compose.ui.graphics.vector.ImageVector, label: String, onClick: () -> Unit) {
    TextButton(onClick = onClick, modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp), contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp)) {
        Icon(icon, null); Spacer(Modifier.width(12.dp)); Text(label, Modifier.weight(1f))
    }
}

@Composable
private fun JadeInspector(state: AppUiState, viewModel: AppViewModel, modifier: Modifier = Modifier) {
    Column(modifier.padding(16.dp).statusBarsPadding(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(stringResource(R.string.settings), style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
        HorizontalDivider()
        Text(stringResource(R.string.model), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        val modelId = if (state.config.modelMode == SettingMode.INHERIT) state.globalSettings.defaultModelId else state.config.model
        Text(state.models.firstOrNull { it.id == modelId }?.displayName ?: stringResource(R.string.conversation_model_missing), maxLines = 2, overflow = TextOverflow.Ellipsis)
        Text(stringResource(R.string.tools), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        ToggleRow(stringResource(R.string.search_web), state.enableSearch, state.exaConfigured) {
            viewModel.setTools(it, state.enableRead, state.showProcess, state.enableKnowledge)
        }
        ToggleRow(stringResource(R.string.read_web), state.enableRead, true) {
            viewModel.setTools(state.enableSearch, it, state.showProcess, state.enableKnowledge)
        }
        ToggleRow(stringResource(R.string.enable_knowledge), state.enableKnowledge, state.knowledgeDocuments.any { it.status == "ready" }) {
            viewModel.setTools(state.enableSearch, state.enableRead, state.showProcess, it)
        }
        HorizontalDivider()
        OutlinedButton(onClick = { viewModel.openScreen(AppScreen.GLOBAL_SETTINGS) }, shape = RoundedCornerShape(8.dp), modifier = Modifier.fillMaxWidth()) {
            Icon(Icons.Outlined.Settings, null)
            Spacer(Modifier.width(8.dp))
            Text(stringResource(R.string.global_settings))
        }
        OutlinedButton(onClick = { viewModel.openScreen(AppScreen.KNOWLEDGE) }, shape = RoundedCornerShape(8.dp), modifier = Modifier.fillMaxWidth()) {
            Icon(Icons.Outlined.FolderOpen, null)
            Spacer(Modifier.width(8.dp))
            Text(stringResource(R.string.knowledge))
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ChatPane(
    state: AppUiState,
    viewModel: AppViewModel,
    showMenu: Boolean,
    onMenu: () -> Unit,
    avatars: LocalAvatarImages,
    userAvatarImage: ImageBitmap?,
    assistantAvatarImage: ImageBitmap?,
    chatFontScale: Float,
    onFontScale: (Float) -> Unit,
    onAvatarSelected: (LocalAvatarKind, Uri) -> Unit,
    onAvatarRemoved: (LocalAvatarKind) -> Unit,
) {
    var settings by rememberSaveable { mutableStateOf(false) }
    var tools by rememberSaveable { mutableStateOf(false) }
    var menu by remember { mutableStateOf(false) }
    var branchMessage by remember { mutableStateOf<ChatMessage?>(null) }
    var playbackState by remember { mutableStateOf(SpeechPlaybackState()) }
    val chatInstanceId = remember(state.activeConversationId) { UUID.randomUUID().toString() }
    val context = LocalContext.current
    val speechPlayer = remember(context, viewModel) {
        SpeechPlaybackController(
            context = context,
            onStateChanged = { playbackState = it },
            onError = viewModel::reportSpeechPlaybackError,
        )
    }
    DisposableEffect(speechPlayer) { onDispose { speechPlayer.release() } }
    LaunchedEffect(viewModel, speechPlayer, state.activeConversationId, chatInstanceId) {
        viewModel.speechAutoPlay.collect { request ->
            if (shouldAutoPlaySpeech(request, state.activeConversationId, chatInstanceId)) {
                speechPlayer.play(request.messageId, request.filePath)
            }
        }
    }
    val activeConversation = state.activeConversation
    val effectiveModelId = if (state.config.modelMode == SettingMode.INHERIT) state.globalSettings.defaultModelId else state.config.model
    val effectiveUserAvatar = if (state.config.userAvatarMode == SettingMode.INHERIT) state.globalSettings.userAvatar else state.config.userAvatar
    val effectiveAssistantAvatar = if (state.config.assistantAvatarMode == SettingMode.INHERIT) state.globalSettings.assistantAvatar else state.config.assistantAvatar
    Column(Modifier.fillMaxSize()) {
        TopAppBar(
            title = {
                Column {
                    Text(state.activeConversation?.let { displayTitle(it, stringResource(R.string.new_chat)) } ?: stringResource(R.string.new_chat), maxLines = 1, overflow = TextOverflow.Ellipsis)
                    val model = state.models.firstOrNull { it.id == effectiveModelId }
                    if (model != null) Text(model.displayName, style = MaterialTheme.typography.labelSmall)
                }
            },
            navigationIcon = { if (showMenu) IconButton(onClick = onMenu, modifier = Modifier.testTag(UiTestTags.OPEN_CONVERSATIONS)) { Icon(Icons.Outlined.Menu, stringResource(R.string.open_conversations)) } },
            actions = {
                IconButton(onClick = { tools = true }) { Icon(Icons.Outlined.Tune, stringResource(R.string.tools)) }
                IconButton(onClick = { settings = true }, modifier = Modifier.testTag(UiTestTags.SETTINGS)) { Icon(Icons.Outlined.Settings, stringResource(R.string.settings)) }
                if (state.activeConversationId != null) Box {
                    IconButton(onClick = { menu = true }) { Icon(Icons.Outlined.MoreVert, stringResource(R.string.more_actions)) }
                    DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                        DropdownMenuItem(text = { Text(stringResource(R.string.generate_title)) }, onClick = { menu = false; viewModel.generateTitle() }, leadingIcon = { Icon(Icons.Outlined.AutoAwesome, null) })
                        DropdownMenuItem(text = { Text(stringResource(R.string.regenerate)) }, onClick = { menu = false; viewModel.regenerateLatest() }, leadingIcon = { Icon(Icons.Outlined.Refresh, null) })
                        DropdownMenuItem(
                            text = { Text(stringResource(if (activeConversation?.pinnedAt == null) R.string.pin else R.string.unpin)) },
                            onClick = { menu = false; activeConversation?.let { viewModel.pinConversation(it.id, it.pinnedAt == null) } },
                            leadingIcon = { Icon(Icons.Outlined.PushPin, null) },
                        )
                        DropdownMenuItem(
                            text = { Text(stringResource(R.string.archive)) },
                            onClick = { menu = false; activeConversation?.let { viewModel.archiveConversation(it.id, true); viewModel.newConversation() } },
                            leadingIcon = { Icon(Icons.Outlined.Archive, null) },
                        )
                    }
                }
            },
        )
        if (activeConversation != null && state.models.none { it.id == effectiveModelId }) {
            Surface(color = MaterialTheme.colorScheme.errorContainer) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(stringResource(R.string.conversation_model_missing), Modifier.weight(1f), color = MaterialTheme.colorScheme.onErrorContainer)
                    TextButton(onClick = { viewModel.openScreen(AppScreen.PROVIDERS) }) { Text(stringResource(R.string.configure_model)) }
                }
            }
        }
        if (state.activeMessages.isEmpty()) {
            Box(Modifier.weight(1f).fillMaxWidth(), contentAlignment = Alignment.Center) {
                Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.padding(24.dp)) {
                    AppLogo(64.dp)
                    Text(stringResource(R.string.empty_title), style = MaterialTheme.typography.titleLarge)
                    Text(stringResource(R.string.empty_detail), style = MaterialTheme.typography.bodyMedium)
                }
            }
        } else MessageList(
            messages = state.activeMessages,
            generation = state.activeGeneration,
            userAvatar = effectiveUserAvatar,
            assistantAvatar = effectiveAssistantAvatar,
            userImage = userAvatarImage,
            assistantImage = assistantAvatarImage,
            fontScale = chatFontScale,
            processExpandedByDefault = state.showProcess,
            onRegenerate = viewModel::regenerateLatest,
            bookmarkedIds = state.bookmarks.map { it.messageId }.toSet(),
            notedMessageIds = state.notes.mapNotNull { it.sourceMessageId }.toSet(),
            scrollToMessageId = state.scrollToMessageId,
            onScrollConsumed = viewModel::consumeScrollTarget,
            onBookmark = viewModel::toggleBookmark,
            onSaveNote = viewModel::saveMessageAsNote,
            attachments = state.activeConversationId?.let { state.attachments[it] }.orEmpty(),
            tts = state.tts,
            playbackState = playbackState,
            onBranch = { message -> branchMessage = message },
            onSpeak = { messageId ->
                if (!state.globalSettings.mimoTtsConfigured) viewModel.openScreen(AppScreen.GLOBAL_SETTINGS)
                else state.activeConversationId?.let { conversationId ->
                    viewModel.synthesizeSpeech(
                        messageId = messageId,
                        autoPlayTarget = SpeechAutoPlayTarget(conversationId, chatInstanceId),
                    )
                }
            },
            onPlay = { messageId, path ->
                speechPlayer.playOrToggle(messageId, path)
            },
            modifier = Modifier.weight(1f),
        )
        Composer(
            state = state,
            onAttachments = viewModel::addAttachments,
            onRemoveAttachment = viewModel::removeAttachment,
            onCameraFailure = viewModel::reportCameraCaptureFailure,
            onSend = viewModel::send,
            onStop = viewModel::stopGeneration,
        )
    }
    if (tools) ToolsDialog(state, { tools = false }) { search, read, knowledge, process -> viewModel.setTools(search, read, process, knowledge); tools = false }
    if (settings) SettingsDialog(
        state = state,
        initial = state.config,
        avatars = avatars,
        chatFontScale = chatFontScale,
        onFontScale = onFontScale,
        onAvatarSelected = onAvatarSelected,
        onAvatarRemoved = onAvatarRemoved,
        onDismiss = { settings = false },
        onSave = { viewModel.saveSettings(it); settings = false },
    )
    branchMessage?.let { message ->
        val branchSuffix = stringResource(R.string.branch)
        val newChatLabel = stringResource(R.string.new_chat)
        val sourceTitle = state.activeConversation?.title?.takeIf(String::isNotBlank) ?: newChatLabel
        val defaultBranchTitle = "$sourceTitle - $branchSuffix"
        var title by rememberSaveable(message.id) {
            mutableStateOf(defaultBranchTitle)
        }
        AlertDialog(
            onDismissRequest = { branchMessage = null },
            title = { Text(stringResource(R.string.create_branch)) },
            text = { OutlinedTextField(title, { title = it }, label = { Text(stringResource(R.string.conversation_title)) }, singleLine = true) },
            confirmButton = { Button(onClick = { viewModel.createBranch(message.id, title); branchMessage = null }, enabled = title.isNotBlank()) { Text(stringResource(R.string.create)) } },
            dismissButton = { TextButton(onClick = { branchMessage = null }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

@Composable
private fun MessageList(
    messages: List<ChatMessage>,
    generation: GenerationState?,
    userAvatar: String,
    assistantAvatar: String,
    userImage: ImageBitmap?,
    assistantImage: ImageBitmap?,
    fontScale: Float,
    processExpandedByDefault: Boolean,
    onRegenerate: () -> Unit,
    bookmarkedIds: Set<String>,
    notedMessageIds: Set<String>,
    scrollToMessageId: String?,
    onScrollConsumed: () -> Unit,
    onBookmark: (String) -> Unit,
    onSaveNote: (ChatMessage) -> Unit,
    attachments: List<MessageAttachment>,
    tts: Map<String, TtsUiState>,
    playbackState: SpeechPlaybackState,
    onBranch: (ChatMessage) -> Unit,
    onSpeak: (String) -> Unit,
    onPlay: (String, String) -> Unit,
    modifier: Modifier,
) {
    val listState = rememberLazyListState()
    val atBottom by remember { derivedStateOf {
        val info = listState.layoutInfo
        info.totalItemsCount == 0 || info.visibleItemsInfo.lastOrNull()?.let { item ->
            item.index == info.totalItemsCount - 1 && item.offset + item.size <= info.viewportEndOffset + 8
        } == true
    } }
    var followStreaming by rememberSaveable { mutableStateOf(true) }
    LaunchedEffect(listState) {
        snapshotFlow { listState.isScrollInProgress to atBottom }
            .collect { (scrolling, bottom) ->
                if (scrolling && !bottom) followStreaming = false
                if (bottom) followStreaming = true
            }
    }
    LaunchedEffect(messages.size, messages.lastOrNull()?.content?.length, followStreaming) {
        if (messages.isNotEmpty() && followStreaming) listState.scrollToItem(messages.lastIndex, Int.MAX_VALUE)
    }
    LaunchedEffect(scrollToMessageId) {
        val index = messages.indexOfFirst { it.id == scrollToMessageId }
        if (index >= 0) listState.animateScrollToItem(index)
        if (scrollToMessageId != null) onScrollConsumed()
    }
    Box(modifier) {
        LazyColumn(
            Modifier.fillMaxSize(),
            state = listState,
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            items(messages, key = ChatMessage::id) { message ->
            val isLastAssistant = message.role == "assistant" && messages.indexOfLast { it.role == "assistant" } == messages.indexOf(message)
            MessageItem(
                message = message,
                generation = if (isLastAssistant) generation else null,
                avatar = if (message.role == "user") userAvatar else assistantAvatar,
                avatarImage = if (message.role == "user") userImage else assistantImage,
                fontScale = fontScale,
                processExpandedByDefault = processExpandedByDefault,
                isLatestAssistant = isLastAssistant,
                onRegenerate = onRegenerate,
                bookmarked = message.id in bookmarkedIds,
                savedAsNote = message.id in notedMessageIds,
                onBookmark = { onBookmark(message.id) },
                onSaveNote = { onSaveNote(message) },
                attachments = attachments.filter { it.messageId == message.id },
                tts = tts[message.id],
                playback = playbackState.takeIf { it.messageId == message.id } ?: SpeechPlaybackState(),
                onBranch = { onBranch(message) },
                onSpeak = { onSpeak(message.id) },
                onPlay = { path -> onPlay(message.id, path) },
            )
        }
        }
        if (!followStreaming && messages.isNotEmpty()) FloatingActionButton(
            onClick = { followStreaming = true },
            modifier = Modifier.align(Alignment.BottomEnd).padding(16.dp).size(44.dp),
        ) { Icon(Icons.Outlined.KeyboardArrowDown, stringResource(R.string.back_to_bottom)) }
    }
}

@Composable
private fun MessageItem(
    message: ChatMessage,
    generation: GenerationState?,
    avatar: String,
    avatarImage: ImageBitmap?,
    fontScale: Float,
    processExpandedByDefault: Boolean,
    isLatestAssistant: Boolean,
    onRegenerate: () -> Unit,
    bookmarked: Boolean,
    savedAsNote: Boolean,
    onBookmark: () -> Unit,
    onSaveNote: () -> Unit,
    attachments: List<MessageAttachment>,
    tts: TtsUiState?,
    playback: SpeechPlaybackState,
    onBranch: () -> Unit,
    onSpeak: () -> Unit,
    onPlay: (String) -> Unit,
) {
    val isUser = message.role == "user"
    val clipboard = LocalClipboardManager.current
    val metadata = message.assistantMetadata(com.tokenflow.chat.data.DirectApiTransport.defaultJson)
    val events = generation?.events?.takeIf { it.isNotEmpty() } ?: metadata.events
    val usage = generation?.usage?.takeIf { it.totalTokens > 0 } ?: metadata.usage.toUsage()
    var expanded by rememberSaveable(message.id) { mutableStateOf(processExpandedByDefault) }
    Row(Modifier.fillMaxWidth(), horizontalArrangement = if (isUser) Arrangement.End else Arrangement.Start, verticalAlignment = Alignment.Top) {
        if (!isUser) {
            Avatar(avatar, avatarImage, Modifier.testTag(UiTestTags.ASSISTANT_MESSAGE_AVATAR)); Spacer(Modifier.width(10.dp))
        }
        Surface(
            color = if (isUser) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface,
            shape = RoundedCornerShape(8.dp),
            modifier = Modifier.widthIn(max = 760.dp).weight(1f, fill = false),
        ) {
            Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                val density = LocalDensity.current
                CompositionLocalProvider(LocalDensity provides Density(density.density, density.fontScale * fontScale)) {
                    if (isUser) SelectionContainer { Text(message.content) }
                    else if (message.content.isNotBlank()) MarkdownContent(message.content)
                    else if (message.status == "generating") Text(stringResource(R.string.calling_model), style = MaterialTheme.typography.bodySmall)
                }
                if (attachments.isNotEmpty()) AttachmentPreview(attachments)
                if (!isUser) {
                    TextButton(onClick = { expanded = !expanded }, modifier = Modifier.testTag(UiTestTags.PROCESS_DETAILS)) {
                        Icon(if (expanded) Icons.Outlined.Close else Icons.Outlined.Tune, null, Modifier.size(18.dp))
                        Spacer(Modifier.width(6.dp)); Text(stringResource(R.string.process))
                    }
                    if (expanded) ProcessDetails(events, message.status)
                }
                val speechReady = tts?.filePath != null
                if (!isUser) Row(verticalAlignment = Alignment.CenterVertically) {
                    if (usage.totalTokens > 0) {
                        val tokenCount = formatTokenCount(usage.totalTokens)
                        val tokenDescription = stringResource(R.string.tokens_used_accessibility, tokenCount)
                        Text(
                            tokenCount,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.weight(1f)
                                .semantics { contentDescription = tokenDescription }
                                .testTag(UiTestTags.TOKEN_USAGE),
                        )
                    }
                    else Spacer(Modifier.weight(1f))
                    IconButton(onClick = onBranch, enabled = message.status == "completed", modifier = Modifier.size(32.dp)) {
                        Icon(Icons.Outlined.CallSplit, stringResource(R.string.create_branch), Modifier.size(17.dp))
                    }
                    IconButton(onClick = { clipboard.setText(AnnotatedString(message.content)) }, modifier = Modifier.size(32.dp).testTag(UiTestTags.COPY_ASSISTANT_MESSAGE)) {
                        Icon(Icons.Outlined.ContentCopy, stringResource(R.string.copy), Modifier.size(17.dp))
                    }
                    if (!speechReady) IconButton(
                        onClick = onSpeak,
                        enabled = tts?.loading != true && message.status == "completed",
                        modifier = Modifier.size(32.dp).testTag(UiTestTags.SPEECH_ACTION),
                    ) {
                        if (tts?.loading == true) CircularProgressIndicator(Modifier.size(16.dp), strokeWidth = 2.dp)
                        else Icon(Icons.Outlined.VolumeUp, stringResource(R.string.generate_speech), Modifier.size(17.dp))
                    }
                    IconButton(onClick = onBookmark, modifier = Modifier.size(32.dp)) {
                        Icon(Icons.Outlined.Bookmarks, stringResource(if (bookmarked) R.string.remove_bookmark else R.string.bookmark), Modifier.size(17.dp), tint = if (bookmarked) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    IconButton(onClick = onSaveNote, enabled = !savedAsNote, modifier = Modifier.size(32.dp)) {
                        Icon(
                            Icons.Outlined.NoteAlt,
                            stringResource(if (savedAsNote) R.string.already_saved_as_note else R.string.save_as_note),
                            Modifier.size(17.dp),
                            tint = if (savedAsNote) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                    if (isLatestAssistant && message.status != "generating") IconButton(onClick = onRegenerate, modifier = Modifier.size(32.dp)) {
                        Icon(Icons.Outlined.Refresh, stringResource(R.string.regenerate), Modifier.size(17.dp))
                    }
                }
                if (!isUser && speechReady) SpeechPlaybackBar(
                    loading = tts.loading,
                    playback = playback,
                    onPlay = { onPlay(tts.filePath) },
                )
                if (!isUser) tts?.error?.let { error ->
                    Text(error.resolve(), color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelSmall)
                }
                if (message.status == "failed" || message.status == "interrupted") {
                    Text(stringResource(if (message.status == "interrupted") R.string.response_interrupted else R.string.assistant_failed), color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelMedium)
                }
            }
        }
        if (isUser) { Spacer(Modifier.width(10.dp)); Avatar(avatar, avatarImage, Modifier.testTag(UiTestTags.USER_MESSAGE_AVATAR)) }
    }
}

@Composable
private fun SpeechPlaybackBar(
    loading: Boolean,
    playback: SpeechPlaybackState,
    onPlay: () -> Unit,
) {
    val playing = playback.phase == SpeechPlaybackPhase.PLAYING
    val preparing = playback.phase == SpeechPlaybackPhase.PREPARING
    Surface(
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.55f),
        shape = RoundedCornerShape(6.dp),
        modifier = Modifier.fillMaxWidth().height(40.dp).testTag(UiTestTags.SPEECH_CONTROLS),
    ) {
        Row(
            Modifier.fillMaxSize().padding(horizontal = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(
                onClick = onPlay,
                enabled = !loading && !preparing,
                modifier = Modifier.size(32.dp),
            ) {
                if (preparing) CircularProgressIndicator(Modifier.size(16.dp), strokeWidth = 2.dp)
                else Icon(
                    if (playing) Icons.Outlined.Pause else Icons.Outlined.PlayArrow,
                    stringResource(if (playing) R.string.pause_speech else R.string.play_speech),
                    Modifier.size(18.dp),
                )
            }
            Text(
                when {
                    loading -> stringResource(R.string.generating_speech)
                    preparing -> stringResource(R.string.preparing_speech)
                    playing -> stringResource(R.string.playing_speech)
                    else -> stringResource(R.string.speech_ready)
                },
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.weight(1f),
            )
            if (playback.durationMs > 0) Text(
                formatAudioDuration(playback.durationMs),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(end = 8.dp),
            )
        }
    }
}

internal fun formatTokenCount(tokens: Long): String {
    val safe = tokens.coerceAtLeast(0)
    if (safe < 1_000) return safe.toString()
    val tenths = safe / 100 + if (safe % 100 >= 50) 1 else 0
    return if (tenths % 10L == 0L) "${tenths / 10}K" else "${tenths / 10}.${tenths % 10}K"
}

private fun formatAudioDuration(durationMs: Long): String {
    val totalSeconds = (durationMs.coerceAtLeast(0) + 500) / 1_000
    return "${totalSeconds / 60}:${(totalSeconds % 60).toString().padStart(2, '0')}"
}

@Composable
private fun Avatar(value: String, image: ImageBitmap?, modifier: Modifier = Modifier, size: Dp = 36.dp) {
    Surface(shape = CircleShape, color = MaterialTheme.colorScheme.secondaryContainer, modifier = modifier.size(size).aspectRatio(1f).clip(CircleShape)) {
        if (image != null) Image(image, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
        else Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { Text(value.take(2), style = MaterialTheme.typography.labelMedium) }
    }
}

@Composable
internal fun AppLogo(size: Dp) {
    Image(painterResource(R.drawable.tokenflow_logo), null, Modifier.size(size), contentScale = ContentScale.Fit)
}

@Composable
private fun ProcessDetails(events: List<ProcessEvent>, messageStatus: String) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (events.isEmpty()) {
            Text(
                stringResource(if (messageStatus == "generating") R.string.calling_model else R.string.no_process_details),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        events.forEach { event ->
            Surface(shape = RoundedCornerShape(6.dp), color = MaterialTheme.colorScheme.surface) {
                Column(Modifier.fillMaxWidth().padding(10.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(
                        when (event.type) {
                            "thinking" -> stringResource(R.string.thinking)
                            "tool_started" -> if (event.name == "read_url") stringResource(R.string.read_url_validating) else "${event.name} · ${stringResource(R.string.tool_started)}"
                            "tool_completed" -> if (event.name == "read_url") stringResource(R.string.read_url_success) else "${event.name} · ${stringResource(R.string.tool_completed)}"
                            "tool_failed" -> if (event.name == "read_url") stringResource(R.string.read_url_failed) else "${event.name} · ${stringResource(R.string.tool_failed)}"
                            "infoflow_fallback" -> stringResource(R.string.infoflow_fallback)
                            "infoflow_rendered" -> stringResource(R.string.infoflow_rendered)
                            "infoflow_success" -> stringResource(R.string.infoflow_success)
                            else -> if (event.messageKey == "retrying") stringResource(R.string.retrying, event.attempt, event.maxAttempts) else event.message
                        },
                        style = MaterialTheme.typography.labelMedium,
                        fontWeight = FontWeight.SemiBold,
                    )
                    val body = event.content.ifBlank { event.result.ifBlank { event.arguments } }
                    if (body.isNotBlank()) SelectionContainer { Text(body, style = MaterialTheme.typography.bodySmall) }
                }
            }
        }
    }
}

@Composable
private fun Composer(
    state: AppUiState,
    onAttachments: (List<PendingAttachment>) -> Unit,
    onRemoveAttachment: (String) -> Unit,
    onCameraFailure: () -> Unit,
    onSend: (String) -> Unit,
    onStop: () -> Unit,
) {
    var value by rememberSaveable(state.activeConversationId) { mutableStateOf("") }
    var attachmentMenu by remember { mutableStateOf(false) }
    var cameraCapturePath by rememberSaveable { mutableStateOf<String?>(null) }
    var cameraProcessing by remember { mutableStateOf(false) }
    val focus = LocalFocusManager.current
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val cameraStore = remember(context.applicationContext) { CameraCaptureStore(context.applicationContext) }
    val generating = state.activeGeneration?.active == true
    val imagePicker = rememberLauncherForActivityResult(ActivityResultContracts.PickMultipleVisualMedia(5)) { uris ->
        onAttachments(uris.mapNotNull { pendingAttachment(context, it) })
    }
    val filePicker = rememberLauncherForActivityResult(ActivityResultContracts.OpenMultipleDocuments()) { uris ->
        onAttachments(uris.mapNotNull { pendingAttachment(context, it) })
    }
    val camera = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { captured ->
        val path = cameraCapturePath
        cameraCapturePath = null
        if (!captured || path == null) {
            cameraStore.cancelCapture(path)
        } else {
            cameraProcessing = true
            scope.launch {
                runCatching { cameraStore.finishCapture(path) }
                    .onSuccess { attachment ->
                        if (state.pendingAttachments.size < 5) onAttachments(listOf(attachment))
                        else cameraStore.cancelCapture(attachment.appOwnedDraftPath)
                    }
                    .onFailure { onCameraFailure() }
                cameraProcessing = false
            }
        }
    }
    Box(
        Modifier.fillMaxWidth()
            .background(MaterialTheme.colorScheme.background)
            .navigationBarsPadding()
            .imePadding(),
    ) {
        Surface(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 10.dp),
            shape = RoundedCornerShape(8.dp),
            color = MaterialTheme.colorScheme.surfaceContainerLow,
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
            tonalElevation = 1.dp,
            shadowElevation = 1.dp,
        ) {
            Column(Modifier.fillMaxWidth().padding(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                if (state.pendingAttachments.isNotEmpty()) Row(
                    Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    state.pendingAttachments.forEach { attachment ->
                        Surface(shape = RoundedCornerShape(6.dp), color = MaterialTheme.colorScheme.surfaceVariant) {
                            Row(Modifier.padding(start = 10.dp, top = 4.dp, bottom = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                                Icon(Icons.Outlined.AttachFile, null, Modifier.size(16.dp))
                                Text(attachment.displayName, Modifier.padding(start = 6.dp).widthIn(max = 160.dp), maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.labelMedium)
                                IconButton(onClick = { onRemoveAttachment(attachment.uri) }, modifier = Modifier.size(30.dp)) {
                                    Icon(Icons.Outlined.Close, stringResource(R.string.remove), Modifier.size(15.dp))
                                }
                            }
                        }
                    }
                }
                Row(
                    Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.Bottom,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    FilledTonalIconButton(onClick = { attachmentMenu = true }, enabled = !generating && !cameraProcessing && state.pendingAttachments.size < 5, modifier = Modifier.size(44.dp)) {
                        if (cameraProcessing) CircularProgressIndicator(Modifier.size(19.dp), strokeWidth = 2.dp)
                        else Icon(Icons.Outlined.AttachFile, stringResource(R.string.add_attachment), Modifier.size(21.dp))
                    }
                    BasicTextField(
                        value = value,
                        onValueChange = { value = it },
                        enabled = !generating,
                        textStyle = MaterialTheme.typography.bodyLarge.copy(color = MaterialTheme.colorScheme.onSurface),
                        cursorBrush = SolidColor(MaterialTheme.colorScheme.primary),
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Default),
                        minLines = 1,
                        maxLines = 6,
                        modifier = Modifier.weight(1f).heightIn(min = 44.dp, max = 144.dp).testTag(UiTestTags.MESSAGE_INPUT),
                        decorationBox = { input ->
                            Box(Modifier.fillMaxWidth().defaultMinSize(minHeight = 44.dp).padding(horizontal = 4.dp), contentAlignment = Alignment.CenterStart) {
                                if (value.isEmpty()) Text(
                                    stringResource(R.string.message_hint),
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    style = MaterialTheme.typography.bodyLarge,
                                )
                                input()
                            }
                        },
                    )
                    FilledIconButton(
                        onClick = {
                            if (generating) onStop() else if (!cameraProcessing && (value.isNotBlank() || state.pendingAttachments.isNotEmpty())) {
                                val sent = value; value = ""; focus.clearFocus(); onSend(sent)
                            }
                        },
                        enabled = generating || (!cameraProcessing && (value.isNotBlank() || state.pendingAttachments.isNotEmpty())),
                        modifier = Modifier.size(44.dp).testTag(UiTestTags.MESSAGE_ACTION),
                    ) {
                        Icon(
                            if (generating) Icons.Outlined.Stop else Icons.AutoMirrored.Outlined.Send,
                            stringResource(if (generating) R.string.stop else R.string.send),
                            Modifier.size(21.dp),
                        )
                    }
                }
            }
        }
    }
    if (attachmentMenu) AlertDialog(
        onDismissRequest = { attachmentMenu = false },
        title = { Text(stringResource(R.string.add_attachment)) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = {
                    attachmentMenu = false
                    var path: String? = null
                    runCatching {
                        val target = cameraStore.createCapture()
                        path = target.path
                        cameraCapturePath = target.path
                        camera.launch(target.uri)
                    }.onFailure {
                        cameraStore.cancelCapture(path)
                        cameraCapturePath = null
                        onCameraFailure()
                    }
                }, modifier = Modifier.fillMaxWidth().testTag(UiTestTags.TAKE_PHOTO)) {
                    Icon(Icons.Outlined.PhotoCamera, null); Spacer(Modifier.width(8.dp)); Text(stringResource(R.string.take_photo))
                }
                OutlinedButton(onClick = {
                    attachmentMenu = false
                    imagePicker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly))
                }, modifier = Modifier.fillMaxWidth()) {
                    Icon(Icons.Outlined.AddAPhoto, null); Spacer(Modifier.width(8.dp)); Text(stringResource(R.string.choose_images))
                }
                OutlinedButton(onClick = {
                    attachmentMenu = false
                    filePicker.launch(arrayOf("text/*", "application/json", "application/xml", "application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
                }, modifier = Modifier.fillMaxWidth()) {
                    Icon(Icons.Outlined.AttachFile, null); Spacer(Modifier.width(8.dp)); Text(stringResource(R.string.choose_files))
                }
            }
        },
        confirmButton = {},
        dismissButton = { TextButton(onClick = { attachmentMenu = false }) { Text(stringResource(R.string.cancel)) } },
    )
}

@Composable
private fun AttachmentPreview(attachments: List<MessageAttachment>) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        attachments.forEach { attachment ->
            if (attachment.kind == com.tokenflow.chat.data.AttachmentKind.IMAGE) {
                val bitmap by produceState<ImageBitmap?>(null, attachment.storedPath) {
                    value = withContext(Dispatchers.IO) { BitmapFactory.decodeFile(attachment.storedPath)?.asImageBitmap() }
                }
                bitmap?.let { image ->
                    Image(
                        image,
                        attachment.fileName,
                        Modifier.fillMaxWidth().heightIn(max = 240.dp).clip(RoundedCornerShape(6.dp)),
                        contentScale = ContentScale.Fit,
                    )
                }
            } else Surface(shape = RoundedCornerShape(6.dp), color = MaterialTheme.colorScheme.surfaceVariant) {
                Row(Modifier.fillMaxWidth().padding(10.dp), verticalAlignment = Alignment.CenterVertically) {
                    Icon(Icons.Outlined.AttachFile, null, Modifier.size(18.dp))
                    Column(Modifier.padding(start = 8.dp).weight(1f)) {
                        Text(attachment.fileName, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.labelMedium)
                        Text("${attachment.mimeType} · ${formatBytes(attachment.sizeBytes)}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            }
        }
    }
}

private fun pendingAttachment(context: android.content.Context, uri: Uri): PendingAttachment? = runCatching {
    var name = uri.lastPathSegment?.substringAfterLast('/').orEmpty().ifBlank { "attachment" }
    var size = -1L
    context.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE), null, null, null)?.use { cursor ->
        if (cursor.moveToFirst()) {
            cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME).takeIf { it >= 0 }?.let { name = cursor.getString(it) ?: name }
            cursor.getColumnIndex(OpenableColumns.SIZE).takeIf { it >= 0 }?.let { if (!cursor.isNull(it)) size = cursor.getLong(it) }
                        }
    }
    PendingAttachment(uri.toString(), name, context.contentResolver.getType(uri).orEmpty().ifBlank { "application/octet-stream" }, size)
}.getOrNull()

private fun formatBytes(value: Long): String = when {
    value < 0 -> "—"
    value < 1024 -> "$value B"
    value < 1024 * 1024 -> "${value / 1024} KiB"
    else -> "%.1f MiB".format(value / (1024.0 * 1024.0))
}

@Composable
private fun ToolsDialog(state: AppUiState, onDismiss: () -> Unit, onSave: (Boolean, Boolean, Boolean, Boolean) -> Unit) {
    var search by remember { mutableStateOf(state.enableSearch) }
    var read by remember { mutableStateOf(state.enableRead) }
    var knowledge by remember { mutableStateOf(state.enableKnowledge) }
    var process by remember { mutableStateOf(state.showProcess) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.tools)) },
        text = {
            Column {
                ToggleRow(stringResource(R.string.search_web), search, state.exaConfigured) { search = it }
                if (!state.exaConfigured) Text(stringResource(R.string.exa_required), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
                ToggleRow(stringResource(R.string.read_web), read, true) { read = it }
                ToggleRow(stringResource(R.string.enable_knowledge), knowledge, state.knowledgeDocuments.any { it.status == "ready" }) { knowledge = it }
                ToggleRow(stringResource(R.string.expand_process_default), process, true) { process = it }
            }
        },
        confirmButton = { Button(onClick = { onSave(search, read, knowledge, process) }) { Text(stringResource(R.string.save)) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}

@Composable
private fun ToggleRow(label: String, checked: Boolean, enabled: Boolean, onChecked: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth().clickable(enabled) { onChecked(!checked) }.padding(vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(label, Modifier.weight(1f)); Switch(checked, onCheckedChange = onChecked, enabled = enabled)
    }
}

@Composable
private fun SettingsDialog(
    state: AppUiState,
    initial: ConversationConfig,
    avatars: LocalAvatarImages,
    chatFontScale: Float,
    onFontScale: (Float) -> Unit,
    onAvatarSelected: (LocalAvatarKind, Uri) -> Unit,
    onAvatarRemoved: (LocalAvatarKind) -> Unit,
    onDismiss: () -> Unit,
    onSave: (ConversationConfig) -> Unit,
) {
    var draft by remember(initial) { mutableStateOf(initial) }
    var modelMenu by remember { mutableStateOf(false) }
    var templateMenu by remember { mutableStateOf(false) }
    var pendingTemplate by remember { mutableStateOf<PromptTemplate?>(null) }
    val effectiveModelId = if (draft.modelMode == SettingMode.INHERIT) state.globalSettings.defaultModelId else draft.model
    val effectivePrompt = if (draft.systemPromptMode == SettingMode.INHERIT) state.globalSettings.systemPrompt else draft.systemPrompt
    val selectedModel = state.models.firstOrNull { it.id == effectiveModelId }
    Dialog(onDismissRequest = onDismiss) {
        Surface(shape = RoundedCornerShape(8.dp), modifier = Modifier.fillMaxWidth().widthIn(max = 680.dp).heightIn(max = 760.dp)) {
            Column(Modifier.padding(18.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(stringResource(R.string.settings), style = MaterialTheme.typography.titleLarge, modifier = Modifier.weight(1f))
                    IconButton(onClick = onDismiss) { Icon(Icons.Outlined.Close, stringResource(R.string.close)) }
                }
                Column(Modifier.weight(1f).verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(14.dp)) {
                    Text(stringResource(R.string.model), style = MaterialTheme.typography.labelLarge)
                    ToggleRow(stringResource(R.string.follow_global), draft.modelMode == SettingMode.INHERIT, true) { inherit ->
                        draft = if (inherit) draft.copy(modelMode = SettingMode.INHERIT)
                        else draft.copy(modelMode = SettingMode.OVERRIDE, model = state.globalSettings.defaultModelId.orEmpty())
                    }
                    Box {
                        OutlinedButton(onClick = { modelMenu = true }, enabled = draft.modelMode == SettingMode.OVERRIDE, modifier = Modifier.fillMaxWidth()) { Text(selectedModel?.displayName ?: stringResource(R.string.select_model), Modifier.weight(1f)) }
                        DropdownMenu(expanded = modelMenu, onDismissRequest = { modelMenu = false }) {
                            state.models.forEach { model -> DropdownMenuItem(text = { Text(model.displayName) }, onClick = { draft = draft.copy(model = model.id, modelMode = SettingMode.OVERRIDE); modelMenu = false }) }
                        }
                    }
                    Text(stringResource(R.string.thinking_effort), style = MaterialTheme.typography.labelLarge)
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        listOf("off", "low", "medium", "high").forEach { effort ->
                            FilterChip(selected = draft.thinkingEffort == effort, onClick = { draft = draft.copy(thinkingEffort = effort) }, label = { Text(thinkingEffortLabel(effort)) })
                        }
                    }
                    Text(stringResource(R.string.prompt_template), style = MaterialTheme.typography.labelLarge)
                    ToggleRow(stringResource(R.string.follow_global), draft.systemPromptMode == SettingMode.INHERIT, true) { inherit ->
                        draft = if (inherit) draft.copy(systemPromptMode = SettingMode.INHERIT)
                        else draft.copy(systemPromptMode = SettingMode.OVERRIDE, systemPrompt = state.globalSettings.systemPrompt)
                    }
                    Box {
                        OutlinedButton(onClick = { templateMenu = true }, modifier = Modifier.fillMaxWidth()) { Text(stringResource(R.string.choose_template), Modifier.weight(1f)) }
                        DropdownMenu(expanded = templateMenu, onDismissRequest = { templateMenu = false }) {
                            SystemPrompts.templates.forEach { template -> DropdownMenuItem(
                                text = { Text(if (Locale.current.language == "zh") template.titleZh else template.titleEn) },
                                onClick = {
                                    templateMenu = false
                                    if (effectivePrompt.isNotBlank() && effectivePrompt != template.content) pendingTemplate = template
                                    else draft = draft.copy(systemPrompt = template.content, systemPromptMode = SettingMode.OVERRIDE)
                                },
                            ) }
                        }
                    }
                    OutlinedTextField(
                        value = effectivePrompt,
                        onValueChange = { draft = draft.copy(systemPrompt = it, systemPromptMode = SettingMode.OVERRIDE) },
                        enabled = draft.systemPromptMode == SettingMode.OVERRIDE,
                        label = { Text(stringResource(R.string.system_prompt)) },
                        minLines = 4,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    OutlinedTextField(value = draft.nickname, onValueChange = { draft = draft.copy(nickname = it) }, label = { Text(stringResource(R.string.nickname)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                    Text(stringResource(R.string.user_avatar), style = MaterialTheme.typography.labelLarge)
                    ToggleRow(stringResource(R.string.follow_global), draft.userAvatarMode == SettingMode.INHERIT, true) { inherit ->
                        draft = draft.copy(userAvatarMode = if (inherit) SettingMode.INHERIT else SettingMode.OVERRIDE)
                        if (inherit) onAvatarRemoved(LocalAvatarKind.USER)
                    }
                    LocalAvatarSetting(
                        LocalAvatarKind.USER,
                        stringResource(R.string.user_avatar),
                        avatars.user,
                        { kind, uri -> draft = draft.copy(userAvatarMode = SettingMode.OVERRIDE); onAvatarSelected(kind, uri) },
                        { kind -> draft = draft.copy(userAvatarMode = SettingMode.INHERIT); onAvatarRemoved(kind) },
                        UiTestTags.USER_AVATAR_PICKER,
                    )
                    Text(stringResource(R.string.assistant_avatar), style = MaterialTheme.typography.labelLarge)
                    ToggleRow(stringResource(R.string.follow_global), draft.assistantAvatarMode == SettingMode.INHERIT, true) { inherit ->
                        draft = draft.copy(assistantAvatarMode = if (inherit) SettingMode.INHERIT else SettingMode.OVERRIDE)
                        if (inherit) onAvatarRemoved(LocalAvatarKind.ASSISTANT)
                    }
                    LocalAvatarSetting(
                        LocalAvatarKind.ASSISTANT,
                        stringResource(R.string.assistant_avatar),
                        avatars.assistant,
                        { kind, uri -> draft = draft.copy(assistantAvatarMode = SettingMode.OVERRIDE); onAvatarSelected(kind, uri) },
                        { kind -> draft = draft.copy(assistantAvatarMode = SettingMode.INHERIT); onAvatarRemoved(kind) },
                        UiTestTags.ASSISTANT_AVATAR_PICKER,
                    )
                    Text(stringResource(R.string.url_reader), style = MaterialTheme.typography.labelLarge)
                    ToggleRow(stringResource(R.string.follow_global), draft.urlReaderMode == SettingMode.INHERIT, true) { inherit ->
                        draft = if (inherit) draft.copy(urlReaderMode = SettingMode.INHERIT)
                        else draft.copy(urlReaderMode = SettingMode.OVERRIDE, urlReaderBackend = state.globalSettings.urlReaderBackend)
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        FilterChip(
                            selected = (if (draft.urlReaderMode == SettingMode.INHERIT) state.globalSettings.urlReaderBackend else draft.urlReaderBackend) == UrlReaderBackend.BUILT_IN,
                            onClick = { draft = draft.copy(urlReaderMode = SettingMode.OVERRIDE, urlReaderBackend = UrlReaderBackend.BUILT_IN) },
                            enabled = draft.urlReaderMode == SettingMode.OVERRIDE,
                            label = { Text(stringResource(R.string.url_reader_builtin)) },
                        )
                        FilterChip(
                            selected = (if (draft.urlReaderMode == SettingMode.INHERIT) state.globalSettings.urlReaderBackend else draft.urlReaderBackend) == UrlReaderBackend.INFOFLOW,
                            onClick = { draft = draft.copy(urlReaderMode = SettingMode.OVERRIDE, urlReaderBackend = UrlReaderBackend.INFOFLOW) },
                            enabled = draft.urlReaderMode == SettingMode.OVERRIDE,
                            label = { Text("InfoFlow") },
                        )
                    }
                    Text("${stringResource(R.string.max_tool_calls)}: ${draft.maxToolCalls}")
                    Slider(value = draft.maxToolCalls.toFloat(), onValueChange = { draft = draft.copy(maxToolCalls = it.roundToInt()) }, valueRange = 0f..20f, steps = 19)
                    Text("${stringResource(R.string.chat_font_size)}: ${(chatFontScale * 100).roundToInt()}%")
                    Slider(
                        value = chatFontScale,
                        onValueChange = onFontScale,
                        valueRange = ChatDisplayPreferences.MIN_FONT_SCALE..ChatDisplayPreferences.MAX_FONT_SCALE,
                        steps = 5,
                        modifier = Modifier.testTag(UiTestTags.CHAT_FONT_SIZE),
                    )
                }
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                    TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) }
                    Button(onClick = { onSave(draft) }) { Text(stringResource(R.string.save)) }
                }
            }
        }
    }
    pendingTemplate?.let { template ->
        AlertDialog(
            onDismissRequest = { pendingTemplate = null },
            title = { Text(stringResource(R.string.replace_prompt_title)) },
            text = { Text(stringResource(R.string.replace_prompt_detail)) },
            confirmButton = { Button(onClick = { draft = draft.copy(systemPrompt = template.content, systemPromptMode = SettingMode.OVERRIDE); pendingTemplate = null }) { Text(stringResource(R.string.replace)) } },
            dismissButton = { TextButton(onClick = { pendingTemplate = null }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

@Composable
private fun LocalAvatarSetting(
    kind: LocalAvatarKind,
    label: String,
    file: LocalAvatarFile?,
    onSelected: (LocalAvatarKind, Uri) -> Unit,
    onRemoved: (LocalAvatarKind) -> Unit,
    testTag: String,
) {
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri -> uri?.let { onSelected(kind, it) } }
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Icon(Icons.Outlined.AddAPhoto, null)
        Text(label, Modifier.padding(horizontal = 10.dp).weight(1f))
        TextButton(onClick = { picker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) }, modifier = Modifier.testTag(testTag)) {
            Text(stringResource(if (file == null) R.string.select_image else R.string.change_image))
        }
        if (file != null) IconButton(onClick = { onRemoved(kind) }) { Icon(Icons.Outlined.DeleteOutline, stringResource(R.string.remove_image)) }
    }
}

@Composable
private fun PasswordDialog(title: String, onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    var password by rememberSaveable { mutableStateOf("") }
    var visible by rememberSaveable { mutableStateOf(false) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = { OutlinedTextField(
            value = password,
            onValueChange = { password = it },
            label = { Text(stringResource(R.string.archive_password)) },
            supportingText = { Text(stringResource(R.string.archive_password_hint)) },
            visualTransformation = if (visible) VisualTransformation.None else PasswordVisualTransformation(),
            trailingIcon = { IconButton(onClick = { visible = !visible }) { Icon(if (visible) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility, null) } },
            singleLine = true,
        ) },
        confirmButton = { Button(onClick = { onConfirm(password) }, enabled = password.length >= 10) { Text(stringResource(R.string.confirm)) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}

@Composable
private fun TextInputDialog(title: String, initial: String, onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    var value by rememberSaveable { mutableStateOf(initial) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = { OutlinedTextField(value, { value = it }, singleLine = true, modifier = Modifier.fillMaxWidth()) },
        confirmButton = { Button(onClick = { onConfirm(value) }, enabled = value.isNotBlank()) { Text(stringResource(R.string.save)) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}

@Composable
private fun UiText.resolve(): String = when (this) {
    is UiText.Dynamic -> value
    is UiText.Resource -> stringResource(id, *args.toTypedArray())
}

@Composable
private fun thinkingEffortLabel(value: String): String = when (value) {
    "off" -> stringResource(R.string.thinking_off)
    "low" -> stringResource(R.string.thinking_low)
    "high" -> stringResource(R.string.thinking_high)
    else -> stringResource(R.string.thinking_medium)
}

@Composable
private fun protocolLabel(protocol: ProviderProtocol): String = when (protocol) {
    ProviderProtocol.OPENAI_CHAT_COMPLETIONS -> stringResource(R.string.protocol_openai_chat)
    ProviderProtocol.OPENAI_RESPONSES -> stringResource(R.string.protocol_openai_responses)
    ProviderProtocol.ANTHROPIC_MESSAGES -> stringResource(R.string.protocol_anthropic)
}

private fun protocolShortLabel(protocol: ProviderProtocol): String = when (protocol) {
    ProviderProtocol.OPENAI_CHAT_COMPLETIONS -> "Chat"
    ProviderProtocol.OPENAI_RESPONSES -> "Responses"
    ProviderProtocol.ANTHROPIC_MESSAGES -> "Anthropic"
}

@Composable
private fun visionStatusLabel(status: VisionStatus): String = stringResource(when (status) {
    VisionStatus.UNKNOWN -> R.string.vision_unknown
    VisionStatus.SUPPORTED -> R.string.vision_supported
    VisionStatus.UNSUPPORTED -> R.string.vision_unsupported
})

private fun displayTitle(conversation: Conversation, fallback: String): String = conversation.title.ifBlank { fallback }
