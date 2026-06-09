package admin

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"
)

const langCookie = "gateway_lang"

var supportedLangs = map[string]bool{
	"en":    true,
	"zh-CN": true,
}

var translations = map[string]map[string]string{
	"en": {
		"app.title":               "TokenFlow",
		"title.login":             "Login",
		"title.setup":             "Setup",
		"title.dashboard":         "TokenFlow Admin",
		"language":                "Language",
		"lang.en":                 "English",
		"lang.zh":                 "简体中文",
		"username":                "Username",
		"password":                "Password",
		"login":                   "Log in",
		"logout":                  "Log out",
		"initial_setup":           "Initial setup",
		"create_admin":            "Create admin",
		"overview":                "Overview",
		"token_usage":             "Token usage",
		"usage_range":             "Usage range",
		"last_24_hours":           "24h",
		"last_7_days":             "7 days",
		"token_usage_empty":       "No token usage in this range.",
		"total_tokens":            "Total tokens",
		"details":                 "Details",
		"model_token_details":     "Model token details",
		"model_token_chart":       "Model token chart",
		"detail_scope_provider":   "Provider",
		"detail_scope_key":        "Distribution key",
		"close":                   "Close",
		"clear":                   "Clear",
		"api_addresses":           "API addresses",
		"api_base":                "API base",
		"openai_chat":             "OpenAI chat",
		"openai_models":           "OpenAI models",
		"anthropic_messages":      "Anthropic messages",
		"anthropic_models":        "Anthropic models",
		"legacy_anthropic":        "Legacy Anthropic messages",
		"providers":               "Providers",
		"add_provider":            "Add provider",
		"name":                    "Name",
		"protocol":                "Protocol",
		"base_api":                "Base API",
		"api_key":                 "API Key",
		"api_key_placeholder":     "leave blank to keep existing key",
		"default_model":           "Default model",
		"supported_models":        "Supported models",
		"models":                  "Models",
		"models_placeholder":      "One model per line or comma-separated. The default model is included automatically.",
		"enabled":                 "Enabled",
		"disabled":                "Disabled",
		"default":                 "Default",
		"save":                    "Save",
		"cancel":                  "Cancel",
		"model_mappings":          "Model mappings",
		"add_mapping":             "Add mapping",
		"client_model":            "Client model",
		"provider":                "Provider",
		"upstream_model":          "Upstream model",
		"distribution_keys":       "Distribution keys",
		"distribution_key":        "Key",
		"create_key":              "Create key",
		"recent_requests":         "Recent requests",
		"logs_search":             "Search requests",
		"logs_search_placeholder": "Key / provider / model",
		"requests":                "Requests",
		"input_tokens":            "Input tokens",
		"output_tokens":           "Output tokens",
		"active_keys":             "Active keys",
		"model":                   "Model",
		"status":                  "Status",
		"prefix":                  "Prefix",
		"tokens":                  "Tokens",
		"cache_hit_rate":          "Cache hit",
		"cache_read_tokens":       "Cache read tokens",
		"cache_creation_tokens":   "Cache write tokens",
		"last_used":               "Last used",
		"previous_page":           "Previous",
		"next_page":               "Next",
		"time":                    "Time",
		"latency":                 "Latency",
		"stream":                  "Stream",
		"yes":                     "yes",
		"no":                      "no",
		"edit":                    "Edit",
		"delete":                  "Delete",
		"empty":                   "No records yet.",
		"loading":                 "Loading...",
		"new_key":                 "New key",
		"reset_key":               "Regenerate",
		"reset_key_confirm":       "Regenerating this key will immediately invalidate the old key. The new key is shown only once. Continue?",
		"reset_key_stats":         "Reset stats",
		"reset_key_stats_confirm": "Reset this key's request and token statistics? The key itself will not change.",
		"request_failed":          "Request failed",
		"delete_provider_confirm": "Delete this provider?",
		"delete_mapping_confirm":  "Delete this mapping?",
		"delete_key_confirm":      "Delete this key?",
		"setup_password_error":    "Username is required and password must be at least 8 characters.",
		"login_invalid":           "Invalid username or password.",
		"provider_required":       "name, base_api, and default_model are required",
		"protocol_required":       "protocol must be openai or anthropic",
		"api_key_required":        "api_key is required",
		"invalid_json":            "invalid JSON body",
		"csrf_invalid":            "invalid CSRF token",
		"login_required":          "login required",
		"not_found":               "not found",
		"invalid_usage_range":     "range must be 24h or 7d",
		"invalid_detail_scope":    "scope must be provider or key",
		"detail_id_required":      "id is required",
		"unsupported_language":    "unsupported language",
		"provider_select_empty":   "Create a provider first.",
	},
	"zh-CN": {
		"app.title":               "TokenFlow",
		"title.login":             "登录",
		"title.setup":             "初始化",
		"title.dashboard":         "TokenFlow 管理",
		"language":                "语言",
		"lang.en":                 "English",
		"lang.zh":                 "简体中文",
		"username":                "用户名",
		"password":                "密码",
		"login":                   "登录",
		"logout":                  "退出登录",
		"initial_setup":           "初始化设置",
		"create_admin":            "创建管理员",
		"overview":                "概览",
		"token_usage":             "Token 使用趋势",
		"usage_range":             "使用范围",
		"last_24_hours":           "近 24 小时",
		"last_7_days":             "近 7 天",
		"token_usage_empty":       "所选时间范围内暂无 Token 使用。",
		"total_tokens":            "总 Token",
		"details":                 "详情",
		"model_token_details":     "模型 Token 明细",
		"model_token_chart":       "模型 Token 图表",
		"detail_scope_provider":   "上游供应商",
		"detail_scope_key":        "分发 Key",
		"close":                   "关闭",
		"clear":                   "清除",
		"api_addresses":           "API 地址",
		"api_base":                "API 基础地址",
		"openai_chat":             "OpenAI 对话",
		"openai_models":           "OpenAI 模型列表",
		"anthropic_messages":      "Anthropic 消息",
		"anthropic_models":        "Anthropic 模型列表",
		"legacy_anthropic":        "旧版 Anthropic 消息",
		"providers":               "上游供应商",
		"add_provider":            "添加供应商",
		"name":                    "名称",
		"protocol":                "协议",
		"base_api":                "Base API",
		"api_key":                 "API Key",
		"api_key_placeholder":     "留空则保留现有 Key",
		"default_model":           "默认模型",
		"supported_models":        "支持模型",
		"models":                  "模型",
		"models_placeholder":      "每行一个模型，也可以用逗号分隔。默认模型会自动加入列表。",
		"enabled":                 "已启用",
		"disabled":                "已禁用",
		"default":                 "默认",
		"save":                    "保存",
		"cancel":                  "取消",
		"model_mappings":          "模型映射",
		"add_mapping":             "添加映射",
		"client_model":            "客户端模型",
		"provider":                "供应商",
		"upstream_model":          "上游模型",
		"distribution_keys":       "分发 Key",
		"distribution_key":        "Key 名称",
		"create_key":              "创建 Key",
		"recent_requests":         "最近请求",
		"logs_search":             "搜索请求",
		"logs_search_placeholder": "Key / 供应商 / 模型",
		"requests":                "请求数",
		"input_tokens":            "输入 Token",
		"output_tokens":           "输出 Token",
		"active_keys":             "启用 Key",
		"model":                   "模型",
		"status":                  "状态",
		"prefix":                  "前缀",
		"tokens":                  "Token",
		"cache_hit_rate":          "缓存命中率",
		"cache_read_tokens":       "缓存命中 Token",
		"cache_creation_tokens":   "缓存写入 Token",
		"last_used":               "最后使用",
		"previous_page":           "上一页",
		"next_page":               "下一页",
		"time":                    "时间",
		"latency":                 "延迟",
		"stream":                  "流式",
		"yes":                     "是",
		"no":                      "否",
		"edit":                    "编辑",
		"delete":                  "删除",
		"empty":                   "暂无记录。",
		"loading":                 "加载中...",
		"new_key":                 "新 Key",
		"reset_key":               "重新生成",
		"reset_key_confirm":       "重新生成后旧 Key 会立即失效，新 Key 只显示一次。确定继续？",
		"reset_key_stats":         "重置统计",
		"reset_key_stats_confirm": "确定清零这个 Key 的请求数和 Token 统计？Key 本身不会改变。",
		"request_failed":          "请求失败",
		"delete_provider_confirm": "确定删除这个供应商？",
		"delete_mapping_confirm":  "确定删除这个模型映射？",
		"delete_key_confirm":      "确定删除这个 Key？",
		"setup_password_error":    "用户名不能为空，密码至少需要 8 个字符。",
		"login_invalid":           "用户名或密码错误。",
		"provider_required":       "名称、base_api 和默认模型不能为空",
		"protocol_required":       "协议必须是 openai 或 anthropic",
		"api_key_required":        "api_key 不能为空",
		"invalid_json":            "请求体不是有效 JSON",
		"csrf_invalid":            "CSRF Token 无效",
		"login_required":          "需要先登录",
		"not_found":               "未找到",
		"invalid_usage_range":     "range 必须是 24h 或 7d",
		"invalid_detail_scope":    "scope 必须是 provider 或 key",
		"detail_id_required":      "id 不能为空",
		"unsupported_language":    "不支持的语言",
		"provider_select_empty":   "请先创建供应商。",
	},
}

func languageFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(langCookie); err == nil {
		if lang, ok := normalizeLang(cookie.Value); ok {
			return lang
		}
	}
	header := strings.ToLower(r.Header.Get("Accept-Language"))
	if strings.Contains(header, "zh") {
		return "zh-CN"
	}
	return "en"
}

func normalizeLang(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en", "en-us", "en-gb":
		return "en", true
	case "zh", "zh-cn", "zh_cn", "cn", "zh-hans":
		return "zh-CN", true
	default:
		return "", false
	}
}

func setLanguageCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     langCookie,
		Value:    lang,
		Path:     "/admin",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

func safeAdminNext(next string) string {
	if next == "/admin" || strings.HasPrefix(next, "/admin/") || strings.HasPrefix(next, "/admin?") {
		return next
	}
	return "/admin"
}

func tr(lang, key string) string {
	if values, ok := translations[lang]; ok {
		if value, ok := values[key]; ok {
			return value
		}
	}
	if value, ok := translations["en"][key]; ok {
		return value
	}
	return key
}

func i18nJSON(lang string) template.JS {
	values := map[string]string{}
	for key, value := range translations["en"] {
		values[key] = value
	}
	for key, value := range translations[lang] {
		values[key] = value
	}
	raw, _ := json.Marshal(values)
	return template.JS(raw)
}

func jsonString(value string) template.JS {
	raw, _ := json.Marshal(value)
	return template.JS(raw)
}
