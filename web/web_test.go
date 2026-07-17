package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestNavigationModuleExportsAdminAndAccountInitializers(t *testing.T) {
	nav, err := fs.ReadFile(Static, "static/core/nav.js")
	if err != nil {
		t.Fatal(err)
	}
	account, err := fs.ReadFile(Static, "static/account/app.js")
	if err != nil {
		t.Fatal(err)
	}

	for _, exported := range []string{"export function initSectionNav", "export function initViewNav"} {
		if !strings.Contains(string(nav), exported) {
			t.Fatalf("navigation module is missing %q", exported)
		}
	}
	if !strings.Contains(string(account), `import { initSectionNav } from "../core/nav.js"`) {
		t.Fatal("account application no longer imports the compatible section navigator")
	}
}

func TestHomeStylesAreEmbedded(t *testing.T) {
	home, err := fs.ReadFile(Static, "static/css/home.css")
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		".home-page", ".route-console", "--home-route-cycle: 7.2s",
		`[data-motion-state=running]`, "animation-play-state:paused",
		"@media(hover:hover)and (pointer:fine)", "prefers-reduced-motion",
		"home-packet-inbound-y", "home-packet-outbound-y", ".home-chat-showcase",
		"home-chat-stream", "home-chat-pulse",
	} {
		if !strings.Contains(string(home), expected) {
			t.Fatalf("home stylesheet is missing %q", expected)
		}
	}
}

func TestHomeMotionControllerIsEmbedded(t *testing.T) {
	home, err := fs.ReadFile(Static, "static/home/app.js")
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		"IntersectionObserver", "visibilitychange", "prefers-reduced-motion: reduce",
		"shouldRunMotion", "homeMotionState", `target.dataset.motionState`,
		`document.querySelectorAll("[data-home-motion]")`,
	} {
		if !strings.Contains(string(home), expected) {
			t.Fatalf("home motion controller is missing %q", expected)
		}
	}
	for _, forbidden := range []string{"requestAnimationFrame", `addEventListener("scroll"`, "setInterval"} {
		if strings.Contains(string(home), forbidden) {
			t.Fatalf("home motion controller should not use %q", forbidden)
		}
	}
}

func TestChineseChatCopyUsesLocalizedBrand(t *testing.T) {
	chat, err := fs.ReadFile(Static, "static/chat/app.js")
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{"给一念通流 TokenFlow 发送消息", "一念通流 TokenFlow 始终应用默认提示词"} {
		if !strings.Contains(string(chat), expected) {
			t.Fatalf("chat application is missing %q", expected)
		}
	}
}

