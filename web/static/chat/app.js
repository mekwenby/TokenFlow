import { esc, cookie } from "../core/dom.js";
import { showToast, copyText } from "../core/toast.js";

const roots = Array.from(document.querySelectorAll("[data-chat-root]"));

const defaultUserAvatar = "😀";
const defaultAssistantAvatar = "🤖";
const userAvatarPresets = ["😀", "😎", "🙂", "😊", "🧑‍💻", "👩‍💻", "🧑‍🚀", "🧑‍🎨", "🧑‍🔬", "🧑‍🏫", "🐱", "🐼"];
const assistantAvatarPresets = ["🤖", "🧠", "✨", "🧭", "📚", "🛠️", "🔍", "🌐", "💡", "🧪", "🧰", "🚀"];

const translations = {
  en: {
    assistant: "Assistant",
    assistant_avatar: "Assistant avatar",
    assistant_avatar_presets: "Assistant avatar presets",
    ask_placeholder: "Ask a question...",
    avatar_settings: "Avatar settings",
    cache_creation_tokens: "Cache write",
    cache_read_tokens: "Cache read",
    calling_model: "Calling model",
    cancel: "Cancel",
    chat_settings: "Chat settings",
    close: "Close",
    close_settings: "Close settings",
    collapse_process: "Collapse process",
    completed: "completed",
    conversation_busy: "This conversation is processing. Please try again later.",
    conversation_name: "Conversation name",
    conversations: "Conversations",
    copied: "Copied",
    copy: "Copy",
    default_system_prompt: "Default system prompt",
    default_system_prompt_empty: "Default system prompt is unavailable.",
    default_system_prompt_hint: "Always applied by TokenFlow. Your instructions below are appended.",
    delete: "Delete",
    delete_confirm: "Delete this conversation?",
    expand_process: "Expand process",
    failed: "failed",
    generate_title: "Generate title",
    high: "High",
    input_tokens: "Input",
    low: "Low",
    medium: "Medium",
    model: "Model",
    model_loading: "Model loading",
    max_tool_calls: "Max tool calls",
    max_tool_calls_summary: "Tools {count}x",
    my_nickname: "My nickname",
    new_chat: "New chat",
    no_conversations: "No conversations",
    no_model: "No model",
    no_models: "No models",
    no_process_events: "No process events yet.",
    off: "Off",
    output_tokens: "Output",
    process: "Process",
    process_subtitle: "Thinking and tools",
    process_timeline: "Process timeline",
    read_web: "Read web",
    rename: "Rename",
    request_failed: "Request failed",
    save: "Save",
    saved_to_conversation: "Saved to this conversation",
    scroll_bottom: "Scroll to bottom",
    scroll_top: "Scroll to top",
    search: "Search",
    send: "Send",
    settings: "Settings",
    settings_saved: "Settings saved",
    show_process_panel: "Show process panel",
    sources: "Sources",
    start_conversation: "Start a conversation.",
    started: "started",
    starting: "Starting",
    status: "Status",
    status_failed: "Failed",
    status_responding: "Responding",
    status_stopped: "Stopped",
    status_title_generating: "Generating title",
    stop: "Stop",
    stopped: "Stopped.",
    system_prompt: "Additional system prompt",
    system_prompt_placeholder: "Optional instructions for this conversation",
    thinking: "Thinking",
    title_generated: "Title updated",
    title_generating: "Generating title",
    title_no_messages: "Current conversation has no messages to summarize.",
    token_usage: "Token",
    tool_controls: "Tool controls",
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
    ask_placeholder: "输入消息...",
    avatar_settings: "头像设置",
    cache_creation_tokens: "缓存写入",
    cache_read_tokens: "缓存读取",
    calling_model: "正在调用模型",
    cancel: "取消",
    chat_settings: "聊天设置",
    close: "关闭",
    close_settings: "关闭设置",
    collapse_process: "折叠过程",
    completed: "已完成",
    conversation_busy: "当前会话正在处理中，请稍后再试",
    conversation_name: "对话名称",
    conversations: "对话",
    copied: "已复制",
    copy: "复制",
    default_system_prompt: "默认系统提示词",
    default_system_prompt_empty: "默认系统提示词暂不可用。",
    default_system_prompt_hint: "TokenFlow 会始终应用默认提示词，下方内容会作为附加指令拼接。",
    delete: "删除",
    delete_confirm: "确定删除这个对话吗？",
    expand_process: "展开过程",
    failed: "失败",
    generate_title: "自动生成标题",
    high: "高",
    input_tokens: "输入",
    low: "低",
    medium: "中",
    model: "模型",
    model_loading: "模型加载中",
    max_tool_calls: "最大工具调用次数",
    max_tool_calls_summary: "工具 {count} 次",
    my_nickname: "我的昵称",
    new_chat: "新对话",
    no_conversations: "暂无对话",
    no_model: "无模型",
    no_models: "暂无模型",
    no_process_events: "暂无过程事件。",
    off: "关闭",
    output_tokens: "输出",
    process: "过程",
    process_subtitle: "思考和工具调用",
    process_timeline: "过程时间线",
    read_web: "读取网页",
    rename: "重命名",
    request_failed: "请求失败",
    save: "保存",
    saved_to_conversation: "保存到当前对话",
    scroll_bottom: "滚到底部",
    scroll_top: "滚到顶部",
    search: "搜索",
    send: "发送",
    settings: "设置",
    settings_saved: "设置已保存",
    show_process_panel: "显示过程面板",
    sources: "来源",
    start_conversation: "开始一个对话。",
    started: "已开始",
    starting: "正在开始",
    status: "状态",
    status_failed: "失败",
    status_responding: "回复中",
    status_stopped: "已停止",
    status_title_generating: "标题生成中",
    stop: "停止",
    stopped: "已停止。",
    system_prompt: "附加系统提示词",
    system_prompt_placeholder: "可选，给当前对话追加额外指令",
    thinking: "思考强度",
    title_generated: "标题已更新",
    title_generating: "标题生成中",
    title_no_messages: "当前会话没有可总结的消息。",
    token_usage: "Token",
    tool_controls: "工具控制",
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
  const settingsWritable = root.dataset.chatSettingsWritable === "true";
  const lang = normalizeLang(root.dataset.chatLang || document.documentElement.lang || "en");
  const t = (key, values) => translate(lang, key, values);
  const numberFormat = new Intl.NumberFormat(lang);
  const state = {
    conversations: [],
    messages: [],
    models: [],
    defaultSystemPrompt: "",
    maxToolCalls: 6,
    settingsWritable,
    activeId: null,
    activeConversation: null,
    messagesByConversationId: new Map(),
    processEventsByConversationId: new Map(),
    streamsByConversationId: new Map(),
    titleGeneratingIds: new Set(),
    processEvents: [],
    processCollapsed: false,
    pollTimer: null,
  };

  const el = {
    list: root.querySelector("[data-chat-conversations]"),
    messages: root.querySelector("[data-chat-messages]"),
    processShell: root.querySelector("[data-chat-process-shell]"),
    process: root.querySelector("[data-chat-process-panel]"),
    processCollapse: root.querySelector("[data-chat-process-collapse]"),
    processCollapseIcon: root.querySelector("[data-chat-process-collapse-icon]"),
    processReopen: root.querySelector("[data-chat-process-reopen]"),
    processTop: root.querySelector("[data-chat-process-top]"),
    processBottom: root.querySelector("[data-chat-process-bottom]"),
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
    search: root.querySelector("[data-chat-search]"),
    read: root.querySelector("[data-chat-read]"),
    processToggle: root.querySelector("[data-chat-process]"),
    stop: root.querySelector("[data-chat-stop]"),
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
    if (!response.ok) throw new Error(localizeError(body.error || t("request_failed")));
    return body;
  };

  async function load() {
    localizeStatic();
    try {
      const [models, conversations, settings] = await Promise.all([
        api("/models"),
        api("/conversations"),
        api("/settings"),
      ]);
      state.models = models.models || [];
      state.defaultSystemPrompt = models.default_system_prompt || "";
      state.maxToolCalls = normalizeMaxToolCalls(settings.max_tool_calls ?? models.max_tool_calls ?? state.maxToolCalls);
      state.conversations = conversations.items || [];
      renderModels();
      renderConversations();
      if (state.conversations.length) {
        await openConversation(state.conversations[0].id);
      } else {
        await createConversation();
      }
      startConversationPolling();
    } catch (error) {
      showToast(error.message, "error");
      el.messages.innerHTML = `<p class="empty">${esc(error.message)}</p>`;
    }
  }

  function renderModels() {
    el.model.innerHTML = state.models.length
      ? state.models.map((model) => `<option value="${esc(model)}">${esc(model)}</option>`).join("")
      : `<option value="">${esc(t("no_models"))}</option>`;
    populateSettings(activeConversation());
  }

  function renderConversations() {
    el.list.innerHTML = state.conversations.length
      ? state.conversations.map((conv) => {
        const busy = isConversationBusy(conv);
        return `
        <div class="chat-conversation-row ${conv.id === state.activeId ? "active" : ""}">
          <button type="button" class="chat-conversation" data-chat-open="${esc(conv.id)}">
            <span class="chat-conversation-title-line">
              <span class="chat-conversation-name">${esc(displayConversationTitle(conv))}</span>
              ${renderConversationStatus(conv)}
            </span>
            <small>${esc(conv.model || "")}</small>
          </button>
          <div class="chat-conversation-actions">
            <button type="button" class="chat-conversation-action" data-chat-rename-id="${esc(conv.id)}" title="${esc(t("rename"))}" aria-label="${esc(t("rename"))}" ${busy ? "disabled" : ""}>
              <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-edit"></use></svg>
            </button>
            <button type="button" class="chat-conversation-action" data-chat-delete-id="${esc(conv.id)}" title="${esc(t("delete"))}" aria-label="${esc(t("delete"))}">
              <svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-trash"></use></svg>
            </button>
          </div>
        </div>`;
      }).join("")
      : `<p class="empty">${esc(t("no_conversations"))}</p>`;
    syncSettingsDisabled();
  }

  function renderConversationStatus(conversation) {
    const status = conversationStatus(conversation);
    const emoji = conversationStatusEmoji(status);
    if (!emoji) return "";
    const label = conversationStatusLabel(status);
    return `<span class="chat-status-emoji" title="${esc(label)}" aria-label="${esc(label)}">${emoji}</span>`;
  }

  function conversationStatus(conversation) {
    const id = conversation?.id;
    const stream = state.streamsByConversationId.get(id);
    if (stream && !stream.responseDone) return "responding";
    if (state.titleGeneratingIds.has(id)) return "title_generating";
    const active = conversation?.active_operation || "";
    if (active === "responding") return "responding";
    if (active === "title_generating") return "title_generating";
    return conversation?.status || "idle";
  }

  function conversationStatusEmoji(status) {
    switch (status) {
      case "responding":
        return "💬";
      case "title_generating":
        return "🏷️";
      case "failed":
        return "⚠️";
      case "stopped":
        return "⏹️";
      default:
        return "";
    }
  }

  function conversationStatusLabel(status) {
    switch (status) {
      case "responding":
        return t("status_responding");
      case "title_generating":
        return t("status_title_generating");
      case "failed":
        return t("status_failed");
      case "stopped":
        return t("status_stopped");
      default:
        return "";
    }
  }

  function renderMessages() {
    if (!state.messages.length) {
      el.messages.innerHTML = `<p class="empty">${esc(t("start_conversation"))}</p>`;
      return;
    }
    const conversation = activeConversation();
    const userLabel = conversation.nickname || t("you");
    const userAvatar = avatarValue(conversation.user_avatar, defaultUserAvatar);
    const assistantAvatar = avatarValue(conversation.assistant_avatar, defaultAssistantAvatar);
    el.messages.innerHTML = state.messages.map((message) => {
      const isUser = message.role === "user";
      const label = isUser ? userLabel : t("assistant");
      const avatar = isUser ? userAvatar : assistantAvatar;
      return `
      <article class="chat-message ${isUser ? "user" : "assistant"}">
        <div class="chat-message-avatar" aria-hidden="true">${esc(avatar)}</div>
        <div class="chat-message-stack">
          <div class="chat-message-role">${esc(label)}</div>
          <div class="markdown-body">${renderMarkdown(message.content || "", t("copy"))}${renderSources(message, t("sources"))}${renderUsage(message)}</div>
        </div>
      </article>`;
    }).join("");
    el.messages.scrollTop = el.messages.scrollHeight;
  }

  function renderUsage(message) {
    if (message.role !== "assistant") return "";
    const usage = usageFromMessage(message);
    if (!usage) return "";
    const total = usage.input_tokens + usage.output_tokens + usage.cache_read_tokens + usage.cache_creation_tokens;
    const parts = [
      `${t("total_tokens")} ${numberFormat.format(total)}`,
      `${t("input_tokens")} ${numberFormat.format(usage.input_tokens)}`,
      `${t("output_tokens")} ${numberFormat.format(usage.output_tokens)}`,
    ];
    if (usage.cache_read_tokens) parts.push(`${t("cache_read_tokens")} ${numberFormat.format(usage.cache_read_tokens)}`);
    if (usage.cache_creation_tokens) parts.push(`${t("cache_creation_tokens")} ${numberFormat.format(usage.cache_creation_tokens)}`);
    return `<div class="chat-message-usage"><span>${esc(t("token_usage"))}</span>${parts.map((part) => `<b>${esc(part)}</b>`).join("")}</div>`;
  }

  function renderProcess(events = state.processEvents) {
    syncProcessPanel();
    if (!el.processToggle.checked) return;
    const wasNearBottom = isNearBottom(el.process);
    const previousScroll = el.process.scrollTop;
    el.process.innerHTML = events.length
      ? events.map(renderProcessEvent).join("")
      : `<p class="empty">${esc(t("no_process_events"))}</p>`;
    if (wasNearBottom) {
      scrollProcess("bottom");
    } else {
      el.process.scrollTop = previousScroll;
    }
  }

  function syncProcessPanel() {
    const visible = el.processToggle.checked;
    el.processShell.classList.toggle("hidden", !visible);
    el.processShell.classList.toggle("collapsed", visible && state.processCollapsed);
    el.processReopen?.classList.toggle("hidden", visible && !state.processCollapsed);
    el.processCollapse.setAttribute("aria-expanded", String(visible && !state.processCollapsed));
    const expanding = state.processCollapsed;
    el.processCollapse.title = expanding ? t("expand_process") : t("collapse_process");
    el.processCollapse.setAttribute("aria-label", expanding ? t("expand_process") : t("collapse_process"));
    const iconHref = `/admin/static/icons.svg#${expanding ? "icon-chevron-left" : "icon-chevron-right"}`;
    el.processCollapseIcon.setAttribute("href", iconHref);
    el.processCollapseIcon.setAttributeNS("http://www.w3.org/1999/xlink", "href", iconHref);
  }

  function openProcessPanel() {
    el.processToggle.checked = true;
    state.processCollapsed = false;
    renderProcess();
    scrollProcess("bottom");
  }

  function isNearBottom(node) {
    return node.scrollHeight - node.scrollTop - node.clientHeight < 32;
  }

  function scrollProcess(position) {
    el.process.scrollTop = position === "top" ? 0 : el.process.scrollHeight;
  }

  function renderProcessEvent(event) {
    const type = event.type || "status";
    if (type === "tool_start") {
      return `<div class="chat-event"><strong>${esc(event.name)}</strong><span>${esc(t("started"))}</span><code>${esc(event.arguments || "")}</code></div>`;
    }
    if (type === "tool_result") {
      return `<div class="chat-event ${event.ok ? "" : "warn"}"><strong>${esc(event.name)}</strong><span>${esc(event.ok ? t("completed") : t("failed"))}</span><pre>${esc(compact(event.result || ""))}</pre></div>`;
    }
    if (type === "thinking") {
      return `<div class="chat-event"><strong>${esc(t("thinking"))}</strong><pre>${esc(event.content || "")}</pre></div>`;
    }
    if (type === "warning") {
      return `<div class="chat-event warn"><strong>${esc(t("warning"))}</strong><span>${esc(event.message || "")}</span></div>`;
    }
    const message = event.message_key ? t(event.message_key) : (event.message || type);
    return `<div class="chat-event"><strong>${esc(t("status"))}</strong><span>${esc(message)}</span></div>`;
  }

  async function createConversation() {
    const model = el.model.value || state.models[0] || "";
    const conv = await api("/conversations", {
      method: "POST",
      body: JSON.stringify({
        title: "",
        model,
        thinking_effort: thinkingEffort(root),
        system_prompt: el.systemPrompt?.value || "",
        nickname: el.nickname?.value || "",
        user_avatar: el.userAvatar?.value || defaultUserAvatar,
        assistant_avatar: el.assistantAvatar?.value || defaultAssistantAvatar,
      }),
    });
    state.conversations = [conv, ...state.conversations.filter((item) => item.id !== conv.id)];
    renderConversations();
    await openConversation(conv.id);
  }

  async function openConversation(id) {
    const body = await api(`/conversations/${encodeURIComponent(id)}`);
    state.activeId = body.conversation.id;
    state.activeConversation = body.conversation;
    if (!state.streamsByConversationId.has(state.activeId)) {
      setConversationMessages(state.activeId, body.messages || []);
      setConversationProcessEvents(state.activeId, eventsFromMessages(body.messages || []));
    } else if (!state.messagesByConversationId.has(state.activeId)) {
      setConversationMessages(state.activeId, body.messages || []);
      setConversationProcessEvents(state.activeId, eventsFromMessages(body.messages || []));
    }
    syncActiveConversationCache();
    populateSettings(body.conversation);
    renderConversations();
    renderMessages();
    renderProcess();
    syncSettingsDisabled();
  }

  async function renameConversation(id = state.activeId) {
    if (!id) return;
    const current = state.conversations.find((conv) => conv.id === id);
    const title = prompt(t("conversation_name"), current?.title || t("new_chat"));
    if (title == null) return;
    const conv = await api(`/conversations/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ title }),
    });
    state.conversations = state.conversations.map((item) => item.id === conv.id ? conv : item);
    if (state.activeId === conv.id) {
      state.activeConversation = conv;
      populateSettings(conv);
    }
    renderConversations();
  }

  async function generateConversationTitle() {
    const conversationId = state.activeId;
    if (isConversationBusyById(conversationId)) {
      showToast(t("conversation_busy"), "error");
      return;
    }
    if (!conversationId) {
      showToast(t("title_no_messages"), "error");
      return;
    }
    state.titleGeneratingIds.add(conversationId);
    markConversationStatus(conversationId, "title_generating", "title_generating");
    syncSettingsDisabled();
    setTitle(el.autoTitle, t("title_generating"));
    try {
      const conv = await api(`/conversations/${encodeURIComponent(conversationId)}/title`, { method: "POST" });
      upsertConversation(conv);
      if (state.activeId === conv.id) {
        state.activeConversation = conv;
        populateSettings(conv);
      }
      renderConversations();
      showToast(t("title_generated"));
    } catch (error) {
      const message = error.message === "current conversation has no messages to summarize" ? t("title_no_messages") : error.message;
      showToast(message || t("request_failed"), "error");
    } finally {
      state.titleGeneratingIds.delete(conversationId);
      await refreshConversations({ silent: true }).catch(() => {});
      syncSettingsDisabled();
    }
  }

  async function deleteConversation(id = state.activeId) {
    if (!id) return;
    if (!confirm(t("delete_confirm"))) return;
    abortConversationStream(id);
    const deletingActive = id === state.activeId;
    await api(`/conversations/${encodeURIComponent(id)}`, { method: "DELETE" });
    state.conversations = state.conversations.filter((conv) => conv.id !== id);
    state.messagesByConversationId.delete(id);
    state.processEventsByConversationId.delete(id);
    state.streamsByConversationId.delete(id);
    state.titleGeneratingIds.delete(id);
    if (deletingActive) {
      state.activeId = null;
      state.activeConversation = null;
      state.messages = [];
      state.processEvents = [];
      renderConversations();
      renderMessages();
      renderProcess();
      if (state.conversations.length) await openConversation(state.conversations[0].id);
      else populateSettings(null);
      return;
    }
    renderConversations();
  }

  async function openSettings() {
    if (!state.activeId) await createConversation();
    populateSettings(activeConversation());
    syncSettingsDisabled();
    el.settingsModal.classList.remove("hidden");
    el.settingsModal.setAttribute("aria-hidden", "false");
    setTimeout(() => el.model?.focus(), 0);
  }

  function closeSettings() {
    el.settingsModal.classList.add("hidden");
    el.settingsModal.setAttribute("aria-hidden", "true");
    populateSettings(activeConversation());
  }

  async function saveSettings(event) {
    event.preventDefault();
    if (isConversationBusyById(state.activeId)) {
      showToast(t("conversation_busy"), "error");
      return;
    }
    if (!state.activeId) await createConversation();
    const conv = await api(`/conversations/${encodeURIComponent(state.activeId)}`, {
      method: "PATCH",
      body: JSON.stringify({
        model: el.model.value,
        thinking_effort: thinkingEffort(root),
        system_prompt: el.systemPrompt?.value || "",
        nickname: el.nickname?.value || "",
        user_avatar: el.userAvatar?.value || defaultUserAvatar,
        assistant_avatar: el.assistantAvatar?.value || defaultAssistantAvatar,
      }),
    });
    state.activeConversation = conv;
    state.conversations = state.conversations.map((item) => item.id === conv.id ? conv : item);
    if (state.settingsWritable && el.maxToolCalls) {
      const settings = await api("/settings", {
        method: "PATCH",
        body: JSON.stringify({ max_tool_calls: normalizeMaxToolCalls(el.maxToolCalls.value) }),
      });
      state.maxToolCalls = normalizeMaxToolCalls(settings.max_tool_calls);
    }
    populateSettings(conv);
    renderConversations();
    renderMessages();
    closeSettings();
    showToast(t("settings_saved"));
  }

  async function sendMessage(event) {
    event.preventDefault();
    if (!state.activeId) await createConversation();
    const conversationId = state.activeId;
    if (isConversationBusyById(conversationId)) {
      showToast(t("conversation_busy"), "error");
      return;
    }
    const content = el.input.value.trim();
    if (!content) return;
    const abort = new AbortController();
    const processEvents = [{ type: "status", message: t("starting") }];
    const messages = [...conversationMessages(conversationId), { role: "user", content }, { role: "assistant", content: "" }];
    const stream = { abort, draft: "", messages, processEvents, responseDone: false };
    state.streamsByConversationId.set(conversationId, stream);
    setConversationMessages(conversationId, messages);
    setConversationProcessEvents(conversationId, processEvents);
    markConversationStatus(conversationId, "responding", "responding");
    syncSettingsDisabled();
    el.input.value = "";
    autosizeInput();
    renderConversationIfActive(conversationId);

    try {
      const response = await fetch(`${apiPrefix}/conversations/${encodeURIComponent(conversationId)}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": cookie(csrfCookie),
        },
        signal: abort.signal,
        body: JSON.stringify({
          content,
          enable_search: el.search.checked,
          enable_read: el.read.checked,
          show_process: el.processToggle.checked,
        }),
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(localizeError(body.error || t("request_failed")));
      }
      await readSSE(response, (streamEvent, data) => handleStreamEvent(conversationId, streamEvent, data));
      await refreshConversations({ silent: true });
    } catch (error) {
      if (error.name !== "AbortError") showToast(error.message, "error");
      const messages = conversationMessages(conversationId);
      const last = messages[messages.length - 1];
      if (last?.role === "assistant" && !last.content) last.content = error.name === "AbortError" ? t("stopped") : error.message;
      setConversationMessages(conversationId, messages);
      renderConversationIfActive(conversationId);
    } finally {
      if (state.streamsByConversationId.get(conversationId) === stream) {
        state.streamsByConversationId.delete(conversationId);
      }
      await refreshConversations({ silent: true }).catch(() => {});
      syncSettingsDisabled();
    }
  }

  function handleStreamEvent(conversationId, event, data) {
    if (event === "delta") {
      const stream = state.streamsByConversationId.get(conversationId);
      const messages = conversationMessages(conversationId);
      if (stream) stream.draft += data.content || "";
      const last = messages[messages.length - 1];
      if (last?.role === "assistant") last.content = stream?.draft || `${last.content || ""}${data.content || ""}`;
      setConversationMessages(conversationId, messages);
      renderConversationIfActive(conversationId);
      return;
    }
    if (event === "assistant_message") {
      const messages = conversationMessages(conversationId);
      const last = messages[messages.length - 1];
      if (last?.role === "assistant") Object.assign(last, data);
      const processEvents = eventsFromMessages(messages);
      setConversationMessages(conversationId, messages);
      setConversationProcessEvents(conversationId, processEvents);
      const stream = state.streamsByConversationId.get(conversationId);
      if (stream) {
        stream.messages = messages;
        stream.processEvents = processEvents;
      }
      renderConversationIfActive(conversationId);
      return;
    }
    if (event === "done") {
      const stream = state.streamsByConversationId.get(conversationId);
      if (stream) stream.responseDone = true;
      const messages = conversationMessages(conversationId);
      const last = messages[messages.length - 1];
      if (last?.role === "assistant" && data.usage) {
        mergeMessageUsage(last, data.usage);
        setConversationMessages(conversationId, messages);
        renderConversationIfActive(conversationId);
      }
      return;
    }
    if (event === "tool_start" || event === "tool_result" || event === "thinking" || event === "warning" || event === "status") {
      const payload = { ...data, type: data.type || event };
      const processEvents = [...conversationProcessEvents(conversationId), payload];
      setConversationProcessEvents(conversationId, processEvents);
      const stream = state.streamsByConversationId.get(conversationId);
      if (stream) stream.processEvents = processEvents;
      renderConversationIfActive(conversationId);
      return;
    }
    if (event === "conversation") {
      upsertConversation(data);
      if (state.activeId === data.id) {
        state.activeConversation = data;
        populateSettings(data);
      }
      renderConversations();
      return;
    }
    if (event === "error") {
      if (state.activeId === conversationId) showToast(data.message || t("request_failed"), "error");
      const messages = conversationMessages(conversationId);
      const last = messages[messages.length - 1];
      if (last?.role === "assistant" && !last.content) {
        last.content = data.message || t("request_failed");
        setConversationMessages(conversationId, messages);
        renderConversationIfActive(conversationId);
      }
    }
  }

  async function refreshConversations(options = {}) {
    try {
      const body = await api("/conversations");
      state.conversations = body.items || [];
      state.activeConversation = state.conversations.find((conv) => conv.id === state.activeId) || state.activeConversation;
      populateSettings(activeConversation());
      renderConversations();
    } catch (error) {
      if (!options.silent) throw error;
    }
  }

  function startConversationPolling() {
    if (state.pollTimer) return;
    state.pollTimer = window.setInterval(() => {
      if (document.hidden) return;
      refreshConversations({ silent: true });
    }, 5000);
    document.addEventListener("visibilitychange", () => {
      if (!document.hidden) refreshConversations({ silent: true });
    });
  }

  function activeConversation() {
    return state.activeConversation || state.conversations.find((conv) => conv.id === state.activeId) || {};
  }

  function conversationMessages(id) {
    return state.messagesByConversationId.get(id) || [];
  }

  function setConversationMessages(id, messages) {
    if (!id) return;
    state.messagesByConversationId.set(id, messages || []);
    if (state.activeId === id) state.messages = state.messagesByConversationId.get(id) || [];
  }

  function conversationProcessEvents(id) {
    return state.processEventsByConversationId.get(id) || [];
  }

  function setConversationProcessEvents(id, events) {
    if (!id) return;
    state.processEventsByConversationId.set(id, events || []);
    if (state.activeId === id) state.processEvents = state.processEventsByConversationId.get(id) || [];
  }

  function syncActiveConversationCache() {
    state.messages = conversationMessages(state.activeId);
    state.processEvents = conversationProcessEvents(state.activeId);
  }

  function renderConversationIfActive(id) {
    if (state.activeId !== id) {
      renderConversations();
      return;
    }
    syncActiveConversationCache();
    renderConversations();
    renderMessages();
    renderProcess();
    syncSettingsDisabled();
  }

  function activeStream() {
    return state.streamsByConversationId.get(state.activeId) || null;
  }

  function isConversationBusy(conversation = activeConversation()) {
    const id = conversation?.id;
    return isConversationBusyById(id, conversation);
  }

  function isConversationBusyById(id, conversation = null) {
    if (!id) return false;
    if (state.streamsByConversationId.has(id) || state.titleGeneratingIds.has(id)) return true;
    const conv = conversation || state.conversations.find((item) => item.id === id) || (state.activeId === id ? state.activeConversation : null);
    const operation = conv?.active_operation || "";
    return operation === "responding" || operation === "title_generating";
  }

  function markConversationStatus(id, activeOperation, status) {
    if (!id) return;
    const patch = { active_operation: activeOperation || "", status: status || "idle" };
    state.conversations = state.conversations.map((conv) => conv.id === id ? { ...conv, ...patch } : conv);
    if (state.activeId === id) state.activeConversation = { ...(activeConversation() || {}), ...patch };
    renderConversations();
  }

  function upsertConversation(conversation) {
    if (!conversation?.id) return;
    state.conversations = [conversation, ...state.conversations.filter((item) => item.id !== conversation.id)];
  }

  function abortConversationStream(id) {
    const stream = state.streamsByConversationId.get(id);
    if (stream) stream.abort.abort();
  }

  function localizeError(message) {
    switch (String(message || "")) {
      case "conversation is already processing":
        return t("conversation_busy");
      case "current conversation has no messages to summarize":
        return t("title_no_messages");
      default:
        return message || t("request_failed");
    }
  }

  function displayConversationTitle(conversation) {
    const title = conversation?.title || "";
    if (conversation?.title_auto_generated && isDefaultConversationTitle(title)) return t("new_chat");
    return title || t("new_chat");
  }

  function isDefaultConversationTitle(title) {
    const value = String(title || "").trim();
    return value === "" || value === "New chat" || value === "新对话";
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
      el.maxToolCalls.value = String(state.maxToolCalls);
      el.maxToolCalls.readOnly = !state.settingsWritable;
    }
    renderSettingsSummary(conv);
  }

  function renderSettingsSummary(conversation = activeConversation()) {
    if (!el.settingsSummary) return;
    const model = conversation.model || el.model.value || state.models[0] || t("no_model");
    const effort = labelEffort(conversation.thinking_effort || thinkingEffort(root), t);
    const avatars = `${avatarValue(conversation.user_avatar, defaultUserAvatar)} ${avatarValue(conversation.assistant_avatar, defaultAssistantAvatar)}`;
    el.settingsSummary.textContent = `${avatars} - ${model} - ${effort} - ${t("max_tool_calls_summary", { count: state.maxToolCalls })}`;
  }

  function syncSettingsDisabled() {
    const disabled = isConversationBusyById(state.activeId);
    for (const item of [el.model, el.systemPrompt, el.nickname, el.userAvatar, el.assistantAvatar, el.settingsSave]) {
      if (item) item.disabled = disabled;
    }
    if (el.maxToolCalls) {
      el.maxToolCalls.disabled = disabled;
      el.maxToolCalls.readOnly = !state.settingsWritable;
    }
    if (el.autoTitle) {
      el.autoTitle.disabled = disabled || !state.activeId;
      setTitle(el.autoTitle, state.titleGeneratingIds.has(state.activeId) ? t("title_generating") : t("generate_title"));
    }
    const submit = root.querySelector(".chat-actions button[type='submit']");
    if (submit) submit.disabled = disabled;
    if (el.rename) el.rename.disabled = disabled || !state.activeId;
    if (el.stop) el.stop.classList.toggle("hidden", !activeStream());
    root.querySelectorAll("[data-chat-avatar-value]").forEach((button) => { button.disabled = disabled; });
    root.querySelectorAll("input[name$='-thinking']").forEach((input) => { input.disabled = disabled; });
  }

  function localizeStatic() {
    root.querySelector(".chat-app-sidebar")?.setAttribute("aria-label", t("conversations"));
    root.querySelector(".chat-tool-toggles")?.setAttribute("aria-label", t("tool_controls"));
    el.processShell?.setAttribute("aria-label", t("process_timeline"));
    setTitle(el.new, t("new_chat"));
    for (const button of el.settingsOpen) {
      setTitle(button, t("chat_settings"));
      setButtonSpan(button, t("settings"));
    }
    setButtonSpan(el.settingsSummary, t("settings"));
    setCheckboxLabel(el.search, t("search"));
    setCheckboxLabel(el.read, t("read_web"));
    setCheckboxLabel(el.processToggle, t("process"));
    setTitle(el.processReopen, t("show_process_panel"));
    setButtonSpan(el.processReopen, t("process"));
    setTitle(el.autoTitle, state.titleGeneratingIds.has(state.activeId) ? t("title_generating") : t("generate_title"));
    setTitle(el.rename, t("rename"));
    setTitle(el.delete, t("delete"));
    setTitle(el.processTop, t("scroll_top"));
    setTitle(el.processBottom, t("scroll_bottom"));
    setTitle(el.processCollapse, t("collapse_process"));
    root.querySelector(".chat-process-title strong")?.replaceChildren(document.createTextNode(t("process")));
    root.querySelector(".chat-process-title span")?.replaceChildren(document.createTextNode(t("process_subtitle")));
    if (el.input) el.input.placeholder = t("ask_placeholder");
    if (el.stop) el.stop.textContent = t("stop");
    root.querySelector(".chat-actions button[type='submit']")?.replaceChildren(document.createTextNode(t("send")));
    root.querySelector(".chat-settings-head h2")?.replaceChildren(document.createTextNode(t("chat_settings")));
    root.querySelector(".chat-settings-head p")?.replaceChildren(document.createTextNode(t("saved_to_conversation")));
    setTitle(root.querySelector(".chat-settings-head [data-chat-settings-close]"), t("close_settings"));
    root.querySelector(".chat-avatar-settings")?.setAttribute("aria-label", t("avatar_settings"));
    setText(root.querySelector(".chat-avatar-card [data-chat-user-avatar]")?.previousElementSibling, t("user_avatar"));
    setText(root.querySelector(".chat-avatar-card [data-chat-assistant-avatar]")?.previousElementSibling, t("assistant_avatar"));
    if (el.userAvatar) el.userAvatar.setAttribute("aria-label", t("user_avatar"));
    if (el.assistantAvatar) el.assistantAvatar.setAttribute("aria-label", t("assistant_avatar"));
    renderAvatarPresets(root.querySelector("[data-chat-user-avatar]")?.closest(".chat-avatar-card")?.querySelector(".chat-avatar-picker"), "user", userAvatarPresets, t("user_avatar_presets"));
    renderAvatarPresets(root.querySelector("[data-chat-assistant-avatar]")?.closest(".chat-avatar-card")?.querySelector(".chat-avatar-picker"), "assistant", assistantAvatarPresets, t("assistant_avatar_presets"));
    setLeadingText(root.querySelector(".chat-settings-dialog .chat-model-field"), t("model"));
    setText(root.querySelector(".chat-settings-thinking legend"), t("thinking"));
    root.querySelectorAll(".chat-settings-thinking label").forEach((label) => {
      const value = label.querySelector("input")?.value || "medium";
      setTrailingText(label, labelEffort(value, t));
    });
    setLeadingText(root.querySelector("[data-chat-system-prompt]")?.closest("label"), t("system_prompt"));
    if (el.systemPrompt) el.systemPrompt.placeholder = t("system_prompt_placeholder");
    setLeadingText(root.querySelector("[data-chat-nickname]")?.closest("label"), t("my_nickname"));
    if (el.nickname) el.nickname.placeholder = "";
    setLeadingText(root.querySelector("[data-chat-max-tool-calls]")?.closest("label"), t("max_tool_calls"));
    setText(root.querySelector("[data-chat-default-system-title]"), t("default_system_prompt"));
    setText(root.querySelector("[data-chat-default-system-hint]"), t("default_system_prompt_hint"));
    root.querySelector(".chat-default-system-prompt")?.setAttribute("aria-label", t("default_system_prompt"));
    if (el.settingsCancel) el.settingsCancel.textContent = t("cancel");
    if (el.settingsSave) el.settingsSave.textContent = t("save");
  }

  el.list.addEventListener("click", (event) => {
    const renameButton = event.target.closest("[data-chat-rename-id]");
    if (renameButton) {
      renameConversation(Number(renameButton.dataset.chatRenameId)).catch((error) => showToast(error.message, "error"));
      return;
    }
    const deleteButton = event.target.closest("[data-chat-delete-id]");
    if (deleteButton) {
      deleteConversation(Number(deleteButton.dataset.chatDeleteId)).catch((error) => showToast(error.message, "error"));
      return;
    }
    const button = event.target.closest("[data-chat-open]");
    if (button) openConversation(Number(button.dataset.chatOpen)).catch((error) => showToast(error.message, "error"));
  });
  el.new.addEventListener("click", () => createConversation().catch((error) => showToast(error.message, "error")));
  el.autoTitle?.addEventListener("click", () => generateConversationTitle());
  el.rename.addEventListener("click", () => renameConversation().catch((error) => showToast(error.message, "error")));
  el.delete.addEventListener("click", () => deleteConversation().catch((error) => showToast(error.message, "error")));
  el.settingsOpen.forEach((button) => button.addEventListener("click", () => openSettings().catch((error) => showToast(error.message, "error"))));
  el.settingsForm.addEventListener("submit", (event) => saveSettings(event).catch((error) => showToast(error.message, "error")));
  el.settingsCancel.addEventListener("click", closeSettings);
  el.settingsModal.addEventListener("click", (event) => {
    if (event.target.closest("[data-chat-settings-close]")) closeSettings();
  });
  el.form.addEventListener("submit", sendMessage);
  el.stop.addEventListener("click", () => abortConversationStream(state.activeId));
  el.processToggle.addEventListener("change", () => {
    if (!el.processToggle.checked) state.processCollapsed = false;
    renderProcess();
  });
  el.processCollapse.addEventListener("click", () => {
    state.processCollapsed = !state.processCollapsed;
    syncProcessPanel();
    if (!state.processCollapsed) scrollProcess("bottom");
  });
  el.processReopen?.addEventListener("click", openProcessPanel);
  el.processTop.addEventListener("click", () => scrollProcess("top"));
  el.processBottom.addEventListener("click", () => scrollProcess("bottom"));
  el.input.addEventListener("input", autosizeInput);
  el.input.addEventListener("keydown", (event) => {
    if (event.key !== "Enter" || event.shiftKey || event.isComposing) return;
    event.preventDefault();
    el.form.requestSubmit();
  });
  root.addEventListener("click", (event) => {
    const avatarButton = event.target.closest("[data-chat-avatar-value]");
    if (avatarButton) {
      const target = avatarButton.dataset.chatAvatarTarget === "assistant" ? el.assistantAvatar : el.userAvatar;
      if (target && !target.disabled) {
        target.value = avatarButton.dataset.chatAvatarValue || "";
        target.focus();
      }
      return;
    }
    const button = event.target.closest("[data-copy-code]");
    if (!button) return;
    copyText(button.nextElementSibling?.textContent || "").then(() => showToast(t("copied")));
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !el.settingsModal.classList.contains("hidden")) closeSettings();
  });

  autosizeInput();
  load();

  function autosizeInput() {
    el.input.style.height = "auto";
    el.input.style.height = `${Math.min(el.input.scrollHeight, 220)}px`;
  }
}

