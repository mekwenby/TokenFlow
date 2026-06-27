package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"tokenflow/internal/store"
)

type RouteConfig struct {
	BasePath         string
	Service          *Service
	Store            *store.Store
	Owner            func(*http.Request) (store.ChatOwner, bool)
	RequireCSRF      func(http.ResponseWriter, *http.Request) bool
	SettingsWritable bool
}

type routeHandler struct {
	cfg RouteConfig
}

type conversationPayload struct {
	Title           *string `json:"title"`
	Model           *string `json:"model"`
	ThinkingEffort  *string `json:"thinking_effort"`
	SystemPrompt    *string `json:"system_prompt"`
	Nickname        *string `json:"nickname"`
	UserAvatar      *string `json:"user_avatar"`
	AssistantAvatar *string `json:"assistant_avatar"`
}

func RegisterRoutes(r chi.Router, cfg RouteConfig) {
	h := routeHandler{cfg: cfg}
	base := strings.TrimRight(cfg.BasePath, "/")
	r.Get(base+"/models", h.models)
	r.Get(base+"/settings", h.settings)
	if cfg.SettingsWritable {
		r.Patch(base+"/settings", h.settings)
	}
	r.Get(base+"/conversations", h.conversations)
	r.Post(base+"/conversations", h.conversations)
	r.Get(base+"/conversations/{conversationID}", h.conversation)
	r.Patch(base+"/conversations/{conversationID}", h.conversation)
	r.Delete(base+"/conversations/{conversationID}", h.conversation)
	r.Post(base+"/conversations/{conversationID}/title", h.title)
	r.Post(base+"/conversations/{conversationID}/messages", h.messages)
}

func (h routeHandler) models(w http.ResponseWriter, r *http.Request) {
	models, err := h.cfg.Service.Models(r.Context())
	maxToolCalls := store.DefaultChatMaxToolCalls
	if err == nil {
		maxToolCalls, err = h.cfg.Store.ChatMaxToolCalls(r.Context())
	}
	writeChatResult(w, map[string]any{
		"models":                models,
		"default_system_prompt": h.cfg.Service.DefaultSystemPrompt(),
		"max_tool_calls":        maxToolCalls,
	}, err)
}

func (h routeHandler) settings(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.owner(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		maxToolCalls, err := h.cfg.Store.ChatMaxToolCalls(r.Context())
		writeChatResult(w, map[string]any{"max_tool_calls": maxToolCalls}, err)
	case http.MethodPatch:
		if !h.cfg.SettingsWritable || owner.Type != store.ChatOwnerAdmin {
			writeChatError(w, http.StatusForbidden, "admin required")
			return
		}
		if !h.cfg.RequireCSRF(w, r) {
			return
		}
		var payload struct {
			MaxToolCalls int `json:"max_tool_calls"`
		}
		if !decodeChatPayload(w, r, &payload) {
			return
		}
		maxToolCalls, err := h.cfg.Store.UpdateChatMaxToolCalls(r.Context(), payload.MaxToolCalls)
		writeChatResult(w, map[string]any{"max_tool_calls": maxToolCalls}, err)
	}
}

func (h routeHandler) conversations(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.owner(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		conversations, err := h.cfg.Store.ListChatConversations(r.Context(), owner)
		writeChatResult(w, map[string]any{"items": conversations}, err)
	case http.MethodPost:
		if !h.cfg.RequireCSRF(w, r) {
			return
		}
		var payload struct {
			Title           string `json:"title"`
			Model           string `json:"model"`
			ThinkingEffort  string `json:"thinking_effort"`
			SystemPrompt    string `json:"system_prompt"`
			Nickname        string `json:"nickname"`
			UserAvatar      string `json:"user_avatar"`
			AssistantAvatar string `json:"assistant_avatar"`
		}
		if !decodeChatPayload(w, r, &payload) {
			return
		}
		conv, err := h.cfg.Store.CreateChatConversation(r.Context(), owner, payload.Title, payload.Model, payload.ThinkingEffort, payload.SystemPrompt, payload.Nickname, payload.UserAvatar, payload.AssistantAvatar)
		writeChatResult(w, conv, err)
	}
}