func TestChatSendsBrowserTimeZone(t *testing.T) {
	chat, err := fs.ReadFile(Static, "static/chat/app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(chat)
	for _, expected := range []string{`Intl.DateTimeFormat().resolvedOptions().timeZone`, `time_zone: browserTimeZone`} {
		if !strings.Contains(source, expected) {
			t.Fatalf("chat application is missing %q", expected)
		}
	}
}

func TestChatDelaysAutomaticTitleUntilStreamCleanup(t *testing.T) {
	chat, err := fs.ReadFile(Static, "static/chat/app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(chat)
	for _, expected := range []string{
		`if (stream.responseDone && stream.autoTitle) scheduleConversationTitle(conversationId);`,
		`await generateConversationTitle(id, false, true);`,
		`cancelScheduledAutoTitle(conversationId);`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("chat application is missing delayed title behavior %q", expected)
		}
	}
	if strings.Contains(source, `if (stream.autoTitle) await generateConversationTitle(conversationId, false, true);`) {
		t.Fatal("chat application still generates the automatic title immediately after SSE")
	}
}

func TestChatUsesExpandableTokenUsageWithAccessibleIcons(t *testing.T) {
	chat, err := fs.ReadFile(Static, "static/chat/app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(chat)
	for _, expected := range []string{
		`class="chat-message-commands"`,
		`<details class="chat-message-usage">`,
		`usageSummaryMetric("icon-arrow-up", inputLabel`,
		`usageSummaryMetric("icon-arrow-down", outputLabel`,
		`class="icon chat-usage-chevron"`,
		`class="chat-sr-only"`,
		`aria-label=`,
		`tokenUsageDetailKeys(usage)`,
		`cache_read_tokens: "缓存命中"`,
		`t(estimated ? "estimated" : "exact")`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("chat application is missing token usage behavior %q", expected)
		}
	}
	if strings.Contains(source, `const actions = !isUser && actionButtons ?`) {
		t.Fatal("chat actions still place token usage in the button grid")
	}
	if strings.Contains(source, `usage.input_tokens + usage.output_tokens + usage.cache_read_tokens`) {
		t.Fatal("chat token total still double-counts cache usage")
	}
}

func TestChatTokenUsageHasMobileLayoutRules(t *testing.T) {
	styles, err := fs.ReadFile(Static, "static/css/chat.css")
	if err != nil {
		t.Fatal(err)
	}
	source := string(styles)
	for _, expected := range []string{
		`.chat-message-commands`,
		`.chat-usage-chevron`,
		`align-items: center`,
		`.chat-message-usage[open]`,
		`grid-column: 1 / -1`,
		`grid-template-columns: 1fr`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("chat stylesheet is missing mobile token usage rule %q", expected)
		}
	}
}

func TestChatMobileStylesPreventIOSFocusZoomAndRespectSafeAreas(t *testing.T) {
	styles, err := fs.ReadFile(Static, "static/css/chat.css")
	if err != nil {
		t.Fatal(err)
	}
	source := string(styles)
	for _, expected := range []string{
		`-webkit-text-size-adjust: 100%`,
		`height: var(--chat-viewport-height, 100dvh)`,
		`transform: translateY(var(--chat-viewport-offset-top, 0px))`,
		`grid-template-rows: calc(56px + env(safe-area-inset-top))`,
		`.chat-page input:not([type="checkbox"]):not([type="radio"])`,
		`font-size: 16px`,
		`max(8px, env(safe-area-inset-bottom))`,
		`max(12px, env(safe-area-inset-left))`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("chat stylesheet is missing iPhone compatibility rule %q", expected)
		}
	}
	chat, err := fs.ReadFile(Static, "static/chat/app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`window.visualViewport`, `--chat-viewport-height`, `--chat-viewport-offset-top`, `style.scrollBehavior = "auto"`, `window.scrollTo(0, 0)`, `focus({ preventScroll: true })`} {
		if !strings.Contains(string(chat), expected) {
			t.Fatalf("chat application is missing iPhone viewport recovery %q", expected)
		}
	}
	for _, forbidden := range []string{`maximum-scale`, `user-scalable`} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("chat stylesheet should not disable user zoom with %q", forbidden)
		}
	}
}

func TestChatUsesAccessibleMarkdownTables(t *testing.T) {
	chat, err := fs.ReadFile(Static, "static/chat/app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`from "./markdown.js"`, `t("markdown_table")`} {
		if !strings.Contains(string(chat), expected) {
			t.Fatalf("chat application is missing Markdown table integration %q", expected)
		}
	}

	markdown, err := fs.ReadFile(Static, "static/chat/markdown.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(markdown)
	for _, expected := range []string{
		`<div class="markdown-table-wrap" role="region"`,
		`tabindex="0"`, `<table class="markdown-table">`, `<thead><tr>`, `<tbody>`,
		`/^:?-{3,}:?$/`, `character === "\\" && source[index + 1] === "|"`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("Markdown renderer is missing table behavior %q", expected)
		}
	}
}

