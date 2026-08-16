import { esc, cookie } from "../core/dom.js";
import { formatCompactNumber } from "../core/format.js";
import { confirmAction } from "../core/confirm.js";
import { showToast, copyText } from "../core/toast.js";
import { loadHTMLPreview, setCodeBlockView } from "./html-preview.js";
import { renderMarkdown } from "./markdown.js";
import { CLIENT_RETRY_DELAYS_MS, canRetryStream, chatViewportFrame, createRequestID, isNearBottom, messageEvents, normalizeMaxToolCalls, normalizeTokenUsage, readSSE, scheduleAutoTitle, tokenUsageDetailKeys, waitForDelay } from "./runtime.js";

const roots = typeof document === "undefined" ? [] : Array.from(document.querySelectorAll("[data-chat-root]"));

const defaultUserAvatar = "😀";
const defaultAssistantAvatar = "🤖";
const userAvatarPresets = ["😀", "😎", "🙂", "😊", "🧑‍💻", "👩‍💻", "🧑‍🚀", "🧑‍🎨", "🧑‍🔬", "🧑‍🏫", "🐱", "🐼"];
const assistantAvatarPresets = ["🤖", "🧠", "✨", "🦾", "📚", "🛠️", "🔎", "🧭", "💡", "🛰️", "🎯", "🚀"];
const desktopSidebarKey = "tokenflow.chat.sidebarCollapsed";

const translations = {
  en: {
    assistant: "Assistant",
    assistant_avatar: "Assistant avatar",
    assistant_avatar_presets: "Assistant avatar presets",
    ask_placeholder: "Message TokenFlow",
    avatar_settings: "Avatar settings",
    cache_creation_tokens: "Cache write",
    cache_read_tokens: "Cache hit",
    calling_model: "Calling model",
    cancel: "Cancel",
    chat_settings: "Chat settings",
    close: "Close",
    close_conversations: "Close conversations",
    close_settings: "Close settings",
    collapse_sidebar: "Collapse sidebar",
    completed: "completed",
    conversation_actions: "Conversation actions",
    conversation_busy: "This conversation is processing. Please try again later.",
    conversation_name: "Conversation name",
    conversations: "Conversations",
    copied: "Copied",
    copy: "Copy",
    copy_response: "Copy response",
    default_system_prompt: "Default system prompt",
    default_system_prompt_empty: "Default system prompt is unavailable.",
    default_system_prompt_hint: "Always applied by TokenFlow. Your instructions below are appended.",
    delete: "Delete",
    delete_confirm: "Delete this conversation? This cannot be undone.",
    delete_title: "Delete conversation",
    failed: "failed",
    generate_title: "Generate title",
    greeting: "How can I help?",
    high: "High",
    identity: "Identity",
    input_tokens: "Input",
    instructions: "Instructions",
    loading: "Loading conversations...",
    low: "Low",
    medium: "Medium",
    model: "Model",
    model_and_reasoning: "Model and reasoning",
    model_loading: "Loading model",
    max_tool_calls: "Max tool calls",
    max_tool_calls_summary: "Tools {count}x",
	markdown_table: "Markdown table",
	code: "Code",
	code_views: "Code block views",
	copy_code: "Copy code",
	html_preview: "HTML preview",
	preview: "Preview",
	reload_preview: "Reload preview",
	preview_navigation_blocked: "Preview navigation was blocked.",
	message_too_large: "Message exceeds the 128K character limit.",
	context_too_large: "This message and the current instructions exceed the chat context limit.",
	model_unavailable: "The selected model is no longer available. Choose another model.",
	accounting_failed: "The response was generated, but usage accounting failed. Please try again later.",
	estimated: "estimated",
	exact: "exact",
	measurement: "Measurement",
	process_saved_hint: "This setting only changes display. Process data is still saved by the server.",
	input_characters_remaining: "{count} characters remaining",
    my_nickname: "My nickname",
    new_chat: "New chat",
    no_conversations: "No conversations yet",
    no_matching_conversations: "No matching conversations",
    no_model: "No model",
    no_models: "No models are available",
    no_models_detail: "Ask an administrator to enable a provider and model before starting a chat.",
    off: "Off",
    output_tokens: "Output",
    process: "Process",
    process_count: "Process · {count} events",
    read_web: "Read web",
    regenerate: "Regenerate response",
    retrying: "Connection interrupted. Retrying {attempt}/{max}...",
    rename: "Rename",
    rename_title: "Rename conversation",
    request_failed: "Request failed",
    responding: "Responding",
    save: "Save",
    saved_to_conversation: "Saved to this conversation",
    scroll_bottom: "Scroll to bottom",
    search: "Search",
    search_conversations: "Search conversations",
    sources: "Sources",
    send: "Send",
    settings_saved: "Settings saved",
    show_conversations: "Show conversations",
    start_conversation: "Start a new conversation",
    started: "started",
    starting: "Starting",
    status: "Status",
    status_failed: "Failed",
    status_responding: "Responding",
    status_stopped: "Stopped",
    status_title_generating: "Generating title",
    stop: "Stop",
    stopping: "Stopping",
    stopped: "Stopped",
    stream_invalid: "The streaming response could not be read.",
    system_prompt: "Additional system prompt",
    system_prompt_placeholder: "Optional instructions for this conversation",
    thinking: "Thinking",
    title_generated: "Title updated",
    title_generating: "Generating title",
    title_no_messages: "Current conversation has no messages to summarize.",
    message_not_latest: "Only the latest response can be regenerated.",
    token_usage: "Token usage",
    tool_controls: "Tools",
    tools: "Tools",
    total_tokens: "Total",
    user_avatar: "User avatar",
    user_avatar_presets: "User avatar presets",
    wait_settings: "Wait for the current response to finish before changing settings.",
    warning: "Warning",
    you: "You",
  },
  "zh-CN": {
    assistant: "助手",
    assistant_avatar: "助手头像",
    assistant_avatar_presets: "助手头像预设",
    ask_placeholder: "给一念通流 TokenFlow 发送消息",
    avatar_settings: "头像设置",
    cache_creation_tokens: "缓存写入",
    cache_read_tokens: "缓存命中",
    calling_model: "正在调用模型",
    cancel: "取消",
    chat_settings: "聊天设置",
    close: "关闭",
    close_conversations: "关闭会话列表",
    close_settings: "关闭设置",
    collapse_sidebar: "收起侧栏",
    completed: "已完成",
    conversation_actions: "会话操作",
    conversation_busy: "当前会话正在处理中，请稍后再试。",
    conversation_name: "会话名称",
    conversations: "会话",
    copied: "已复制",
    copy: "复制",
    copy_response: "复制回复",
    default_system_prompt: "默认系统提示词",
    default_system_prompt_empty: "默认系统提示词暂不可用。",
    default_system_prompt_hint: "一念通流 TokenFlow 始终应用默认提示词，下方内容将作为附加指令。",
    delete: "删除",
    delete_confirm: "确定删除这个会话吗？此操作无法撤销。",
    delete_title: "删除会话",
    failed: "失败",
    generate_title: "自动生成标题",
    greeting: "有什么可以帮你？",
    high: "高",
    identity: "身份与头像",
    input_tokens: "输入",
    instructions: "提示词",
    loading: "正在加载会话...",
    low: "低",
    medium: "中",
    model: "模型",
    model_and_reasoning: "模型与思考",
    model_loading: "模型加载中",
    max_tool_calls: "最大工具调用次数",
    max_tool_calls_summary: "工具 {count} 次",
	markdown_table: "Markdown 表格",
	code: "代码",
	code_views: "代码块视图",
	copy_code: "复制代码",
	html_preview: "HTML 预览",
	preview: "预览",
	reload_preview: "重新加载预览",
	preview_navigation_blocked: "已阻止预览页面跳转。",
	message_too_large: "消息超过 128K 字符上限。",
	context_too_large: "消息与当前指令超过了聊天上下文上限。",
	model_unavailable: "当前模型已不可用，请重新选择模型。",
	accounting_failed: "回复已生成，但用量记账失败，请稍后重试。",
	estimated: "估算",
	exact: "精确",
	measurement: "计量方式",
	process_saved_hint: "此开关只影响显示，过程数据仍会由服务器保存。",
	input_characters_remaining: "还可输入 {count} 个字符",
    my_nickname: "我的昵称",
    new_chat: "新对话",
    no_conversations: "暂无历史会话",
    no_matching_conversations: "没有匹配的会话",
    no_model: "无模型",
    no_models: "暂无可用模型",
    no_models_detail: "请联系管理员启用供应商与模型后再开始对话。",
    off: "关闭",
    output_tokens: "输出",
    process: "过程",
    process_count: "过程 · {count} 项",
    read_web: "读取网页",
    regenerate: "重新生成回复",
    retrying: "连接中断，正在重试 {attempt}/{max}...",
    rename: "重命名",
    rename_title: "重命名会话",
    request_failed: "请求失败",
    responding: "回复中",
    save: "保存",
    saved_to_conversation: "保存到当前会话",
    scroll_bottom: "回到底部",
    search: "联网搜索",
    search_conversations: "搜索会话",
    sources: "来源",
    send: "发送",
    settings_saved: "设置已保存",
    show_conversations: "显示会话列表",
    start_conversation: "开始一个新对话",
    started: "已开始",
    starting: "正在开始",
    status: "状态",
    status_failed: "失败",
    status_responding: "回复中",
    status_stopped: "已停止",
    status_title_generating: "标题生成中",
    stop: "停止",
    stopping: "正在停止",
    stopped: "已停止",
    stream_invalid: "无法读取流式响应。",
    system_prompt: "附加系统提示词",
    system_prompt_placeholder: "可选，为当前会话添加额外指令",
    thinking: "思考强度",
    title_generated: "标题已更新",
    title_generating: "标题生成中",
    title_no_messages: "当前会话没有可总结的消息。",
    message_not_latest: "只能重新生成最后一条回复。",
    token_usage: "Token 用量",
    tool_controls: "工具",
    tools: "工具",
    total_tokens: "总计",
    user_avatar: "用户头像",
    user_avatar_presets: "用户头像预设",
    wait_settings: "请等待当前回复完成后再修改设置。",
    warning: "警告",
    you: "你",
  },
};