async function readSSE(response, onEvent) {
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split("\n\n");
    buffer = parts.pop() || "";
    for (const part of parts) {
      const event = parseSSE(part);
      if (event) onEvent(event.event, event.data);
    }
  }
}

function parseSSE(raw) {
  const lines = raw.split(/\r?\n/);
  let event = "message";
  let data = "";
  for (const line of lines) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    if (line.startsWith("data:")) data += line.slice(5).trim();
  }
  if (!data) return null;
  return { event, data: JSON.parse(data) };
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
    case "off":
      return t("off");
    case "low":
      return t("low");
    case "high":
      return t("high");
    default:
      return t("medium");
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
  for (const [name, value] of Object.entries(values)) {
    text = text.replaceAll(`{${name}}`, String(value));
  }
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

function setButtonSpan(button, text) {
  const span = button?.querySelector("span");
  if (span) span.textContent = text;
}

function setCheckboxLabel(input, text) {
  const label = input?.closest("label");
  if (label) setTrailingText(label, text);
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
      node.textContent = node.previousSibling ? ` ${text}` : text;
      return;
    }
  }
  label.appendChild(document.createTextNode(` ${text}`));
}

function renderAvatarPresets(picker, target, values, label) {
  if (!picker) return;
  picker.setAttribute("aria-label", label);
  picker.innerHTML = values.map((value) => `
    <button type="button" class="chat-avatar-preset" data-chat-avatar-target="${esc(target)}" data-chat-avatar-value="${esc(value)}">${esc(value)}</button>
  `).join("");
}

