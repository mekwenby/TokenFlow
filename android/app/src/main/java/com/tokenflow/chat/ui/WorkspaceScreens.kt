package com.tokenflow.chat.ui

import android.provider.OpenableColumns
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.AutoAwesome
import androidx.compose.material.icons.outlined.Bookmarks
import androidx.compose.material.icons.outlined.ChatBubbleOutline
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.DeleteOutline
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.FolderOpen
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.Hub
import androidx.compose.material.icons.outlined.ImportExport
import androidx.compose.material.icons.outlined.NoteAlt
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.SelectAll
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.SmartToy
import androidx.compose.material.icons.outlined.UploadFile
import androidx.compose.material3.Button
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Checkbox
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.tokenflow.chat.BuildConfig
import com.tokenflow.chat.R
import com.tokenflow.chat.data.AgentProfile
import com.tokenflow.chat.data.KnowledgeImportSource
import com.tokenflow.chat.data.Note
import java.util.UUID
import kotlin.math.roundToInt

@Composable
internal fun WorkspaceScreen(state: AppUiState, viewModel: AppViewModel) {
    BoxWithConstraints(Modifier.fillMaxSize()) {
        val persistent = maxWidth >= 600.dp
        val railWidth = if (maxWidth >= 1000.dp) 280.dp else 264.dp
        Row(Modifier.fillMaxSize()) {
            if (persistent) {
                WorkspaceRail(state.screen, viewModel, Modifier.width(railWidth))
                HorizontalDivider(Modifier.fillMaxHeight().width(1.dp))
            }
            Box(Modifier.weight(1f)) {
                when (state.screen) {
                    AppScreen.BOOKMARKS -> BookmarksScreen(state, viewModel, !persistent)
                    AppScreen.NOTES -> NotesScreen(state, viewModel, !persistent)
                    AppScreen.AGENTS -> AgentsScreen(state, viewModel, !persistent)
                    AppScreen.KNOWLEDGE -> KnowledgeScreen(state, viewModel, !persistent)
                    AppScreen.ABOUT -> AboutScreen(viewModel, !persistent)
                    else -> Unit
                }
            }
        }
    }
}

@Composable
internal fun AdaptiveConfigurationShell(
    state: AppUiState,
    viewModel: AppViewModel,
    content: @Composable () -> Unit,
) {
    BoxWithConstraints(Modifier.fillMaxSize()) {
        val railWidth = if (maxWidth >= 1000.dp) 280.dp else 264.dp
        if (maxWidth < 600.dp) content()
        else Row(Modifier.fillMaxSize()) {
            WorkspaceRail(state.screen, viewModel, Modifier.width(railWidth))
            HorizontalDivider(Modifier.fillMaxHeight().width(1.dp))
            Box(Modifier.weight(1f)) { content() }
        }
    }
}