roots.forEach((root) => initChat(root));

function initChat(root) {
  const apiPrefix = root.dataset.chatApiPrefix;
  const csrfCookie = root.dataset.chatCsrfCookie;
  const lang = normalizeLang(root.dataset.chatLang || document.documentElement.lang || "en");
  const t = (key, values) => translate(lang, key, values);
  const numberFormat = new Intl.NumberFormat(lang);
	const browserTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || "";
  const mobileMedia = window.matchMedia("(max-width: 960px)");
  const state = {
    conversations: [],
    messages: [],
    models: [],
    defaultSystemPrompt: "",
    defaultMaxToolCalls: 7,
	maxUserMessageChars: 131072,
    activeId: null,
    activeConversation: null,
    draft: null,
    draftCreating: null,
    messagesByConversationId: new Map(),
    streamsByConversationId: new Map(),
    titleGeneratingIds: new Set(),
    autoTitleTimersByConversationId: new Map(),
    pollTimer: null,
    renderFrame: 0,
    sidebarCollapsed: readSidebarPreference(),
    menuConversationId: null,
    settingsPreviousFocus: null,
    settingsDirty: false,
    loading: true,
  };

  const el = {
    sidebar: root.querySelector("[data-chat-sidebar]"),
    sidebarBackdrop: root.querySelector("[data-chat-sidebar-backdrop]"),
    sidebarToggle: root.querySelector("[data-chat-sidebar-toggle]"),
    sidebarCollapse: root.querySelector("[data-chat-sidebar-collapse]"),
    list: root.querySelector("[data-chat-conversations]"),
    conversationSearch: root.querySelector("[data-chat-conversation-search]"),
    messages: root.querySelector("[data-chat-messages]"),
    scrollBottom: root.querySelector("[data-chat-scroll-bottom]"),
    title: root.querySelector("[data-chat-title]"),
    conversationMenuToggle: root.querySelector("[data-chat-conversation-menu-toggle]"),
    conversationMenu: root.querySelector("[data-chat-conversation-menu]"),
    accountToggle: root.querySelector("[data-chat-account-toggle]"),
    accountMenu: root.querySelector("[data-chat-account-menu]"),
    accountAvatar: root.querySelector(".chat-account-avatar"),
    toolsToggle: root.querySelector("[data-chat-tools-toggle]"),
    toolsMenu: root.querySelector("[data-chat-tools-menu]"),
    toolStatus: root.querySelector("[data-chat-tool-status]"),
    model: root.querySelector("[data-chat-model]"),
    settingsOpen: Array.from(root.querySelectorAll("[data-chat-settings-open]")),
    settingsModal: root.querySelector("[data-chat-settings-modal]"),
    settingsForm: root.querySelector("[data-chat-settings-form]"),
    settingsCancel: root.querySelector("[data-chat-settings-cancel]"),
    settingsSummary: root.querySelector("[data-chat-settings-summary]"),
    defaultSystemPrompt: root.querySelector("[data-chat-default-system-prompt]"),
    systemPrompt: root.querySelector("[data-chat-system-prompt]"),
    nickname: root.querySelector("[data-chat-nickname]"),
    userAvatar: root.querySelector("[data-chat-user-avatar]"),
    assistantAvatar: root.querySelector("[data-chat-assistant-avatar]"),
    maxToolCalls: root.querySelector("[data-chat-max-tool-calls]"),
    settingsSave: root.querySelector("[data-chat-settings-save]"),
    form: root.querySelector("[data-chat-form]"),
    input: root.querySelector("[data-chat-input]"),
	characterCount: root.querySelector("[data-chat-character-count]"),
    search: root.querySelector("[data-chat-search]"),
    read: root.querySelector("[data-chat-read]"),
    processToggle: root.querySelector("[data-chat-process]"),
    stop: root.querySelector("[data-chat-stop]"),
    send: root.querySelector("[data-chat-send]"),
    autoTitle: root.querySelector("[data-chat-auto-title]"),
    rename: root.querySelector("[data-chat-rename]"),
    delete: root.querySelector("[data-chat-delete]"),
    new: root.querySelector("[data-chat-new]"),
  };

  const api = async (path, options = {}) => {
    const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
    if (options.method && options.method !== "GET") headers["X-CSRF-Token"] = cookie(csrfCookie);
    const response = await fetch(`${apiPrefix}${path}`, { ...options, headers });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw chatAPIError(body);
    return body;
  };

  localizeStatic();
  applySidebarState();
  renderLoading();
  updateToolStatus();
  root.dataset.chatReady = "true";
  autosizeInput();
  bindViewport();
  bindEvents();
  load();

  async function load() {
    try {
      const [models, conversations] = await Promise.all([
        api("/models"),
        api("/conversations"),
      ]);
      state.models = models.models || [];
      state.defaultSystemPrompt = models.default_system_prompt || "";
	  state.maxUserMessageChars = Number(models.max_user_message_chars) || 131072;
      state.defaultMaxToolCalls = normalizeMaxToolCalls(models.max_tool_calls ?? state.defaultMaxToolCalls);
      state.conversations = conversations.items || [];
      state.loading = false;
      renderModels();
	  updateCharacterCount();
      renderConversations();
      if (state.conversations.length) {
        await openConversation(state.conversations[0].id, { forceBottom: true });
      } else {
        beginDraft({ inherit: false });
      }
      startConversationPolling();
    } catch (error) {
      state.loading = false;
      showToast(error.message, "error");
      renderLoadError(error.message);
    }
  }

  function renderLoading() {
    el.messages.innerHTML = `<div class="chat-empty"><img src="/admin/static/tokenflow-logo.png" alt=""><p>${esc(t("loading"))}</p></div>`;
  }

  function renderLoadError(message) {
    el.messages.innerHTML = `<div class="chat-empty chat-load-error"><img src="/admin/static/tokenflow-logo.png" alt=""><h2>${esc(t("request_failed"))}</h2><p>${esc(message)}</p></div>`;
  }

  function renderModels() {
    el.model.innerHTML = state.models.length
      ? state.models.map((model) => `<option value="${esc(model)}">${esc(model)}</option>`).join("")
      : `<option value="">${esc(t("no_models"))}</option>`;
    populateSettings(currentConfig());
  }

  function beginDraft(options = {}) {
    const source = options.inherit === false ? {} : currentConfig();
    state.activeId = null;
    state.activeConversation = null;
    state.messages = [];
    state.draft = {
      title: "",
      model: source.model || state.models[0] || "",
      thinking_effort: source.thinking_effort || "medium",
      system_prompt: source.system_prompt || "",
      nickname: source.nickname || "",
      user_avatar: avatarValue(source.user_avatar, defaultUserAvatar),
      assistant_avatar: avatarValue(source.assistant_avatar, defaultAssistantAvatar),
      max_tool_calls: state.defaultMaxToolCalls,
    };
    populateSettings(state.draft);
    renderHeader();
    renderConversations();
    renderMessages({ forceBottom: true });
    syncDisabled();
    closeAllMenus();
    closeMobileSidebar();
    requestAnimationFrame(() => el.input.focus({ preventScroll: true }));
  }

  async function createConversationFromDraft() {
    if (state.activeId) return state.activeId;
    if (state.draftCreating) return state.draftCreating;
    const draft = { ...(state.draft || draftFromForm()) };
    state.draftCreating = (async () => {
      syncDisabled();
      const conv = await api("/conversations", {
        method: "POST",
        body: JSON.stringify({
          title: "",
          model: draft.model || state.models[0] || "",
          thinking_effort: draft.thinking_effort || "medium",
          system_prompt: draft.system_prompt || "",
          nickname: draft.nickname || "",
          user_avatar: avatarValue(draft.user_avatar, defaultUserAvatar),
          assistant_avatar: avatarValue(draft.assistant_avatar, defaultAssistantAvatar),
          max_tool_calls: normalizeMaxToolCalls(draft.max_tool_calls),
        }),
      });
      state.activeId = conv.id;
      state.activeConversation = conv;
      state.draft = null;
      state.conversations = [conv, ...state.conversations.filter((item) => item.id !== conv.id)];
      state.messagesByConversationId.set(conv.id, state.messages);
      populateSettings(conv);
      renderHeader();
      renderConversations();
      return conv.id;
    })();
    try {
      return await state.draftCreating;
    } finally {
      state.draftCreating = null;
      syncDisabled();
    }
  }

  async function openConversation(id, options = {}) {
    const body = await api(`/conversations/${encodeURIComponent(id)}`);
    state.activeId = body.conversation.id;
    state.activeConversation = body.conversation;
    state.draft = null;
    if (!state.messagesByConversationId.has(state.activeId)) {
      state.messagesByConversationId.set(state.activeId, body.messages || []);
    }
    state.messages = conversationMessages(state.activeId);
    populateSettings(body.conversation);
    renderHeader();
    renderConversations();
    renderMessages({ forceBottom: options.forceBottom !== false });
    syncDisabled();
    closeAllMenus();
    closeMobileSidebar();
  }

  function renderHeader() {
    const conversation = currentConfig();
    el.title.textContent = displayConversationTitle(conversation);
    renderSettingsSummary(conversation);
  }

  function renderConversations() {
    const query = String(el.conversationSearch.value || "").trim().toLocaleLowerCase(lang);
    const visible = query
      ? state.conversations.filter((conv) => `${displayConversationTitle(conv)} ${conv.model || ""}`.toLocaleLowerCase(lang).includes(query))
      : state.conversations;
    if (!visible.length) {
      el.list.innerHTML = `<p class="empty">${esc(query ? t("no_matching_conversations") : t("no_conversations"))}</p>`;
      return;
    }
    el.list.innerHTML = visible.map((conv) => {
      const busy = isConversationBusyById(conv.id, conv);
      const status = conversationStatus(conv);
      const statusLabel = conversationStatusLabel(status);
      return `
        <div class="chat-conversation-row ${conv.id === state.activeId ? "active" : ""}">
          <button type="button" class="chat-conversation" data-chat-open="${esc(conv.id)}">
            <span class="chat-conversation-title-line">
              <span class="chat-conversation-name">${esc(displayConversationTitle(conv))}</span>
              ${statusLabel ? `<span class="chat-status-dot ${esc(status)}" title="${esc(statusLabel)}" aria-label="${esc(statusLabel)}"></span>` : ""}
            </span>
            <small>${esc(conv.model || t("no_model"))}</small>
          </button>
          <div class="chat-conversation-actions">
            <button type="button" class="chat-conversation-action" data-chat-row-menu="${esc(conv.id)}" title="${esc(t("conversation_actions"))}" aria-label="${esc(t("conversation_actions"))}" ${busy ? "disabled" : ""}><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-more"></use></svg></button>
          </div>
        </div>`;
    }).join("");
  }

  function renderMessages(options = {}) {
    const follow = options.forceBottom || isNearBottom(el.messages);
    const previousTop = el.messages.scrollTop;
    if (!state.messages.length) {
      const noModels = !state.models.length;
      el.messages.innerHTML = `<div class="chat-empty"><img src="/admin/static/tokenflow-logo.png" alt=""><h2>${esc(noModels ? t("no_models") : t("greeting"))}</h2><p>${esc(noModels ? t("no_models_detail") : t("start_conversation"))}</p></div>`;
    } else {
      el.messages.innerHTML = state.messages.map((message, index) => renderMessage(message, index)).join("");
    }
    if (follow) scrollMessagesToBottom(false);
    else el.messages.scrollTop = previousTop;
    updateScrollButton();
  }

  function renderMessage(message, index) {
    const conversation = currentConfig();
    const isUser = message.role === "user";
    const avatar = isUser
      ? avatarValue(conversation.user_avatar, defaultUserAvatar)
      : avatarValue(conversation.assistant_avatar, defaultAssistantAvatar);
    const stream = activeStream();
    const isLatestAssistant = !isUser && index === state.messages.length - 1;
    const isStreamingMessage = isLatestAssistant && (Boolean(stream) || message.status === "generating");
    const events = isStreamingMessage ? stream.processEvents : messageEvents(message);
    const content = message.content || "";
    const body = !isUser && !content && isStreamingMessage
      ? `<div class="chat-loading-dots" aria-label="${esc(t("responding"))}"><span></span><span></span><span></span></div>`
      : `<div class="markdown-body">${renderMarkdown(content, {
        codeLabel: t("code"),
        codeViewsLabel: t("code_views"),
        copyLabel: t("copy_code"),
        htmlPreviewLabel: t("html_preview"),
        previewLabel: t("preview"),
        reloadPreviewLabel: t("reload_preview"),
        tableLabel: t("markdown_table"),
      })}${renderSources(message, t("sources"))}</div>`;
    const process = !isUser && el.processToggle.checked ? renderProcessDetails(events, Boolean(isStreamingMessage)) : "";
    const displayStatus = message._ui_status || (["failed", "stopped"].includes(message.status) ? message.status : "");
    const statusMessage = message._ui_message || messageFailureMessage(message) || (displayStatus ? t(displayStatus) : "");
    const status = !isUser && displayStatus
      ? `<div class="chat-stream-state ${displayStatus === "failed" ? "chat-text-error" : ""}" role="status" aria-live="polite">${esc(statusMessage)}</div>`
      : "";
    const canRegenerate = isLatestAssistant && message.id && ["completed", "failed", "stopped"].includes(message.status || "completed") && !isConversationBusyById(state.activeId);
    const actionButtons = [
      content ? `<button type="button" class="chat-message-action" data-copy-message="${index}" title="${esc(t("copy_response"))}" aria-label="${esc(t("copy_response"))}"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-copy"></use></svg></button>` : "",
      canRegenerate ? `<button type="button" class="chat-message-action" data-regenerate-message="${index}" title="${esc(t("regenerate"))}" aria-label="${esc(t("regenerate"))}"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-refresh"></use></svg></button>` : "",
    ].filter(Boolean).join("");
    const usage = renderUsage(message);
    const actionContent = `${actionButtons ? `<div class="chat-message-commands">${actionButtons}</div>` : ""}${usage}`;
    const actions = !isUser && actionContent ? `<div class="chat-message-actions">${actionContent}</div>` : "";
    return `<article class="chat-message ${isUser ? "user" : "assistant"}" data-message-index="${index}"><div class="chat-message-avatar" aria-hidden="true">${esc(avatar)}</div><div class="chat-message-stack">${body}${process}${status}${actions}</div></article>`;
  }

  function messageFailureMessage(message) {
    if (!message?.metadata) return "";
    try {
      const metadata = typeof message.metadata === "string" ? JSON.parse(message.metadata) : message.metadata;
      return metadata?.error_code ? t(metadata.error_code) : localizeError(metadata?.error || "");
    } catch {
      return "";
    }
  }

  function renderUsage(message) {
    const usage = usageFromMessage(message);
    if (!usage) return "";
	const estimated = messageUsageEstimated(message);
	const inputLabel = t("input_tokens");
	const outputLabel = t("output_tokens");
	const inputExact = numberFormat.format(usage.input_tokens);
	const outputExact = numberFormat.format(usage.output_tokens);
	const detailIcons = { input_tokens: "icon-arrow-up", output_tokens: "icon-arrow-down" };
	const rows = tokenUsageDetailKeys(usage).map((key) => usageDetailRow(t(key), usage[key], detailIcons[key] || ""));
	rows.push(`<div class="chat-usage-detail chat-usage-measurement"><dt>${esc(t("measurement"))}</dt><dd>${esc(t(estimated ? "estimated" : "exact"))}</dd></div>`);
	return `<details class="chat-message-usage"><summary title="${esc(t("token_usage"))}"><svg class="icon chat-usage-chevron" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-right"></use></svg><span class="chat-usage-total"><span>Token</span><strong>${esc(formatCompactNumber(usage.total_tokens))}</strong></span>${usageSummaryMetric("icon-arrow-up", inputLabel, inputExact, usage.input_tokens)}${usageSummaryMetric("icon-arrow-down", outputLabel, outputExact, usage.output_tokens)}${estimated ? `<span class="chat-usage-estimated">${esc(t("estimated"))}</span>` : ""}</summary><dl class="chat-usage-details">${rows.join("")}</dl></details>`;
  }

  function usageSummaryMetric(icon, label, exact, value) {
	return `<span class="chat-usage-metric" title="${esc(`${label} ${exact}`)}" aria-label="${esc(`${label} ${exact}`)}"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#${icon}"></use></svg><span class="chat-sr-only">${esc(label)}</span><strong>${esc(formatCompactNumber(value))}</strong></span>`;
  }

  function usageDetailRow(label, value, icon = "") {
	const iconHTML = icon ? `<svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#${icon}"></use></svg>` : "";
	return `<div class="chat-usage-detail"><dt>${iconHTML}<span>${esc(label)}</span></dt><dd>${esc(numberFormat.format(value))}</dd></div>`;
  }

  function renderProcessDetails(events, streaming) {
    if (!events.length && !streaming) return "";
    const summary = events.length ? t("process_count", { count: events.length }) : t("process");
    return `<details class="chat-process-details" ${streaming ? "open" : ""}><summary><svg class="icon chat-details-chevron" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-right"></use></svg><span>${esc(summary)}</span></summary><div class="chat-process-events">${events.length ? events.map(renderProcessEvent).join("") : `<div class="chat-event"><span>${esc(t("starting"))}</span></div>`}</div></details>`;
  }

  function renderProcessEvent(event) {
    const type = event.type || "status";
    if (type === "tool_start") {
      return `<div class="chat-event"><strong>${esc(event.name || t("tools"))}</strong><span>${esc(t("started"))}</span>${event.arguments ? `<code>${esc(event.arguments)}</code>` : ""}</div>`;
    }
    if (type === "tool_result") {
      return `<div class="chat-event ${event.ok ? "" : "warn"}"><strong>${esc(event.name || t("tools"))}</strong><span>${esc(event.ok ? t("completed") : t("failed"))}</span>${event.result ? `<pre>${esc(compact(event.result))}</pre>` : ""}</div>`;
    }
    if (type === "thinking") {
      return `<div class="chat-event"><strong>${esc(t("thinking"))}</strong><pre>${esc(event.content || "")}</pre></div>`;
    }
    if (type === "warning") {
      return `<div class="chat-event warn"><strong>${esc(t("warning"))}</strong><span>${esc(event.message || "")}</span></div>`;
    }
    if (type === "retry") {
      return `<div class="chat-event warn"><strong>${esc(t("status"))}</strong><span>${esc(t("retrying", { attempt: event.attempt || 1, max: event.max_attempts || 2 }))}</span></div>`;
    }
    const message = event.message_key ? t(event.message_key) : (event.message || type);
    return `<div class="chat-event"><strong>${esc(t("status"))}</strong><span>${esc(message)}</span></div>`;
  }

  function scheduleStreamingRender(conversationId) {
    if (state.activeId !== conversationId || state.renderFrame) return;
    state.renderFrame = requestAnimationFrame(() => {
      state.renderFrame = 0;
      if (state.activeId !== conversationId) return;
      state.messages = conversationMessages(conversationId);
      const index = state.messages.length - 1;
      const message = state.messages[index];
      const current = el.messages.querySelector(`[data-message-index="${index}"]`);
      if (!message || !current) {
        renderMessages();
        return;
      }
      const follow = isNearBottom(el.messages);
      const template = document.createElement("template");
      template.innerHTML = renderMessage(message, index).trim();
      current.replaceWith(template.content.firstElementChild);
      if (follow) scrollMessagesToBottom(false);
      updateScrollButton();
      syncDisabled();
    });
  }

  function scrollMessagesToBottom(smooth = true) {
    el.messages.scrollTo({ top: el.messages.scrollHeight, behavior: smooth ? "smooth" : "auto" });
  }

  function updateScrollButton() {
    el.scrollBottom.classList.toggle("hidden", !state.messages.length || isNearBottom(el.messages));
  }

  async function sendMessage(event) {
    event.preventDefault();
    const content = el.input.value.trim();
    if (!content || !state.models.length) return;
	if (Array.from(content).length > state.maxUserMessageChars) {
	  showToast(t("message_too_large"), "error");
	  return;
	}
    if (state.activeId && isConversationBusyById(state.activeId)) {
      showToast(t("conversation_busy"), "error");
      return;
    }

    let conversationId;
    try {
      conversationId = state.activeId || await createConversationFromDraft();
    } catch (error) {
      showToast(error.message, "error");
      return;
    }
    if (isConversationBusyById(conversationId)) return;
    cancelScheduledAutoTitle(conversationId);

    const abort = new AbortController();
    const processEvents = [{ type: "status", message: t("starting") }];
    const requestId = createRequestID();
    const messages = [...conversationMessages(conversationId), { role: "user", content, status: "completed" }, { role: "assistant", content: "", status: "generating", request_id: requestId }];
    const stream = { abort, draft: "", messages, processEvents, responseDone: false, requestId, hasDelta: false, serverError: false, stopRequested: false };
    state.streamsByConversationId.set(conversationId, stream);
    setConversationMessages(conversationId, messages);
    markConversationStatus(conversationId, "responding", "responding");
    el.input.value = "";
    autosizeInput();
    renderMessages({ forceBottom: true });
    syncDisabled();
    await runGenerationStream(conversationId, stream, `/conversations/${encodeURIComponent(conversationId)}/messages`, {
      content,
      request_id: requestId,
      enable_search: el.search.checked,
      enable_read: el.read.checked,
	  time_zone: browserTimeZone,
    });
  }

  async function regenerateMessage(index) {
    const conversationId = state.activeId;
    const messages = conversationMessages(conversationId);
    const message = messages[index];
    if (!conversationId || index !== messages.length - 1 || message?.role !== "assistant" || !message.id || isConversationBusyById(conversationId)) {
      showToast(t("message_not_latest"), "error");
      return;
    }
    cancelScheduledAutoTitle(conversationId);
    const abort = new AbortController();
    const requestId = createRequestID();
    message.content = "";
    message.metadata = "{}";
    message.status = "generating";
    message.request_id = requestId;
    delete message._ui_status;
    delete message._ui_message;
    const stream = { abort, draft: "", messages, processEvents: [{ type: "status", message: t("starting") }], responseDone: false, requestId, hasDelta: false, serverError: false, stopRequested: false };
    state.streamsByConversationId.set(conversationId, stream);
    setConversationMessages(conversationId, messages);
    markConversationStatus(conversationId, "responding", "responding");
    renderMessages({ forceBottom: true });
    syncDisabled();
    await runGenerationStream(conversationId, stream, `/conversations/${encodeURIComponent(conversationId)}/messages/${encodeURIComponent(message.id)}/regenerate`, {
      request_id: requestId,
      enable_search: el.search.checked,
      enable_read: el.read.checked,
      time_zone: browserTimeZone,
    });
  }

  async function runGenerationStream(conversationId, stream, path, payload) {
    try {
      for (let attempt = 0; ; attempt += 1) {
        try {
          const response = await fetch(`${apiPrefix}${path}`, {
            method: "POST",
            headers: { "Content-Type": "application/json", "X-CSRF-Token": cookie(csrfCookie) },
            signal: stream.abort.signal,
            body: JSON.stringify(payload),
          });
          if (!response.ok) {
            const body = await response.json().catch(() => ({}));
            throw chatAPIError(body);
          }
          await readSSE(response, (streamEvent, data) => handleStreamEvent(conversationId, streamEvent, data));
          if (stream.retryableServerError) {
            const retryError = new Error(stream.retryableServerError.message || t("conversation_busy"));
            retryError.retryable = true;
            stream.retryableServerError = null;
            throw retryError;
          }
          break;
        } catch (error) {
          if (!canRetryStream({ attempt, hasDelta: stream.hasDelta, stopRequested: stream.stopRequested, serverError: stream.serverError, error })) throw error;
          const delay = CLIENT_RETRY_DELAYS_MS[attempt];
          handleStreamEvent(conversationId, "retry", { attempt: attempt + 1, max_attempts: CLIENT_RETRY_DELAYS_MS.length, delay_ms: delay, message_key: "retrying" });
          await waitForDelay(delay, stream.abort.signal);
        }
      }
      await refreshConversations({ silent: true });
    } catch (error) {
      const currentMessages = conversationMessages(conversationId);
      const last = currentMessages[currentMessages.length - 1];
      if (last?.role === "assistant") {
        if (error.name === "AbortError" || stream.stopRequested) {
          last._ui_status = "stopped";
          last._ui_message = t("stopped");
        } else {
          last._ui_status = "failed";
          last._ui_message = error.message === "Invalid streaming response" ? t("stream_invalid") : error.message;
          showToast(last._ui_message, "error");
        }
        setConversationMessages(conversationId, currentMessages);
      }
    } finally {
      if (state.streamsByConversationId.get(conversationId) === stream) state.streamsByConversationId.delete(conversationId);
      await refreshConversations({ silent: true }).catch(() => {});
	  await reloadConversationAfterStream(conversationId).catch(() => {});
      if (state.activeId === conversationId) {
        state.messages = conversationMessages(conversationId);
        renderMessages();
      }
      syncDisabled();
      if (stream.responseDone && stream.autoTitle) scheduleConversationTitle(conversationId);
    }
  }

  async function stopGeneration() {
    const conversationId = state.activeId;
    if (!conversationId) return;
    const stream = activeStream();
    if (stream) {
      stream.stopRequested = true;
      stream.stopping = true;
      stream.abort.abort();
    }
    syncDisabled();
    try {
      await api(`/conversations/${encodeURIComponent(conversationId)}/stop`, { method: "POST", body: "{}" });
    } catch (error) {
      if (!stream) showToast(error.message, "error");
    } finally {
      await refreshConversations({ silent: true }).catch(() => {});
      await reloadConversationAfterStream(conversationId).catch(() => {});
      if (state.activeId === conversationId) renderMessages();
      syncDisabled();
    }
  }

  function handleStreamEvent(conversationId, event, data) {
    const stream = state.streamsByConversationId.get(conversationId);
    if (event === "delta") {
      const messages = conversationMessages(conversationId);
      if (stream) {
        stream.draft += data.content || "";
        if (data.content) stream.hasDelta = true;
      }
      const last = messages[messages.length - 1];
      if (last?.role === "assistant") {
        last.content = stream?.draft || `${last.content || ""}${data.content || ""}`;
        delete last._ui_status;
        delete last._ui_message;
      }
      setConversationMessages(conversationId, messages);
      scheduleStreamingRender(conversationId);
      return;
    }
    if (event === "assistant_message") {
      const messages = conversationMessages(conversationId);
      const last = messages[messages.length - 1];
      if (last?.role === "assistant") Object.assign(last, data);
      const persistedEvents = messageEvents(last);
      if (stream && persistedEvents.length) stream.processEvents = persistedEvents;
      setConversationMessages(conversationId, messages);
      scheduleStreamingRender(conversationId);
      return;
    }
    if (event === "done") {
      if (stream) stream.responseDone = true;
	  if (stream) stream.autoTitle = Boolean(data.auto_title);
      const messages = conversationMessages(conversationId);
      const last = messages[messages.length - 1];
      if (last?.role === "assistant" && data.usage) {
        mergeMessageUsage(last, data.usage);
        setConversationMessages(conversationId, messages);
      }
      scheduleStreamingRender(conversationId);
      return;
    }
    if (event === "retry") {
      const messages = conversationMessages(conversationId);
      const last = messages[messages.length - 1];
      if (last?.role === "assistant") {
        last._ui_status = "retrying";
        last._ui_message = t("retrying", { attempt: data.attempt || 1, max: data.max_attempts || 2 });
        setConversationMessages(conversationId, messages);
      }
    }
    if (["tool_start", "tool_result", "thinking", "warning", "status", "retry"].includes(event)) {
      if (stream) stream.processEvents = [...stream.processEvents, { ...data, type: data.type || event }];
      scheduleStreamingRender(conversationId);
      return;
    }
    if (event === "conversation") {
      upsertConversation(data);
      if (state.activeId === data.id) {
        state.activeConversation = data;
        populateSettings(data);
        renderHeader();
      }
      renderConversations();
      return;
    }
    if (event === "error") {
      if (stream) {
        stream.serverError = data.retryable !== true;
        if (data.retryable === true) stream.retryableServerError = data;
      }
      const messages = conversationMessages(conversationId);
      const last = messages[messages.length - 1];
      if (last?.role === "assistant") {
        last.status = "failed";
        last._ui_status = "failed";
		last._ui_message = data.code ? t(data.code) : localizeError(data.message || t("request_failed"));
        setConversationMessages(conversationId, messages);
      }
      if (state.activeId === conversationId && data.retryable !== true) showToast(last?._ui_message || t("request_failed"), "error");
      scheduleStreamingRender(conversationId);
    }
  }

  async function generateConversationTitle(id = state.activeId, force = true, quiet = false) {
    if (force) cancelScheduledAutoTitle(id);
    if (!id || isConversationBusyById(id)) {
      showToast(id ? t("conversation_busy") : t("title_no_messages"), "error");
      return;
    }
    state.titleGeneratingIds.add(id);
    markConversationStatus(id, "title_generating", "title_generating");
    syncDisabled();
    try {
	  const conv = await api(`/conversations/${encodeURIComponent(id)}/title`, { method: "POST", body: JSON.stringify({ force }) });
      upsertConversation(conv);
      if (state.activeId === conv.id) {
        state.activeConversation = conv;
        populateSettings(conv);
        renderHeader();
      }
      renderConversations();
	  if (!quiet) showToast(t("title_generated"));
    } catch (error) {
	  if (!quiet) showToast(error.message || t("request_failed"), "error");
    } finally {
      state.titleGeneratingIds.delete(id);
      await refreshConversations({ silent: true }).catch(() => {});
      syncDisabled();
    }
  }

  async function renameConversation(id = state.activeId) {
    if (!id || isConversationBusyById(id)) return;
    const current = state.conversations.find((conv) => conv.id === id);
    const title = await requestText({
      title: t("rename_title"),
      label: t("conversation_name"),
      value: current?.title || t("new_chat"),
    });
    if (!title) return;
    cancelScheduledAutoTitle(id);
    const conv = await api(`/conversations/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ title }),
    });
    state.conversations = state.conversations.map((item) => item.id === conv.id ? conv : item);
    if (state.activeId === conv.id) {
      state.activeConversation = conv;
      populateSettings(conv);
      renderHeader();
    }
    renderConversations();
  }

  async function deleteConversation(id = state.activeId) {
    if (!id || isConversationBusyById(id)) {
      if (id) showToast(t("conversation_busy"), "error");
      return;
    }
    const confirmed = await confirmAction({
      title: t("delete_title"),
      message: t("delete_confirm"),
      cancelLabel: t("cancel"),
      confirmLabel: t("delete"),
      tone: "danger",
    });
    if (!confirmed) return;
    cancelScheduledAutoTitle(id);
    await api(`/conversations/${encodeURIComponent(id)}`, { method: "DELETE" });
    const deletingActive = id === state.activeId;
    state.conversations = state.conversations.filter((conv) => conv.id !== id);
    state.messagesByConversationId.delete(id);
    state.streamsByConversationId.delete(id);
    state.titleGeneratingIds.delete(id);
    if (deletingActive) {
      if (state.conversations.length) await openConversation(state.conversations[0].id, { forceBottom: true });
      else beginDraft({ inherit: false });
      return;
    }
    renderConversations();
  }

  function scheduleConversationTitle(id) {
    cancelScheduledAutoTitle(id);
    const timer = scheduleAutoTitle(async () => {
      if (state.autoTitleTimersByConversationId.get(id) !== timer) return;
      state.autoTitleTimersByConversationId.delete(id);
      if (!state.conversations.some((conversation) => conversation.id === id)) return;
      await generateConversationTitle(id, false, true);
    });
    state.autoTitleTimersByConversationId.set(id, timer);
  }

  function cancelScheduledAutoTitle(id) {
    if (!id) return;
    const timer = state.autoTitleTimersByConversationId.get(id);
    if (timer === undefined) return;
    clearTimeout(timer);
    state.autoTitleTimersByConversationId.delete(id);
  }

  async function openSettings(trigger) {
    populateSettings(currentConfig());
    syncDisabled();
    state.settingsPreviousFocus = trigger || document.activeElement;
    state.settingsDirty = false;
    el.settingsModal.classList.remove("hidden");
    el.settingsModal.setAttribute("aria-hidden", "false");
    requestAnimationFrame(() => el.model.focus());
  }

  function closeSettings() {
    if (el.settingsModal.classList.contains("hidden")) return;
    el.settingsModal.classList.add("hidden");
    el.settingsModal.setAttribute("aria-hidden", "true");
    populateSettings(currentConfig());
    state.settingsPreviousFocus?.focus?.();
    state.settingsPreviousFocus = null;
  }

  async function saveSettings(event) {
    event.preventDefault();
    if (state.activeId && isConversationBusyById(state.activeId)) {
      showToast(t("conversation_busy"), "error");
      return;
    }
    const values = draftFromForm();
    if (state.activeId) {
      const conv = await api(`/conversations/${encodeURIComponent(state.activeId)}`, {
        method: "PATCH",
        body: JSON.stringify(values),
      });
      state.activeConversation = conv;
      state.conversations = state.conversations.map((item) => item.id === conv.id ? conv : item);
    } else {
      state.draft = { ...(state.draft || {}), ...values };
    }
    populateSettings(currentConfig());
    renderHeader();
    renderConversations();
    renderMessages();
    closeSettings();
    showToast(t("settings_saved"));
  }

  function draftFromForm() {
    return {
      model: el.model.value,
      thinking_effort: thinkingEffort(root),
      system_prompt: el.systemPrompt?.value || "",
      nickname: el.nickname?.value || "",
      user_avatar: el.userAvatar?.value || defaultUserAvatar,
      assistant_avatar: el.assistantAvatar?.value || defaultAssistantAvatar,
      max_tool_calls: normalizeMaxToolCalls(el.maxToolCalls?.value),
    };
  }

  function populateSettings(conversation) {
    const conv = conversation || {};
    const model = conv.model || el.model.value || state.models[0] || "";
    if (model) el.model.value = model;
    setThinking(root, conv.thinking_effort || thinkingEffort(root) || "medium");
    if (el.defaultSystemPrompt) el.defaultSystemPrompt.textContent = state.defaultSystemPrompt || t("default_system_prompt_empty");
    if (el.systemPrompt) el.systemPrompt.value = conv.system_prompt || "";
    if (el.nickname) el.nickname.value = conv.nickname || "";
    if (el.userAvatar) el.userAvatar.value = avatarValue(conv.user_avatar, defaultUserAvatar);
    if (el.assistantAvatar) el.assistantAvatar.value = avatarValue(conv.assistant_avatar, defaultAssistantAvatar);
    if (el.maxToolCalls) {
      el.maxToolCalls.value = String(normalizeMaxToolCalls(conv.max_tool_calls ?? state.defaultMaxToolCalls));
    }
    renderSettingsSummary(conv);
  }

  function renderSettingsSummary(conversation = currentConfig()) {
    const model = conversation?.model || el.model.value || state.models[0] || t("no_model");
    const effort = labelEffort(conversation?.thinking_effort || thinkingEffort(root), t);
    el.settingsSummary.textContent = `${model} · ${effort}`;
  }

  function syncDisabled() {
    const busy = Boolean(state.activeId && isConversationBusyById(state.activeId));
    for (const item of [el.model, el.systemPrompt, el.nickname, el.userAvatar, el.assistantAvatar, el.settingsSave]) {
      if (item) item.disabled = busy;
    }
    if (el.maxToolCalls) {
      el.maxToolCalls.disabled = busy;
    }
    root.querySelectorAll("[data-chat-avatar-value], input[name$='-thinking']").forEach((item) => { item.disabled = busy; });
    const stream = activeStream();
    const responding = Boolean(stream) || state.activeConversation?.active_operation === "responding";
    el.stop.classList.toggle("hidden", !responding);
    el.stop.disabled = Boolean(stream?.stopping);
    setTitle(el.stop, stream?.stopping ? t("stopping") : t("stop"));
    el.send.classList.toggle("hidden", responding);
	const inputLength = Array.from(el.input.value.trim()).length;
	el.send.disabled = busy || Boolean(state.draftCreating) || !state.models.length || inputLength === 0 || inputLength > state.maxUserMessageChars;
    el.autoTitle.disabled = busy || !state.activeId || !hasConversationMessages(state.activeId);
    el.rename.disabled = busy || !state.activeId;
    el.delete.disabled = busy || !state.activeId;
    el.conversationMenuToggle.disabled = !state.activeId;
  }

  async function refreshConversations(options = {}) {
    try {
      const body = await api("/conversations");
      state.conversations = body.items || [];
      if (state.activeId) {
        state.activeConversation = state.conversations.find((conv) => conv.id === state.activeId) || state.activeConversation;
        if (el.settingsModal.classList.contains("hidden") || !state.settingsDirty) populateSettings(state.activeConversation);
        renderHeader();
      }
      renderConversations();
      syncDisabled();
    } catch (error) {
      if (!options.silent) throw error;
    }
  }

  function startConversationPolling() {
    if (state.pollTimer) return;
    state.pollTimer = window.setInterval(() => {
      if (!document.hidden) refreshConversations({ silent: true });
    }, 5000);
    document.addEventListener("visibilitychange", () => {
      if (!document.hidden) refreshConversations({ silent: true });
    });
  }

  function currentConfig() {
    return state.activeConversation || state.draft || {};
  }

  function activeStream() {
    return state.activeId ? state.streamsByConversationId.get(state.activeId) || null : null;
  }

  function conversationMessages(id) {
    return id ? (state.messagesByConversationId.get(id) || []) : state.messages;
  }

  function setConversationMessages(id, messages) {
    if (id) state.messagesByConversationId.set(id, messages || []);
    if (state.activeId === id) state.messages = messages || [];
  }

  function hasConversationMessages(id) {
    return conversationMessages(id).some((message) => message.role === "user");
  }

  function isConversationBusyById(id, conversation = null) {
    if (!id) return false;
    if (state.streamsByConversationId.has(id) || state.titleGeneratingIds.has(id)) return true;
    const conv = conversation || state.conversations.find((item) => item.id === id) || (state.activeId === id ? state.activeConversation : null);
    return conv?.active_operation === "responding" || conv?.active_operation === "title_generating";
  }

  function conversationStatus(conversation) {
    const id = conversation?.id;
    if (state.streamsByConversationId.has(id)) return "responding";
    if (state.titleGeneratingIds.has(id)) return "title_generating";
    if (["responding", "title_generating"].includes(conversation?.active_operation)) return conversation.active_operation;
    return conversation?.status || "idle";
  }

  function conversationStatusLabel(status) {
    switch (status) {
      case "responding": return t("status_responding");
      case "title_generating": return t("status_title_generating");
      case "failed": return t("status_failed");
      case "stopped": return t("status_stopped");
      default: return "";
    }
  }

  function markConversationStatus(id, activeOperation, status) {
    if (!id) return;
    const patch = { active_operation: activeOperation || "", status: status || "idle" };
    state.conversations = state.conversations.map((conv) => conv.id === id ? { ...conv, ...patch } : conv);
    if (state.activeId === id) state.activeConversation = { ...(state.activeConversation || {}), ...patch };
    renderConversations();
  }

  function upsertConversation(conversation) {
    if (!conversation?.id) return;
    state.conversations = [conversation, ...state.conversations.filter((item) => item.id !== conversation.id)];
  }

  function displayConversationTitle(conversation) {
    const title = conversation?.title || "";
    if (conversation?.title_auto_generated && isDefaultConversationTitle(title)) return t("new_chat");
    if (conversation?.title_auto_generated) {
      const plain = title
        .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
        .replace(/[*_`#]+/g, "")
        .replace(/\s+/g, " ")
        .trim();
      return plain || t("new_chat");
    }
    return title || t("new_chat");
  }

  function isDefaultConversationTitle(title) {
    const value = String(title || "").trim();
    return !value || value === "New chat" || value === "新对话";
  }

  function localizeError(message) {
    switch (String(message || "")) {
      case "conversation is already processing": return t("conversation_busy");
      case "current conversation has no messages to summarize": return t("title_no_messages");
      case "only the latest assistant message can be regenerated": return t("message_not_latest");
      default: return message || t("request_failed");
    }
  }

  function chatAPIError(body = {}) {
	const error = new Error(body.code ? t(body.code) : localizeError(body.error || t("request_failed")));
	error.code = body.code || "";
	error.retryable = body.retryable === true;
	if (error.code === "model_unavailable") requestAnimationFrame(() => openSettings(el.settingsOpen[0]));
	return error;
  }

  async function reloadConversation(id) {
	if (!id) return;
	const body = await api(`/conversations/${encodeURIComponent(id)}`);
	upsertConversation(body.conversation);
	setConversationMessages(id, body.messages || []);
	if (state.activeId === id) {
	  state.activeConversation = body.conversation;
	  state.messages = conversationMessages(id);
	}
  }

  async function reloadConversationAfterStream(id) {
	for (let attempt = 0; attempt < 5; attempt += 1) {
	  await reloadConversation(id);
	  const conversation = state.conversations.find((item) => item.id === id);
	  if (!isConversationBusyById(id, conversation)) return;
	  await new Promise((resolve) => window.setTimeout(resolve, 200));
	}
  }

  function updateCharacterCount() {
	const count = Array.from(el.input.value || "").length;
	const remaining = Math.max(0, state.maxUserMessageChars - count);
	if (el.characterCount) {
	  el.characterCount.textContent = numberFormat.format(remaining);
	  el.characterCount.setAttribute("aria-label", t("input_characters_remaining", { count: numberFormat.format(remaining) }));
	  el.characterCount.classList.toggle("warning", remaining < 1024);
	}
  }

  function updateToolStatus() {
    const items = [
      { key: "search", label: t("search"), enabled: el.search.checked },
      { key: "read", label: t("read_web"), enabled: el.read.checked },
      { key: "process", label: t("process"), enabled: el.processToggle.checked },
    ];
    el.toolStatus.innerHTML = items.map((item) => `<button type="button" class="chat-tool-chip ${item.enabled ? "active" : ""}" data-chat-tool-chip="${item.key}" aria-pressed="${item.enabled}" title="${esc(item.label)}">${esc(item.label)}</button>`).join("");
    if (!state.loading) renderMessages();
  }

  function applySidebarState() {
    if (mobileMedia.matches) {
      root.classList.remove("sidebar-collapsed", "sidebar-open");
      el.sidebarToggle.setAttribute("aria-expanded", "false");
      return;
    }
    root.classList.toggle("sidebar-collapsed", state.sidebarCollapsed);
    root.classList.remove("sidebar-open");
    el.sidebarToggle.setAttribute("aria-expanded", String(!state.sidebarCollapsed));
  }

  function toggleSidebar() {
    if (mobileMedia.matches) {
      const open = !root.classList.contains("sidebar-open");
      root.classList.toggle("sidebar-open", open);
      el.sidebarToggle.setAttribute("aria-expanded", String(open));
      if (open) requestAnimationFrame(() => el.conversationSearch.focus());
      return;
    }
    state.sidebarCollapsed = !state.sidebarCollapsed;
    try { localStorage.setItem(desktopSidebarKey, String(state.sidebarCollapsed)); } catch {}
    applySidebarState();
  }

  function closeMobileSidebar() {
    if (!mobileMedia.matches) return;
    root.classList.remove("sidebar-open");
    el.sidebarToggle.setAttribute("aria-expanded", "false");
  }

  function readSidebarPreference() {
    try { return localStorage.getItem(desktopSidebarKey) === "true"; } catch { return false; }
  }

  function toggleMenu(toggle, menu) {
    const open = menu.classList.contains("hidden");
    closeAllMenus(menu);
    menu.classList.toggle("hidden", !open);
    toggle.setAttribute("aria-expanded", String(open));
  }

  function closeAllMenus(except = null) {
    for (const [toggle, menu] of [[el.accountToggle, el.accountMenu], [el.toolsToggle, el.toolsMenu], [el.conversationMenuToggle, el.conversationMenu]]) {
      if (menu === except) continue;
      menu.classList.add("hidden");
      toggle.setAttribute("aria-expanded", "false");
    }
    if (!except) closeFloatingMenu();
  }

  function openFloatingMenu(button, id) {
    closeFloatingMenu();
    const conversation = state.conversations.find((item) => item.id === id);
    const busy = isConversationBusyById(id, conversation);
    const menu = document.createElement("div");
    menu.className = "chat-popover chat-floating-menu";
    menu.dataset.chatFloatingMenu = String(id);
    menu.innerHTML = `<button type="button" data-floating-action="title" ${busy ? "disabled" : ""}><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-refresh"></use></svg><span>${esc(t("generate_title"))}</span></button><button type="button" data-floating-action="rename" ${busy ? "disabled" : ""}><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-edit"></use></svg><span>${esc(t("rename"))}</span></button><button type="button" class="danger-text" data-floating-action="delete" ${busy ? "disabled" : ""}><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-trash"></use></svg><span>${esc(t("delete"))}</span></button>`;
    document.body.appendChild(menu);
    const rect = button.getBoundingClientRect();
    const width = 210;
    menu.style.left = `${Math.max(8, Math.min(window.innerWidth - width - 8, rect.right - width))}px`;
    menu.style.top = `${Math.min(window.innerHeight - menu.offsetHeight - 8, rect.bottom + 5)}px`;
    menu.addEventListener("click", (event) => {
      const action = event.target.closest("[data-floating-action]")?.dataset.floatingAction;
      if (!action) return;
      closeFloatingMenu();
      if (action === "title") generateConversationTitle(id);
      if (action === "rename") renameConversation(id).catch((error) => showToast(error.message, "error"));
      if (action === "delete") deleteConversation(id).catch((error) => showToast(error.message, "error"));
    });
  }

  function closeFloatingMenu() {
    document.querySelector("[data-chat-floating-menu]")?.remove();
  }

  function requestText(options) {
    const modal = ensureTextModal();
    const input = modal.querySelector("input");
    modal.querySelector("h2").textContent = options.title;
    modal.querySelector("label").firstChild.textContent = options.label;
    modal.querySelector("[data-chat-text-cancel]").textContent = t("cancel");
    modal.querySelector("[data-chat-text-save]").textContent = t("save");
    input.value = options.value || "";
    modal.classList.remove("hidden");
    modal.removeAttribute("aria-hidden");
    const previous = document.activeElement;
    requestAnimationFrame(() => { input.focus(); input.select(); });
    return new Promise((resolve) => {
      const finish = (value) => {
        modal.classList.add("hidden");
        modal.setAttribute("aria-hidden", "true");
        modal._finish = null;
        previous?.focus?.();
        resolve(value);
      };
      modal._finish = finish;
    });
  }

  function ensureTextModal() {
    let modal = document.querySelector("[data-chat-text-modal]");
    if (modal) return modal;
    modal = document.createElement("div");
    modal.className = "chat-text-modal hidden";
    modal.dataset.chatTextModal = "";
    modal.setAttribute("aria-hidden", "true");
    modal.innerHTML = `<button type="button" class="chat-text-backdrop" data-chat-text-cancel aria-label="${esc(t("close"))}"></button><form class="chat-text-dialog" role="dialog" aria-modal="true" aria-labelledby="chat-text-title"><div class="chat-text-head"><h2 id="chat-text-title"></h2><button type="button" class="chat-icon-button" data-chat-text-cancel title="${esc(t("close"))}" aria-label="${esc(t("close"))}"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-close"></use></svg></button></div><div class="chat-text-body"><label>${esc(t("conversation_name"))}<input maxlength="120" required></label></div><div class="chat-text-actions"><button type="button" class="secondary" data-chat-text-cancel>${esc(t("cancel"))}</button><button type="submit" data-chat-text-save>${esc(t("save"))}</button></div></form>`;
    document.body.appendChild(modal);
    modal.addEventListener("click", (event) => {
      if (event.target.closest("[data-chat-text-cancel]")) modal._finish?.(null);
    });
    modal.querySelector("form").addEventListener("submit", (event) => {
      event.preventDefault();
      const value = modal.querySelector("input").value.trim();
      if (value) modal._finish?.(value);
    });
    modal.addEventListener("keydown", (event) => {
      if (event.key === "Escape") { event.preventDefault(); modal._finish?.(null); }
      else trapFocus(event, modal.querySelector("form"));
    });
    return modal;
  }

  function localizeStatic() {
    el.sidebar.setAttribute("aria-label", t("conversations"));
    setTitle(el.sidebarBackdrop, t("close_conversations"));
    setTitle(el.sidebarToggle, t("show_conversations"));
    setTitle(el.sidebarCollapse, t("collapse_sidebar"));
    setTitle(el.new, t("new_chat"));
    el.conversationSearch.placeholder = t("search_conversations");
    setTitle(el.conversationMenuToggle, t("conversation_actions"));
    setTitle(el.toolsToggle, t("tools"));
    setCheckboxLabel(el.search, t("search"));
    setCheckboxLabel(el.read, t("read_web"));
    setCheckboxLabel(el.processToggle, t("process"));
	setTitle(el.processToggle, t("process_saved_hint"));
    for (const button of el.settingsOpen) setTitle(button, t("chat_settings"));
    setButtonContent(el.autoTitle, t("generate_title"));
    setButtonContent(el.rename, t("rename"));
    setButtonContent(el.delete, t("delete"));
    setTitle(el.scrollBottom, t("scroll_bottom"));
    el.input.placeholder = t("ask_placeholder");
    setTitle(el.stop, t("stop"));
    setTitle(el.send, t("send"));
    root.querySelector(".chat-settings-head h2").textContent = t("chat_settings");
    root.querySelector(".chat-settings-head p").textContent = t("saved_to_conversation");
    setTitle(root.querySelector(".chat-settings-head [data-chat-settings-close]"), t("close_settings"));
    setText(root.querySelector("[data-chat-model-section-title]"), t("model_and_reasoning"));
    setText(root.querySelector("[data-chat-instructions-title]"), t("instructions"));
    setText(root.querySelector("[data-chat-identity-title]"), t("identity"));
    setLeadingText(root.querySelector(".chat-settings-dialog .chat-model-field"), t("model"));
    setText(root.querySelector(".chat-settings-thinking legend"), t("thinking"));
    root.querySelectorAll(".chat-settings-thinking label").forEach((label) => {
      const value = label.querySelector("input")?.value || "medium";
      setTrailingText(label, labelEffort(value, t));
    });
    setLeadingText(el.systemPrompt.closest("label"), t("system_prompt"));
    el.systemPrompt.placeholder = t("system_prompt_placeholder");
    setLeadingText(el.nickname.closest("label"), t("my_nickname"));
    el.nickname.placeholder = "";
    setLeadingText(el.maxToolCalls.closest("label"), t("max_tool_calls"));
    setText(root.querySelector("[data-chat-default-system-title]"), t("default_system_prompt"));
    setText(root.querySelector("[data-chat-default-system-hint]"), t("default_system_prompt_hint"));
    root.querySelector(".chat-avatar-settings")?.setAttribute("aria-label", t("avatar_settings"));
    setText(el.userAvatar.previousElementSibling, t("user_avatar"));
    setText(el.assistantAvatar.previousElementSibling, t("assistant_avatar"));
    el.userAvatar.setAttribute("aria-label", t("user_avatar"));
    el.assistantAvatar.setAttribute("aria-label", t("assistant_avatar"));
    renderAvatarPresets(el.userAvatar.closest(".chat-avatar-field")?.querySelector(".chat-avatar-picker"), "user", userAvatarPresets, t("user_avatar_presets"));
    renderAvatarPresets(el.assistantAvatar.closest(".chat-avatar-field")?.querySelector(".chat-avatar-picker"), "assistant", assistantAvatarPresets, t("assistant_avatar_presets"));
    el.settingsCancel.textContent = t("cancel");
    el.settingsSave.textContent = t("save");
    const accountName = root.querySelector(".chat-account-name")?.textContent.trim() || "T";
    if (el.accountAvatar) el.accountAvatar.textContent = Array.from(accountName)[0]?.toUpperCase() || "T";
  }

  function htmlPreviewOptions(focusTab = false) {
    return {
      focusTab,
      previewTitle: t("html_preview"),
      onNavigationBlocked: () => showToast(t("preview_navigation_blocked"), "error"),
    };
  }

  function bindEvents() {
    el.list.addEventListener("click", (event) => {
      const menuButton = event.target.closest("[data-chat-row-menu]");
      if (menuButton) {
        event.stopPropagation();
        openFloatingMenu(menuButton, Number(menuButton.dataset.chatRowMenu));
        return;
      }
      const button = event.target.closest("[data-chat-open]");
      if (button) openConversation(Number(button.dataset.chatOpen), { forceBottom: true }).catch((error) => showToast(error.message, "error"));
    });
    el.conversationSearch.addEventListener("input", renderConversations);
    el.new.addEventListener("click", () => beginDraft({ inherit: true }));
    el.sidebarToggle.addEventListener("click", toggleSidebar);
    el.sidebarCollapse.addEventListener("click", toggleSidebar);
    el.sidebarBackdrop.addEventListener("click", closeMobileSidebar);
    mobileMedia.addEventListener("change", applySidebarState);
    el.accountToggle.addEventListener("click", (event) => { event.stopPropagation(); toggleMenu(el.accountToggle, el.accountMenu); });
    el.toolsToggle.addEventListener("click", (event) => { event.stopPropagation(); toggleMenu(el.toolsToggle, el.toolsMenu); });
    el.conversationMenuToggle.addEventListener("click", (event) => { event.stopPropagation(); toggleMenu(el.conversationMenuToggle, el.conversationMenu); });
    el.autoTitle.addEventListener("click", () => { closeAllMenus(); generateConversationTitle(); });
    el.rename.addEventListener("click", () => { closeAllMenus(); renameConversation().catch((error) => showToast(error.message, "error")); });
    el.delete.addEventListener("click", () => { closeAllMenus(); deleteConversation().catch((error) => showToast(error.message, "error")); });
    el.settingsOpen.forEach((button) => button.addEventListener("click", () => openSettings(button)));
    el.settingsForm.addEventListener("submit", (event) => saveSettings(event).catch((error) => showToast(error.message, "error")));
    el.settingsForm.addEventListener("input", () => { state.settingsDirty = true; });
    el.settingsCancel.addEventListener("click", closeSettings);
    el.settingsModal.addEventListener("click", (event) => {
      if (event.target.closest("[data-chat-settings-close]")) closeSettings();
    });
    el.settingsModal.addEventListener("keydown", (event) => trapFocus(event, el.settingsForm));
    el.form.addEventListener("submit", sendMessage);
    el.stop.addEventListener("click", stopGeneration);
	  el.input.addEventListener("input", () => { autosizeInput(); updateCharacterCount(); syncDisabled(); });
    el.input.addEventListener("keydown", (event) => {
      if (event.key !== "Enter" || event.shiftKey || event.isComposing) return;
      event.preventDefault();
      if (!el.send.disabled) el.form.requestSubmit();
    });
    el.messages.addEventListener("scroll", updateScrollButton, { passive: true });
    el.scrollBottom.addEventListener("click", () => scrollMessagesToBottom(true));
    for (const toggle of [el.search, el.read, el.processToggle]) toggle.addEventListener("change", updateToolStatus);
    root.addEventListener("click", (event) => {
      const toolChip = event.target.closest("[data-chat-tool-chip]");
      if (toolChip) {
        const target = { search: el.search, read: el.read, process: el.processToggle }[toolChip.dataset.chatToolChip];
        if (target) {
          target.checked = !target.checked;
          target.dispatchEvent(new Event("change", { bubbles: true }));
        }
        return;
      }
      const avatarButton = event.target.closest("[data-chat-avatar-value]");
      if (avatarButton) {
        const target = avatarButton.dataset.chatAvatarTarget === "assistant" ? el.assistantAvatar : el.userAvatar;
        if (target && !target.disabled) {
          target.value = avatarButton.dataset.chatAvatarValue || "";
          target.focus();
        }
        return;
      }
      const codeView = event.target.closest("[data-code-view]");
      if (codeView) {
        setCodeBlockView(codeView.closest("[data-code-block]"), codeView.dataset.codeView, htmlPreviewOptions());
        return;
      }
      const reloadPreview = event.target.closest("[data-reload-preview]");
      if (reloadPreview) {
        loadHTMLPreview(reloadPreview.closest("[data-code-block]"), htmlPreviewOptions());
        return;
      }
      const codeButton = event.target.closest("[data-copy-code]");
      if (codeButton) {
        const source = codeButton.closest("[data-code-block]")?.querySelector('[data-code-pane="code"] code')?.textContent || "";
        copyText(source).then(() => showToast(t("copied")));
        return;
      }
      const messageButton = event.target.closest("[data-copy-message]");
      if (messageButton) {
        const message = state.messages[Number(messageButton.dataset.copyMessage)];
        copyText(message?.content || "").then(() => showToast(t("copied")));
        return;
      }
      const regenerateButton = event.target.closest("[data-regenerate-message]");
      if (regenerateButton) {
        regenerateMessage(Number(regenerateButton.dataset.regenerateMessage)).catch((error) => showToast(error.message, "error"));
      }
    });
    root.addEventListener("keydown", (event) => {
      const tab = event.target.closest("[data-code-view]");
      if (!tab || !["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
      const tabs = Array.from(tab.closest('[role="tablist"]')?.querySelectorAll("[data-code-view]") || []);
      if (!tabs.length) return;
      event.preventDefault();
      const current = tabs.indexOf(tab);
      const index = event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : event.key === "ArrowRight" ? (current + 1) % tabs.length : (current - 1 + tabs.length) % tabs.length;
      setCodeBlockView(tab.closest("[data-code-block]"), tabs[index].dataset.codeView, htmlPreviewOptions(true));
    });
    document.addEventListener("click", (event) => {
      if (!event.target.closest(".chat-account-button, .chat-account-menu, .chat-tools-wrap, .chat-header-menu-wrap, [data-chat-floating-menu], [data-chat-row-menu]")) closeAllMenus();
    });
    document.addEventListener("keydown", (event) => {
      if (event.key !== "Escape") return;
      if (!el.settingsModal.classList.contains("hidden")) {
        event.preventDefault();
        closeSettings();
        return;
      }
      if (root.classList.contains("sidebar-open")) closeMobileSidebar();
      closeAllMenus();
    });
  }

  function bindViewport() {
    const viewport = window.visualViewport;
    let baselineHeight = Math.round(viewport?.height || window.innerHeight || document.documentElement.clientHeight || 1);
    let editableFocused = false;
    let restoreTimer = 0;

    const resetPageScroll = () => {
      const previousBehavior = document.documentElement.style.scrollBehavior;
      document.documentElement.style.scrollBehavior = "auto";
      window.scrollTo(0, 0);
      document.documentElement.style.scrollBehavior = previousBehavior;
    };

    const sync = ({ restore = false } = {}) => {
      const viewportHeight = viewport?.height || window.innerHeight || document.documentElement.clientHeight || baselineHeight;
      if (!editableFocused || viewportHeight > baselineHeight) baselineHeight = Math.round(viewportHeight);
      const frame = chatViewportFrame({
        viewportHeight,
        viewportOffsetTop: viewport?.offsetTop || 0,
        baselineHeight,
        editableFocused,
      });
      root.style.setProperty("--chat-viewport-height", `${frame.height}px`);
      root.style.setProperty("--chat-viewport-offset-top", `${frame.offsetTop}px`);
      if (!frame.keyboardOpen && (restore || window.scrollX || window.scrollY)) resetPageScroll();
    };

    const scheduleRestore = () => {
      window.clearTimeout(restoreTimer);
      requestAnimationFrame(() => sync({ restore: true }));
      restoreTimer = window.setTimeout(() => sync({ restore: true }), 350);
    };

    root.addEventListener("focusin", (event) => {
      if (!event.target.matches("input, select, textarea")) return;
      editableFocused = true;
      sync();
    });
    root.addEventListener("focusout", (event) => {
      if (!event.target.matches("input, select, textarea")) return;
      editableFocused = false;
      scheduleRestore();
    });
    viewport?.addEventListener("resize", () => sync(), { passive: true });
    viewport?.addEventListener("scroll", () => sync(), { passive: true });
    window.addEventListener("orientationchange", () => {
      editableFocused = false;
      window.setTimeout(() => {
        baselineHeight = Math.round(viewport?.height || window.innerHeight || baselineHeight);
        scheduleRestore();
      }, 250);
    });
    sync({ restore: true });
  }

  function autosizeInput() {
    el.input.style.height = "auto";
    el.input.style.height = `${Math.min(el.input.scrollHeight, 200)}px`;
  }
}

function trapFocus(event, container) {
  if (event.key !== "Tab") return;
  const focusable = Array.from(container.querySelectorAll("button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex='-1'])"));
  if (!focusable.length) return;
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

function thinkingEffort(root) {
  return root.querySelector("input[name$='-thinking']:checked")?.value || "medium";
}

function setThinking(root, value) {
  const input = Array.from(root.querySelectorAll("input[name$='-thinking']")).find((item) => item.value === value);
  if (input) input.checked = true;
}

function labelEffort(value, t) {
  switch (value) {
    case "off": return t("off");
    case "low": return t("low");
    case "high": return t("high");
    default: return t("medium");
  }
}

function avatarValue(value, fallback) {
  const text = String(value || "").trim();
  if (!text) return fallback;
  return Array.from(text).slice(0, 16).join("");
}

function normalizeLang(lang) {
  return String(lang || "").toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

function translate(lang, key, values = {}) {
  const dictionary = translations[lang] || translations.en;
  let text = dictionary[key] || translations.en[key] || key;
  for (const [name, value] of Object.entries(values)) text = text.replaceAll(`{${name}}`, String(value));
  return text;
}

function setText(node, text) {
  if (node) node.textContent = text;
}

function setTitle(node, text) {
  if (!node) return;
  node.title = text;
  node.setAttribute("aria-label", text);
}

function setButtonContent(button, text) {
  const span = button?.querySelector("span");
  if (span) span.textContent = text;
  setTitle(button, text);
}

function setCheckboxLabel(input, text) {
  const span = input?.closest("label")?.querySelector("span");
  if (span) span.textContent = text;
}

function setLeadingText(label, text) {
  if (!label) return;
  for (const node of Array.from(label.childNodes)) {
    if (node.nodeType === Node.TEXT_NODE && node.textContent.trim()) {
      node.textContent = text;
      return;
    }
  }
  label.insertBefore(document.createTextNode(text), label.firstChild);
}

function setTrailingText(label, text) {
  if (!label) return;
  for (const node of Array.from(label.childNodes)) {
    if (node.nodeType === Node.TEXT_NODE) {
      node.textContent = ` ${text}`;
      return;
    }
  }
  label.appendChild(document.createTextNode(` ${text}`));
}

function renderAvatarPresets(picker, target, values, label) {
  if (!picker) return;
  picker.setAttribute("aria-label", label);
  picker.innerHTML = values.map((value) => `<button type="button" class="chat-avatar-preset" data-chat-avatar-target="${esc(target)}" data-chat-avatar-value="${esc(value)}" title="${esc(value)}">${esc(value)}</button>`).join("");
}

function usageFromMessage(message) {
  if (!message?.metadata) return null;
  try {
    const metadata = typeof message.metadata === "string" ? JSON.parse(message.metadata) : message.metadata;
    return normalizeTokenUsage(metadata.usage);
  } catch {
    return null;
  }
}

function mergeMessageUsage(message, usage) {
  const normalized = normalizeTokenUsage(usage);
  if (!normalized) return;
  let metadata = {};
  try {
    metadata = message.metadata ? (typeof message.metadata === "string" ? JSON.parse(message.metadata) : message.metadata) : {};
  } catch {
    metadata = {};
  }
  metadata.usage = {
    input_tokens: normalized.input_tokens,
    output_tokens: normalized.output_tokens,
    cache_read_tokens: normalized.cache_read_tokens,
    cache_creation_tokens: normalized.cache_creation_tokens,
  };
  message.metadata = JSON.stringify(metadata);
}

function messageUsageEstimated(message) {
  try {
    const metadata = typeof message.metadata === "string" ? JSON.parse(message.metadata) : message.metadata;
    return Boolean(metadata?.usage_estimated);
  } catch {
    return false;
  }
}

function renderSources(message, label = "Sources") {
  const urls = new Set((message.content || "").match(/https?:\/\/[^\s)]+/g) || []);
  if (!urls.size) return "";
  return `<details class="chat-sources"><summary><svg class="icon chat-details-chevron" aria-hidden="true"><use href="/admin/static/icons.svg#icon-chevron-right"></use></svg><span>${esc(label)}</span></summary>${Array.from(urls).map((url) => `<a href="${esc(url)}" target="_blank" rel="noopener noreferrer">${esc(url)}</a>`).join("")}</details>`;
}

function compact(value, limit = 1200) {
  const text = String(value || "");
  return text.length > limit ? `${text.slice(0, limit)}\n[truncated]` : text;
}