function eventsFromMessages(messages) {
  return messages.flatMap((message) => {
    if (!message.metadata) return [];
    try {
      const metadata = JSON.parse(message.metadata);
      return metadata.events || [];
    } catch {
      return [];
    }
  });
}

function usageFromMessage(message) {
  if (!message?.metadata) return null;
  try {
    const metadata = typeof message.metadata === "string" ? JSON.parse(message.metadata) : message.metadata;
    return normalizeUsage(metadata.usage);
  } catch {
    return null;
  }
}

function mergeMessageUsage(message, usage) {
  const normalized = normalizeUsage(usage);
  if (!normalized) return;
  let metadata = {};
  try {
    metadata = message.metadata ? (typeof message.metadata === "string" ? JSON.parse(message.metadata) : message.metadata) : {};
  } catch {
    metadata = {};
  }
  metadata.usage = normalized;
  message.metadata = JSON.stringify(metadata);
}

function normalizeUsage(usage) {
  if (!usage || typeof usage !== "object") return null;
  return {
    input_tokens: Number(usage.input_tokens || 0),
    output_tokens: Number(usage.output_tokens || 0),
    cache_read_tokens: Number(usage.cache_read_tokens || 0),
    cache_creation_tokens: Number(usage.cache_creation_tokens || 0),
  };
}