func TestChatMarkdownTablesOverrideGlobalTableLayout(t *testing.T) {
	styles, err := fs.ReadFile(Static, "static/css/chat.css")
	if err != nil {
		t.Fatal(err)
	}
	source := string(styles)
	for _, expected := range []string{
		`.markdown-table-wrap`, `overflow-x: auto`, `overscroll-behavior-inline: contain`,
		`.markdown-body .markdown-table`, `width: max-content`, `min-width: 100%`,
		`position: static`, `white-space: normal`, `overflow-wrap: anywhere`,
		`.markdown-align-center`, `.markdown-align-right`,
		`max-width: calc(100vw - 65px)`, `max-width: min(18rem, 75vw)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("chat stylesheet is missing Markdown table rule %q", expected)
		}
	}
}

func TestChatCodeBlocksSupportHighlightingAndSandboxedHTMLPreview(t *testing.T) {
	markdown, err := fs.ReadFile(Static, "static/chat/markdown.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`from "./highlight.bundle.js"`, `role="tablist"`, `data-code-view="preview"`,
		`data-reload-preview`, `class="hljs`, `icon-copy`,
	} {
		if !strings.Contains(string(markdown), expected) {
			t.Fatalf("Markdown code block is missing %q", expected)
		}
	}

	preview, err := fs.ReadFile(Static, "static/chat/html-preview.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`script-src 'unsafe-inline'`, `connect-src 'none'`, `frame-src 'none'`,
		`form-action 'none'`, `img-src https: data: blob:`, `Content-Security-Policy`,
		`frame.setAttribute("sandbox", "allow-scripts")`,
		`frame.setAttribute("referrerpolicy", "no-referrer")`,
	} {
		if !strings.Contains(string(preview), expected) {
			t.Fatalf("HTML preview policy is missing %q", expected)
		}
	}

	chat, err := fs.ReadFile(Static, "static/chat/app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`from "./html-preview.js"`, `htmlPreviewOptions`, `setCodeBlockView(`,
		`loadHTMLPreview(`, `t("preview_navigation_blocked")`, `[data-code-pane="code"] code`,
	} {
		if !strings.Contains(string(chat), expected) {
			t.Fatalf("Chat HTML preview integration is missing %q", expected)
		}
	}

	if _, err := fs.Stat(Static, "static/chat/highlight.bundle.js"); err != nil {
		t.Fatalf("embedded syntax highlighter: %v", err)
	}
}

func TestChatCodePreviewAndToastHaveResponsiveStyles(t *testing.T) {
	styles, err := fs.ReadFile(Static, "static/css/chat.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`.code-block-toolbar`, `.code-block-tabs`, `.hljs-keyword`, `.html-preview-pane`,
		`height: 360px`, `height: 300px`, `resize: vertical`, `resize: none`,
		`.chat-page .toast`, `left: 50%`, `top: max(12px, env(safe-area-inset-top))`,
		`transform: translate(-50%, -8px)`, `max-width: calc(100vw - 24px)`,
	} {
		if !strings.Contains(string(styles), expected) {
			t.Fatalf("Chat code preview or toast style is missing %q", expected)
		}
	}
}

func TestChatSourcesAndProcessUseAlignedDisclosureChevrons(t *testing.T) {
	chat, err := fs.ReadFile(Static, "static/chat/app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(chat)
	if strings.Count(source, `class="icon chat-details-chevron" aria-hidden="true"`) != 2 {
		t.Fatal("sources and process should each render the shared accessible disclosure chevron")
	}
	for _, expected := range []string{`class="chat-process-details"`, `class="chat-sources"`, `#icon-chevron-right`} {
		if !strings.Contains(source, expected) {
			t.Fatalf("Chat disclosure markup is missing %q", expected)
		}
	}

	styles, err := fs.ReadFile(Static, "static/css/chat.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(styles)
	for _, expected := range []string{
		`.chat-sources > summary`, `.chat-process-details > summary`, `display: inline-flex`,
		`align-items: center`, `summary::-webkit-details-marker`, `.chat-details-chevron`,
		`transform: rotate(90deg)`, `.chat-process-events`, `border-left: 2px solid var(--chat-border)`,
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("Chat disclosure styles are missing %q", expected)
		}
	}
}
