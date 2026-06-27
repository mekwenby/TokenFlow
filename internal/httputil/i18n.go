package httputil

import (
	"net/http"
	"strings"
	"time"
)

// NormalizeLang normalizes a language tag to "en" or "zh-CN".
// Returns false if the value is not a recognized language.
func NormalizeLang(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en", "en-us", "en-gb":
		return "en", true
	case "zh", "zh-cn", "zh_cn", "cn", "zh-hans":
		return "zh-CN", true
	default:
		return "", false
	}
}

// LanguageFromRequest detects the user's language preference.
// Checks the named cookie first, then falls back to Accept-Language header.
// Returns "en" by default.
func LanguageFromRequest(r *http.Request, cookieName string) string {
	if cookie, err := r.Cookie(cookieName); err == nil {
		if lang, ok := NormalizeLang(cookie.Value); ok {
			return lang
		}
	}
	header := strings.ToLower(r.Header.Get("Accept-Language"))
	if strings.Contains(header, "zh") {
		return "zh-CN"
	}
	return "en"
}

// SetLanguageCookie sets a language preference cookie with a 1-year expiry.
func SetLanguageCookie(w http.ResponseWriter, name, path, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    lang,
		Path:     path,
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

// SafeNextPath validates that next is under the given prefix. Returns prefix if not.
func SafeNextPath(next, prefix string) string {
	if next == prefix || strings.HasPrefix(next, prefix+"/") || strings.HasPrefix(next, prefix+"?") {
		return next
	}
	return prefix
}