function normalizeMaxToolCalls(value) {
  const parsed = Number.parseInt(String(value ?? 6), 10);
  if (!Number.isFinite(parsed)) return 6;
  return Math.min(20, Math.max(0, parsed));
}

function renderMarkdown(markdown, copyLabel = "Copy") {
  const text = String(markdown || "");
  const blocks = [];
  let rest = text.replace(/```([a-zA-Z0-9_-]*)\n([\s\S]*?)```/g, (_, lang, code) => {
    const token = `\u0000CODE${blocks.length}\u0000`;
    blocks.push({ lang, code });
    return token;
  });
  rest = esc(rest)
    .replace(/^### (.*)$/gm, "<h3>$1</h3>")
    .replace(/^## (.*)$/gm, "<h2>$1</h2>")
    .replace(/^# (.*)$/gm, "<h1>$1</h1>")
    .replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>")
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')
    .replace(/(^|\s)(https?:\/\/[^\s<]+)/g, '$1<a href="$2" target="_blank" rel="noopener noreferrer">$2</a>');
  rest = rest.split(/\n{2,}/).map((paragraph) => {
    if (/^<h[1-3]>/.test(paragraph) || paragraph.includes("\u0000CODE")) return paragraph;
    const lines = paragraph.split(/\n/);
    if (lines.every((line) => /^[-*] /.test(line))) {
      return `<ul>${lines.map((line) => `<li>${line.slice(2)}</li>`).join("")}</ul>`;
    }
    return `<p>${lines.join("<br>")}</p>`;
  }).join("");
  blocks.forEach((block, index) => {
    const code = esc(block.code.replace(/\n$/, ""));
    const html = `<div class="code-block"><button type="button" class="secondary" data-copy-code>${esc(copyLabel)}</button><pre><code>${code}</code></pre></div>`;
    rest = rest.replace(`\u0000CODE${index}\u0000`, html);
  });
  return rest;
}

function renderSources(message, label = "Sources") {
  const urls = new Set((message.content || "").match(/https?:\/\/[^\s)]+/g) || []);
  if (!urls.size) return "";
  return `<details class="chat-sources"><summary>${esc(label)}</summary>${Array.from(urls).map((url) =>
    `<a href="${esc(url)}" target="_blank" rel="noopener noreferrer">${esc(url)}</a>`).join("")}</details>`;
}

function compact(value, limit = 1200) {
  const text = String(value || "");
  return text.length > limit ? `${text.slice(0, limit)}\n[truncated]` : text;
}