func (h routeHandler) conversation(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.owner(w, r)
	if !ok {
		return
	}
	id := conversationID(r)
	switch r.Method {
	case http.MethodGet:
		conv, err := h.cfg.Store.ChatConversation(r.Context(), owner, id)
		if err != nil {
			writeChatResult(w, nil, err)
			return
		}
		messages, err := h.cfg.Store.ChatMessages(r.Context(), owner, id)
		writeChatResult(w, map[string]any{"conversation": conv, "messages": messages}, err)
	case http.MethodPatch:
		if !h.cfg.RequireCSRF(w, r) {
			return
		}
		var payload conversationPayload
		if !decodeChatPayload(w, r, &payload) {
			return
		}
		conv, err := h.cfg.Store.UpdateChatConversation(r.Context(), owner, id, payload.Title, payload.Model, payload.ThinkingEffort, payload.SystemPrompt, payload.Nickname, payload.UserAvatar, payload.AssistantAvatar)
		writeChatResult(w, conv, err)
	case http.MethodDelete:
		if !h.cfg.RequireCSRF(w, r) {
			return
		}
		writeChatResult(w, map[string]bool{"ok": true}, h.cfg.Store.DeleteChatConversation(r.Context(), owner, id))
	}
}

func (h routeHandler) messages(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.owner(w, r)
	if !ok {
		return
	}
	if !h.cfg.RequireCSRF(w, r) {
		return
	}
	var payload SendRequest
	if !decodeChatPayload(w, r, &payload) {
		return
	}
	if err := h.cfg.Service.CanChat(r.Context(), owner); err != nil {
		writeChatResult(w, nil, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, _ := w.(http.Flusher)
	emit := func(event string, payload any) error {
		raw, err := json.Marshal(payload)
		if err != nil {
			raw, _ = json.Marshal(map[string]any{"error": err.Error()})
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}
	if flusher != nil {
		flusher.Flush()
	}
	if _, err := h.cfg.Service.SendMessage(r.Context(), owner, conversationID(r), payload, emit); err != nil {
		_ = emit("error", map[string]any{"message": chatErrorMessage(err)})
	}
}

func (h routeHandler) title(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.owner(w, r)
	if !ok {
		return
	}
	if !h.cfg.RequireCSRF(w, r) {
		return
	}
	conv, err := h.cfg.Service.GenerateConversationTitle(r.Context(), owner, conversationID(r), true)
	writeChatResult(w, conv, err)
}

func (h routeHandler) owner(w http.ResponseWriter, r *http.Request) (store.ChatOwner, bool) {
	if h.cfg.Service == nil || h.cfg.Store == nil || h.cfg.Owner == nil || h.cfg.RequireCSRF == nil {
		writeChatError(w, http.StatusInternalServerError, "chat service is not configured")
		return store.ChatOwner{}, false
	}
	owner, ok := h.cfg.Owner(r)
	if !ok {
		writeChatError(w, http.StatusUnauthorized, "login required")
		return store.ChatOwner{}, false
	}
	return owner, true
}

func conversationID(r *http.Request) int64 {
	id, _ := strconvParseInt(chi.URLParam(r, "conversationID"))
	return id
}

func strconvParseInt(value string) (int64, error) {
	var id int64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, errors.New("invalid id")
		}
		id = id*10 + int64(ch-'0')
	}
	return id, nil
}

func decodeChatPayload(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		writeChatError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeChatResult(w http.ResponseWriter, body any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, store.ErrQuotaExceeded) {
			status = http.StatusTooManyRequests
		}
		if errors.Is(err, store.ErrInvalidChatMaxToolCalls) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, store.ErrChatConversationBusy) {
			status = http.StatusConflict
		}
		if errors.Is(err, ErrEmptyMessage) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, ErrNoModel) {
			status = http.StatusBadGateway
		}
		if errors.Is(err, ErrNoTitleMessages) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, ErrEmptyTitle) {
			status = http.StatusBadGateway
		}
		writeChatError(w, status, chatErrorMessage(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func writeChatError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

func chatErrorMessage(err error) string {
	switch {
	case errors.Is(err, store.ErrQuotaExceeded):
		return "quota exceeded"
	case errors.Is(err, store.ErrNotFound):
		return "not found"
	case errors.Is(err, store.ErrInvalidChatMaxToolCalls):
		return err.Error()
	case errors.Is(err, store.ErrChatConversationBusy):
		return err.Error()
	case errors.Is(err, ErrEmptyMessage), errors.Is(err, ErrNoModel), errors.Is(err, ErrNoTitleMessages), errors.Is(err, ErrEmptyTitle):
		return err.Error()
	default:
		return err.Error()
	}
}