@Composable
private fun WorkspaceRail(selected: AppScreen, viewModel: AppViewModel, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxHeight().statusBarsPadding().padding(12.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(4.dp, 8.dp, 4.dp, 20.dp)) {
            AppLogo(34.dp)
            Text(stringResource(R.string.app_name), Modifier.padding(start = 10.dp), style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
        }
        val destinations = listOf(
            Triple(AppScreen.CHAT, Icons.Outlined.ChatBubbleOutline, R.string.conversations),
            Triple(AppScreen.BOOKMARKS, Icons.Outlined.Bookmarks, R.string.bookmarks),
            Triple(AppScreen.NOTES, Icons.Outlined.NoteAlt, R.string.notes),
            Triple(AppScreen.AGENTS, Icons.Outlined.SmartToy, R.string.agents),
            Triple(AppScreen.KNOWLEDGE, Icons.Outlined.FolderOpen, R.string.knowledge),
            Triple(AppScreen.GLOBAL_SETTINGS, Icons.Outlined.Settings, R.string.global_settings),
            Triple(AppScreen.PROVIDERS, Icons.Outlined.Hub, R.string.providers_models),
            Triple(AppScreen.EXA, Icons.Outlined.Search, R.string.exa_search),
            Triple(AppScreen.TRANSFER, Icons.Outlined.ImportExport, R.string.import_export),
            Triple(AppScreen.ABOUT, Icons.Outlined.Info, R.string.about),
        )
        destinations.forEach { (screen, icon, label) ->
            Surface(
                color = if (selected == screen) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface,
                shape = RoundedCornerShape(6.dp),
                modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp).clickable { viewModel.openScreen(screen) },
            ) {
                Row(Modifier.padding(horizontal = 12.dp, vertical = 11.dp), verticalAlignment = Alignment.CenterVertically) {
                    Icon(icon, null, Modifier.size(20.dp))
                    Text(stringResource(label), Modifier.padding(start = 12.dp), style = MaterialTheme.typography.labelLarge)
                }
            }
        }
        Spacer(Modifier.weight(1f))
        Text(stringResource(R.string.all_content_local), color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.labelMedium)
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun WorkspaceScaffold(title: String, showBack: Boolean, onBack: () -> Unit, action: (@Composable () -> Unit)? = null, content: @Composable (PaddingValues) -> Unit) {
    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text(title, fontWeight = FontWeight.SemiBold) },
                navigationIcon = { if (showBack) IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Outlined.ArrowBack, stringResource(R.string.back)) } },
                actions = { action?.invoke() },
            )
        },
        content = content,
    )
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun BookmarksScreen(state: AppUiState, viewModel: AppViewModel, showBack: Boolean) {
    var search by rememberSaveable { mutableStateOf("") }
    var selected by remember { mutableStateOf(emptySet<String>()) }
    var confirmDelete by remember { mutableStateOf(false) }
    val items = state.bookmarks.filter { search.isBlank() || it.content.contains(search, true) || it.conversationTitle.contains(search, true) }
    WorkspaceScaffold(stringResource(R.string.bookmarks), showBack, { viewModel.openScreen(AppScreen.CHAT) }) { padding ->
        Column(Modifier.padding(padding).fillMaxSize().padding(horizontal = 16.dp)) {
            OutlinedTextField(search, { search = it }, placeholder = { Text(stringResource(R.string.search_saved)) }, leadingIcon = { Icon(Icons.Outlined.Search, null) }, singleLine = true, modifier = Modifier.fillMaxWidth().widthIn(max = 760.dp).align(Alignment.CenterHorizontally))
            if (selected.isNotEmpty()) SelectionToolbar(
                selectedCount = selected.size,
                allVisibleSelected = items.isNotEmpty() && items.all { it.messageId in selected },
                onSelectAll = {
                    val visible = items.map { it.messageId }.toSet()
                    selected = if (visible.isNotEmpty() && visible.all { it in selected }) selected - visible else selected + visible
                },
                onDelete = { confirmDelete = true },
                onClear = { selected = emptySet() },
                modifier = Modifier.fillMaxWidth().widthIn(max = 820.dp).align(Alignment.CenterHorizontally),
            )
            if (items.isEmpty()) EmptyWorkspace(Icons.Outlined.Bookmarks, stringResource(R.string.empty_bookmarks), Modifier.weight(1f))
            else LazyColumn(Modifier.weight(1f).fillMaxWidth().widthIn(max = 820.dp).align(Alignment.CenterHorizontally), contentPadding = PaddingValues(vertical = 12.dp)) {
                items(items, key = { it.id }) { item ->
                    val isSelected = item.messageId in selected
                    Column(
                        Modifier.fillMaxWidth()
                            .combinedClickable(
                                onClick = {
                                    if (selected.isEmpty()) viewModel.openBookmark(item)
                                    else selected = if (isSelected) selected - item.messageId else selected + item.messageId
                                },
                                onLongClick = { selected = if (isSelected) selected - item.messageId else selected + item.messageId },
                            )
                            .padding(vertical = 12.dp)
                            .testTag(UiTestTags.bookmarkItem(item.messageId)),
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(item.conversationTitle.ifBlank { stringResource(R.string.new_chat) }, Modifier.weight(1f), color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.labelLarge)
                            if (selected.isNotEmpty()) Checkbox(isSelected, { checked -> selected = if (checked) selected + item.messageId else selected - item.messageId })
                        }
                        SelectionContainer { Text(item.content, Modifier.padding(top = 8.dp), maxLines = 8, overflow = TextOverflow.Ellipsis) }
                        if (selected.isEmpty()) Row {
                            TextButton(onClick = { viewModel.openBookmark(item) }) { Text(stringResource(R.string.open_original)) }
                            TextButton(onClick = { viewModel.toggleBookmark(item.messageId) }) { Text(stringResource(R.string.remove_bookmark)) }
                        }
                    }
                    HorizontalDivider()
                }
            }
        }
    }
    if (confirmDelete) DeleteSelectedDialog(selected.size, { confirmDelete = false }) {
        viewModel.deleteBookmarks(selected)
        selected = emptySet()
        confirmDelete = false
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun NotesScreen(state: AppUiState, viewModel: AppViewModel, showBack: Boolean) {
    var search by rememberSaveable { mutableStateOf("") }
    var editing by remember { mutableStateOf<Note?>(null) }
    var reading by remember { mutableStateOf<Note?>(null) }
    var selected by remember { mutableStateOf(emptySet<String>()) }
    var confirmDelete by remember { mutableStateOf(false) }
    val currentReading = reading?.let { selectedNote ->
        state.notes.firstOrNull { it.id == selectedNote.id } ?: selectedNote
    }
    val navigateBack = {
        when {
            editing != null -> editing = null
            reading != null -> reading = null
            else -> viewModel.openScreen(AppScreen.CHAT)
        }
    }
    BackHandler(enabled = showBack || editing != null || currentReading != null) { navigateBack() }
    val title = when {
        editing != null -> stringResource(if (reading == null) R.string.new_note else R.string.edit)
        currentReading != null -> currentReading.title.ifBlank { stringResource(R.string.notes) }
        else -> stringResource(R.string.notes)
    }
    val toolbarAction: (@Composable () -> Unit)? = when {
        editing != null -> null
        currentReading != null -> {{
            IconButton(
                onClick = { editing = currentReading },
                modifier = Modifier.testTag("note_reader_edit"),
            ) {
                Icon(Icons.Outlined.Edit, stringResource(R.string.edit))
            }
        }}
        else -> {{
            IconButton(
                onClick = { editing = Note() },
                modifier = Modifier.testTag("note_create"),
            ) {
                Icon(Icons.Outlined.Add, stringResource(R.string.new_note))
            }
        }}
    }
    WorkspaceScaffold(
        title = title,
        showBack = showBack || editing != null || currentReading != null,
        onBack = navigateBack,
        action = toolbarAction,
    ) { padding ->
        when {
            editing != null -> NoteEditor(
                note = editing!!,
                onSave = { saved ->
                    viewModel.saveNote(saved)
                    reading = saved
                    editing = null
                },
                modifier = Modifier.padding(padding),
            )
            currentReading != null -> NoteReader(currentReading, Modifier.padding(padding))
            else -> Column(Modifier.padding(padding).fillMaxSize().padding(horizontal = 16.dp)) {
            OutlinedTextField(search, { search = it }, placeholder = { Text(stringResource(R.string.search_notes)) }, leadingIcon = { Icon(Icons.Outlined.Search, null) }, singleLine = true, modifier = Modifier.fillMaxWidth().widthIn(max = 760.dp).align(Alignment.CenterHorizontally))
            val notes = state.notes.filter { search.isBlank() || it.title.contains(search, true) || it.body.contains(search, true) }
            if (selected.isNotEmpty()) SelectionToolbar(
                selectedCount = selected.size,
                allVisibleSelected = notes.isNotEmpty() && notes.all { it.id in selected },
                onSelectAll = {
                    val visible = notes.map(Note::id).toSet()
                    selected = if (visible.isNotEmpty() && visible.all { it in selected }) selected - visible else selected + visible
                },
                onDelete = { confirmDelete = true },
                onClear = { selected = emptySet() },
                modifier = Modifier.fillMaxWidth().widthIn(max = 820.dp).align(Alignment.CenterHorizontally),
            )
            if (notes.isEmpty()) EmptyWorkspace(Icons.Outlined.NoteAlt, stringResource(R.string.empty_notes), Modifier.weight(1f))
            else LazyColumn(Modifier.weight(1f).fillMaxWidth().widthIn(max = 820.dp).align(Alignment.CenterHorizontally), contentPadding = PaddingValues(vertical = 10.dp)) {
                items(notes, key = Note::id) { note ->
                    val isSelected = note.id in selected
                    Row(
                        Modifier.fillMaxWidth()
                            .combinedClickable(
                                onClick = {
                                    if (selected.isEmpty()) reading = note
                                    else selected = if (isSelected) selected - note.id else selected + note.id
                                },
                                onLongClick = { selected = if (isSelected) selected - note.id else selected + note.id },
                            )
                            .padding(vertical = 12.dp)
                            .testTag(UiTestTags.noteItem(note.id)),
                        verticalAlignment = Alignment.Top,
                    ) {
                        Column(Modifier.weight(1f)) {
                            Text(note.title, fontWeight = FontWeight.SemiBold)
                            Text(note.body, maxLines = 3, overflow = TextOverflow.Ellipsis, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 6.dp))
                        }
                        if (selected.isNotEmpty()) Checkbox(isSelected, { checked -> selected = if (checked) selected + note.id else selected - note.id })
                        else Row {
                            if (note.sourceMessageId != null) IconButton(onClick = { viewModel.summarizeNoteTitle(note.id) }) {
                                Icon(Icons.Outlined.AutoAwesome, stringResource(R.string.summarize_title))
                            }
                            IconButton(onClick = { viewModel.deleteNote(note.id) }) { Icon(Icons.Outlined.DeleteOutline, stringResource(R.string.delete)) }
                        }
                    }
                    HorizontalDivider()
                }
            }
        }
        }
    }
    if (confirmDelete) DeleteSelectedDialog(selected.size, { confirmDelete = false }) {
        viewModel.deleteNotes(selected)
        selected = emptySet()
        confirmDelete = false
    }
}

@Composable
private fun NoteReader(note: Note, modifier: Modifier = Modifier) {
    Column(
        modifier
            .fillMaxSize()
            .navigationBarsPadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 12.dp)
            .testTag("note_reader"),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        MarkdownContent(
            markdown = note.body,
            modifier = Modifier.fillMaxWidth().widthIn(max = 860.dp),
        )
    }
}

@Composable
private fun SelectionToolbar(
    selectedCount: Int,
    allVisibleSelected: Boolean,
    onSelectAll: () -> Unit,
    onDelete: () -> Unit,
    onClear: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(modifier.padding(top = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(stringResource(R.string.selected_items, selectedCount), Modifier.weight(1f), style = MaterialTheme.typography.labelLarge)
        IconButton(onClick = onSelectAll, modifier = Modifier.testTag(UiTestTags.WORKSPACE_SELECT_ALL)) {
            Icon(Icons.Outlined.SelectAll, stringResource(R.string.select_all), tint = if (allVisibleSelected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant)
        }
        IconButton(onClick = onDelete, modifier = Modifier.testTag(UiTestTags.WORKSPACE_DELETE_SELECTED)) { Icon(Icons.Outlined.DeleteOutline, stringResource(R.string.delete)) }
        IconButton(onClick = onClear) { Icon(Icons.Outlined.Close, stringResource(R.string.clear_selection)) }
    }
}

@Composable
private fun DeleteSelectedDialog(count: Int, onDismiss: () -> Unit, onConfirm: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.confirm_delete_selected_items)) },
        text = { Text(pluralStringResource(R.plurals.delete_selected_items_detail, count, count)) },
        confirmButton = { Button(onClick = onConfirm) { Text(stringResource(R.string.delete)) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}

@Composable
private fun NoteEditor(note: Note, onSave: (Note) -> Unit, modifier: Modifier = Modifier) {
    var title by remember(note.id) { mutableStateOf(note.title) }
    var body by remember(note.id) { mutableStateOf(note.body) }
    var preview by rememberSaveable { mutableStateOf(false) }
    Column(
        modifier.fillMaxSize().padding(16.dp).navigationBarsPadding().testTag("note_editor"),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
            FilterChip(preview, { preview = !preview }, label = { Text(stringResource(R.string.preview)) })
        }
        OutlinedTextField(title, { title = it }, label = { Text(stringResource(R.string.note_title)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
        if (preview) MarkdownContent(body, Modifier.weight(1f).fillMaxWidth().verticalScroll(rememberScrollState()).padding(12.dp))
        else OutlinedTextField(body, { body = it }, label = { Text(stringResource(R.string.note_body)) }, modifier = Modifier.weight(1f).fillMaxWidth())
        Button(onClick = { onSave(note.copy(title = title, body = body)) }, enabled = body.isNotBlank(), modifier = Modifier.align(Alignment.End)) { Text(stringResource(R.string.save)) }
    }
}

@Composable
private fun AgentsScreen(state: AppUiState, viewModel: AppViewModel, showBack: Boolean) {
    var editing by remember { mutableStateOf<AgentProfile?>(null) }
    WorkspaceScaffold(
        stringResource(R.string.agents), showBack, { viewModel.openScreen(AppScreen.CHAT) },
        action = { IconButton(onClick = { editing = AgentProfile() }) { Icon(Icons.Outlined.Add, stringResource(R.string.new_agent)) } },
    ) { padding ->
        if (editing != null) AgentEditor(editing!!, state, { editing = null }) { viewModel.saveAgent(it); editing = null }
        else if (state.agents.isEmpty()) EmptyWorkspace(Icons.Outlined.SmartToy, stringResource(R.string.empty_agents), Modifier.padding(padding).fillMaxSize())
        else LazyColumn(Modifier.padding(padding).fillMaxSize().widthIn(max = 860.dp), contentPadding = PaddingValues(16.dp)) {
            items(state.agents, key = AgentProfile::id) { agent ->
                Row(Modifier.fillMaxWidth().clickable { editing = agent }.padding(vertical = 14.dp), verticalAlignment = Alignment.CenterVertically) {
                    Surface(color = MaterialTheme.colorScheme.primaryContainer, shape = RoundedCornerShape(8.dp)) { Icon(Icons.Outlined.SmartToy, null, Modifier.padding(10.dp)) }
                    Column(Modifier.padding(horizontal = 12.dp).weight(1f)) {
                        Text(agent.name, fontWeight = FontWeight.SemiBold)
                        Text(agent.description, maxLines = 2, overflow = TextOverflow.Ellipsis, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    IconButton(onClick = { viewModel.startAgent(agent.id) }) { Icon(Icons.Outlined.PlayArrow, stringResource(R.string.start_chat)) }
                    IconButton(onClick = { viewModel.deleteAgent(agent.id) }) { Icon(Icons.Outlined.DeleteOutline, stringResource(R.string.delete)) }
                }
                HorizontalDivider()
            }
        }
    }
}

@Composable
private fun AgentEditor(agent: AgentProfile, state: AppUiState, onClose: () -> Unit, onSave: (AgentProfile) -> Unit) {
    var draft by remember(agent.id) { mutableStateOf(agent) }
    var modelMenu by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxSize().padding(16.dp).verticalScroll(rememberScrollState()).navigationBarsPadding(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            IconButton(onClick = onClose) { Icon(Icons.AutoMirrored.Outlined.ArrowBack, stringResource(R.string.back)) }
            Text(stringResource(R.string.agents), style = MaterialTheme.typography.titleLarge)
        }
        OutlinedTextField(draft.name, { draft = draft.copy(name = it) }, label = { Text(stringResource(R.string.agent_name)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(draft.description, { draft = draft.copy(description = it) }, label = { Text(stringResource(R.string.agent_description)) }, minLines = 2, modifier = Modifier.fillMaxWidth())
        Box {
            OutlinedButton(onClick = { modelMenu = true }, modifier = Modifier.fillMaxWidth()) { Text(state.models.firstOrNull { it.id == draft.modelId }?.displayName ?: stringResource(R.string.select_model), Modifier.weight(1f)) }
            DropdownMenu(modelMenu, { modelMenu = false }) { state.models.forEach { model -> DropdownMenuItem({ Text(model.displayName) }, { draft = draft.copy(modelId = model.id); modelMenu = false }) } }
        }
        OutlinedTextField(draft.systemPrompt, { draft = draft.copy(systemPrompt = it) }, label = { Text(stringResource(R.string.system_prompt)) }, minLines = 5, modifier = Modifier.fillMaxWidth())
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) { listOf("off", "low", "medium", "high").forEach { effort -> FilterChip(draft.thinkingEffort == effort, { draft = draft.copy(thinkingEffort = effort) }, label = { Text(effort) }) } }
        Text("${stringResource(R.string.max_tool_calls)}: ${draft.maxToolCalls}")
        Slider(draft.maxToolCalls.toFloat(), { draft = draft.copy(maxToolCalls = it.roundToInt()) }, valueRange = 0f..20f, steps = 19)
        AgentToggle(stringResource(R.string.search_web), draft.enableSearch) { draft = draft.copy(enableSearch = it) }
        AgentToggle(stringResource(R.string.read_web), draft.enableRead) { draft = draft.copy(enableRead = it) }
        AgentToggle(stringResource(R.string.enable_knowledge), draft.enableKnowledge) { draft = draft.copy(enableKnowledge = it) }
        Button(onClick = { onSave(draft) }, enabled = draft.name.isNotBlank() && draft.modelId != null, modifier = Modifier.align(Alignment.End)) { Text(stringResource(R.string.save)) }
    }
}

@Composable
private fun AgentToggle(label: String, checked: Boolean, onChange: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) { Text(label, Modifier.weight(1f)); Switch(checked, onChange) }
}

@Composable
private fun KnowledgeScreen(state: AppUiState, viewModel: AppViewModel, showBack: Boolean) {
    val context = LocalContext.current
    var query by rememberSaveable { mutableStateOf("") }
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        uri ?: return@rememberLauncherForActivityResult
        var name = uri.lastPathSegment ?: "document"
        var size = -1L
        context.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE), null, null, null)?.use { cursor ->
            if (cursor.moveToFirst()) {
                name = cursor.getString(0) ?: name
                if (!cursor.isNull(1)) size = cursor.getLong(1)
            }
        }
        viewModel.importKnowledge(KnowledgeImportSource(uri.toString(), name, context.contentResolver.getType(uri).orEmpty(), size))
    }
    WorkspaceScaffold(
        stringResource(R.string.knowledge), showBack, { viewModel.openScreen(AppScreen.CHAT) },
        action = { IconButton(onClick = { picker.launch(arrayOf("text/plain", "text/markdown", "application/json", "text/csv", "application/pdf")) }) { Icon(Icons.Outlined.UploadFile, stringResource(R.string.import_document)) } },
    ) { padding ->
        Column(Modifier.padding(padding).fillMaxSize().padding(horizontal = 16.dp)) {
            Row(Modifier.fillMaxWidth().widthIn(max = 820.dp).align(Alignment.CenterHorizontally), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(query, { query = it }, placeholder = { Text(stringResource(R.string.search_knowledge)) }, leadingIcon = { Icon(Icons.Outlined.Search, null) }, singleLine = true, modifier = Modifier.weight(1f))
                FilledTonalButton(onClick = { viewModel.searchKnowledge(query) }, enabled = query.isNotBlank()) { Text(stringResource(R.string.search_action)) }
            }
            if (state.pendingKnowledgeChunkIds.isNotEmpty()) Text(stringResource(R.string.attached_count, state.pendingKnowledgeChunkIds.size), color = MaterialTheme.colorScheme.primary, modifier = Modifier.align(Alignment.CenterHorizontally).padding(top = 8.dp))
            if (state.knowledgeResults.isNotEmpty()) LazyColumn(Modifier.weight(1f).fillMaxWidth().widthIn(max = 820.dp).align(Alignment.CenterHorizontally)) {
                items(state.knowledgeResults, key = { it.chunkId }) { snippet ->
                    Row(Modifier.fillMaxWidth().padding(vertical = 10.dp), verticalAlignment = Alignment.Top) {
                        Checkbox(snippet.chunkId in state.pendingKnowledgeChunkIds, { viewModel.toggleKnowledgeAttachment(snippet.chunkId) })
                        Column(Modifier.weight(1f)) { Text("${snippet.documentName} · #${snippet.position + 1}", color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.labelLarge); Text(snippet.text, maxLines = 5, overflow = TextOverflow.Ellipsis) }
                    }
                    HorizontalDivider()
                }
            } else if (state.knowledgeDocuments.isEmpty()) EmptyWorkspace(Icons.Outlined.FolderOpen, stringResource(R.string.empty_knowledge), Modifier.weight(1f))
            else LazyColumn(Modifier.weight(1f).fillMaxWidth().widthIn(max = 820.dp).align(Alignment.CenterHorizontally), contentPadding = PaddingValues(vertical = 12.dp)) {
                items(state.knowledgeDocuments, key = { it.id }) { document ->
                    Row(Modifier.fillMaxWidth().padding(vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                        Icon(if (document.status == "error") Icons.Outlined.ErrorOutline else if (document.status == "ready") Icons.Outlined.CheckCircle else Icons.Outlined.Description, null, tint = if (document.status == "error") MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary)
                        Column(Modifier.padding(horizontal = 12.dp).weight(1f)) {
                            Text(document.name, fontWeight = FontWeight.Medium)
                            Text(if (document.status == "error") document.error else "${document.chunkCount} chunks · ${document.sizeBytes / 1024} KiB", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        IconButton(onClick = { viewModel.deleteKnowledge(document.id) }) { Icon(Icons.Outlined.DeleteOutline, stringResource(R.string.delete)) }
                    }
                    HorizontalDivider()
                }
            }
        }
    }
}

@Composable
private fun AboutScreen(viewModel: AppViewModel, showBack: Boolean) {
    WorkspaceScaffold(stringResource(R.string.about), showBack, { viewModel.openScreen(AppScreen.CHAT) }) { padding ->
        Column(Modifier.padding(padding).fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp).widthIn(max = 760.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) { AppLogo(56.dp); Column(Modifier.padding(start = 16.dp)) { Text(stringResource(R.string.app_name), style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.SemiBold); Text(stringResource(R.string.version_label, BuildConfig.VERSION_NAME)) } }
            HorizontalDivider()
            Text(stringResource(R.string.local_data), style = MaterialTheme.typography.titleMedium)
            Text(stringResource(R.string.privacy_summary), color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text(stringResource(R.string.dependencies), style = MaterialTheme.typography.titleMedium)
            Text(stringResource(R.string.dependencies_detail), color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun EmptyWorkspace(icon: androidx.compose.ui.graphics.vector.ImageVector, text: String, modifier: Modifier = Modifier) {
    Box(modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.padding(32.dp)) {
            Surface(color = MaterialTheme.colorScheme.primaryContainer, shape = RoundedCornerShape(8.dp)) { Icon(icon, null, Modifier.padding(14.dp).size(28.dp), tint = MaterialTheme.colorScheme.primary) }
            Text(text, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}
