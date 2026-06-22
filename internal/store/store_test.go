package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreRouteKeysAndStats(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	hasAdmin, err := st.HasAdmin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if hasAdmin {
		t.Fatal("fresh database should not have an admin")
	}
	if err := st.CreateAdmin(ctx, "admin", "hash"); err != nil {
		t.Fatal(err)
	}
	user, err := st.AdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == 0 || user.PasswordHash != "hash" {
		t.Fatalf("unexpected user: %#v", user)
	}

	provider, err := st.CreateProvider(ctx, ProviderInput{
		Name:         "openai",
		Protocol:     "openai",
		BaseAPI:      "https://api.example.test/v1",
		APIKeyCipher: "cipher",
		DefaultModel: "fallback-model",
		Models:       []string{"model-a", "model-b"},
		Enabled:      true,
		IsDefault:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	route, err := st.ResolveRoute(ctx, "unknown")
	if err != nil {
		t.Fatal(err)
	}
	if route.Provider.ID != provider.ID || route.UpstreamModel != "fallback-model" {
		t.Fatalf("unexpected fallback route: %#v", route)
	}
	route, err = st.ResolveRoute(ctx, "model-b")
	if err != nil {
		t.Fatal(err)
	}
	if route.Provider.ID != provider.ID || route.UpstreamModel != "model-b" {
		t.Fatalf("unexpected provider model route: %#v", route)
	}
	mapping, err := st.CreateMapping(ctx, "client-model", provider.ID, "upstream-model")
	if err != nil {
		t.Fatal(err)
	}
	route, err = st.ResolveRoute(ctx, mapping.ClientModel)
	if err != nil {
		t.Fatal(err)
	}
	if route.UpstreamModel != "upstream-model" {
		t.Fatalf("unexpected mapped route: %#v", route)
	}

	key, err := st.CreateDistributionKey(ctx, "test", "sk-prefix", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if !key.Enabled {
		t.Fatal("key should be enabled by default")
	}
	if err := st.UpdateKeyStats(ctx, key.ID, 7, 3, 11); err != nil {
		t.Fatal(err)
	}
	found, err := st.DistributionKeyByHash(ctx, "hash")
	if err != nil {
		t.Fatal(err)
	}
	if found.RequestCount != 1 || found.InputTokens != 7 || found.CacheReadTokens != 3 || found.OutputTokens != 11 || found.LastUsedAt == nil {
		t.Fatalf("key stats were not updated: %#v", found)
	}
	reset, err := st.ResetDistributionKey(ctx, key.ID, "sk-new-prefix", "new-hash")
	if err != nil {
		t.Fatal(err)
	}
	if reset.Prefix != "sk-new-prefix" || reset.RequestCount != 1 || reset.InputTokens != 7 || reset.CacheReadTokens != 3 || reset.OutputTokens != 11 {
		t.Fatalf("key reset did not preserve stats: %#v", reset)
	}
	if _, err := st.DistributionKeyByHash(ctx, "hash"); err != ErrNotFound {
		t.Fatalf("old key hash should be invalid after reset, got %v", err)
	}
	if _, err := st.DistributionKeyByHash(ctx, "new-hash"); err != nil {
		t.Fatal(err)
	}
	cleared, err := st.ResetDistributionKeyStats(ctx, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Prefix != "sk-new-prefix" || cleared.KeyHash != "new-hash" || !cleared.Enabled {
		t.Fatalf("key stats reset changed key identity: %#v", cleared)
	}
	if cleared.RequestCount != 0 || cleared.InputTokens != 0 || cleared.CacheReadTokens != 0 || cleared.OutputTokens != 0 || cleared.LastUsedAt != nil {
		t.Fatalf("key stats were not cleared: %#v", cleared)
	}
	if _, err := st.DistributionKeyByHash(ctx, "new-hash"); err != nil {
		t.Fatal(err)
	}

	providerID := provider.ID
	keyID := key.ID
	if err := st.InsertRequestLog(ctx, RequestLog{
		Protocol:            "openai",
		Model:               "client-model",
		ProviderID:          &providerID,
		DistributionKeyID:   &keyID,
		DistributionKeyName: "test",
		StatusCode:          200,
		LatencyMS:           25,
		InputTokens:         7,
		OutputTokens:        11,
		CacheReadTokens:     3,
		CacheCreationTokens: 2,
		Stream:              true,
	}); err != nil {
		t.Fatal(err)
	}
	stats, err := st.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRequests != 1 || stats.InputTokens != 7 || stats.OutputTokens != 11 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	providers, err := st.Providers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 || providers[0].RequestCount != 1 || providers[0].InputTokens != 7 || providers[0].OutputTokens != 11 || providers[0].CacheReadTokens != 3 {
		t.Fatalf("unexpected provider stats: %#v", providers)
	}
	logs, err := st.Logs(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].ProviderName != "openai" || !logs[0].Stream {
		t.Fatalf("unexpected logs: %#v", logs)
	}
	if logs[0].DistributionKeyID == nil || *logs[0].DistributionKeyID != key.ID || logs[0].DistributionKeyName != "test" {
		t.Fatalf("distribution key was not returned in log: %#v", logs[0])
	}
	if logs[0].CacheReadTokens != 3 || logs[0].CacheCreationTokens != 2 || logs[0].CacheHitRate < 0.42 || logs[0].CacheHitRate > 0.43 {
		t.Fatalf("cache stats were not returned: %#v", logs[0])
	}
	count, err := st.LogCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unexpected log count: %d", count)
	}
}

func TestDistributionKeyCacheColumnMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gateway.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE distribution_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		prefix TEXT NOT NULL,
		key_hash TEXT NOT NULL UNIQUE,
		enabled INTEGER NOT NULL DEFAULT 1,
		request_count INTEGER NOT NULL DEFAULT 0,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		last_used_at TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO distribution_keys(name, prefix, key_hash, input_tokens, output_tokens) VALUES('old', 'sk-old', 'old-hash', 12, 5)`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	key, err := st.DistributionKeyByHash(ctx, "old-hash")
	if err != nil {
		t.Fatal(err)
	}
	if key.InputTokens != 12 || key.CacheReadTokens != 0 || key.OutputTokens != 5 {
		t.Fatalf("unexpected migrated key: %#v", key)
	}
}

func TestRequestLogDistributionKeyColumnMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gateway.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE request_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		protocol TEXT NOT NULL,
		model TEXT NOT NULL,
		provider_id INTEGER,
		status_code INTEGER NOT NULL,
		latency_ms INTEGER NOT NULL,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0,
		cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
		stream INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	key, err := st.CreateDistributionKey(ctx, "client", "sk-client", "hash-client")
	if err != nil {
		t.Fatal(err)
	}
	keyID := key.ID
	if err := st.InsertRequestLog(ctx, RequestLog{
		Protocol:            "openai",
		Model:               "model",
		DistributionKeyID:   &keyID,
		DistributionKeyName: "client",
		StatusCode:          200,
		LatencyMS:           1,
	}); err != nil {
		t.Fatal(err)
	}
	logs, err := st.Logs(ctx, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].DistributionKeyName != "client" {
		t.Fatalf("distribution key columns were not migrated: %#v", logs)
	}
}

func TestConsumerUsersKeysQuotaAndLogs(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	user, err := st.CreateConsumerUser(ctx, "USER@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "user@example.com" || user.Status != ConsumerStatusPending || user.QuotaTotalTokens != 0 {
		t.Fatalf("unexpected new consumer: %#v", user)
	}
	user, err = st.UpdateConsumerUser(ctx, user.ID, ConsumerStatusEnabled, 10)
	if err != nil {
		t.Fatal(err)
	}
	if user.Status != ConsumerStatusEnabled || user.QuotaRemainingTokens != 10 {
		t.Fatalf("unexpected approved consumer: %#v", user)
	}
	if _, err := st.UpdateConsumerUser(ctx, user.ID, "bad", 10); err != ErrInvalidUserStatus {
		t.Fatalf("invalid status should be rejected, got %v", err)
	}

	key, err := st.CreateConsumerDistributionKey(ctx, user.ID, "consumer-key", "sk-consumer", "hash-consumer")
	if err != nil {
		t.Fatal(err)
	}
	if key.ConsumerUserID == nil || *key.ConsumerUserID != user.ID || key.ConsumerEmail != user.Email {
		t.Fatalf("consumer key was not linked: %#v", key)
	}
	unbound, err := st.CreateDistributionKey(ctx, "admin-key", "sk-admin", "hash-admin")
	if err != nil {
		t.Fatal(err)
	}
	if unbound.ConsumerUserID != nil {
		t.Fatalf("admin key should remain unbound: %#v", unbound)
	}

	keyID := key.ID
	userID := user.ID
	if err := st.RecordRequest(ctx, RequestLog{
		Protocol:            "openai",
		Model:               "model-a",
		DistributionKeyID:   &keyID,
		DistributionKeyName: key.Name,
		ConsumerUserID:      &userID,
		ConsumerEmail:       user.Email,
		StatusCode:          200,
		LatencyMS:           3,
		InputTokens:         7,
		OutputTokens:        4,
		CacheReadTokens:     2,
		CacheCreationTokens: 1,
	}); err != nil {
		t.Fatal(err)
	}
	user, err = st.ConsumerUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if user.RequestCount != 1 || user.QuotaUsedTokens != 11 || user.QuotaRemainingTokens != -1 || user.InputTokens != 7 || user.OutputTokens != 4 {
		t.Fatalf("consumer usage was not updated: %#v", user)
	}
	key, err = st.DistributionKey(ctx, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if key.RequestCount != 1 || key.InputTokens != 7 || key.OutputTokens != 4 {
		t.Fatalf("key usage was not updated: %#v", key)
	}

	if err := st.RecordRequest(ctx, RequestLog{
		Protocol:          "openai",
		Model:             "model-a",
		DistributionKeyID: &keyID,
		ConsumerUserID:    &userID,
		ConsumerEmail:     user.Email,
		StatusCode:        500,
		LatencyMS:         2,
		InputTokens:       100,
		OutputTokens:      100,
	}); err != nil {
		t.Fatal(err)
	}
	user, err = st.ConsumerUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if user.RequestCount != 1 || user.QuotaUsedTokens != 11 {
		t.Fatalf("failed request should not consume quota: %#v", user)
	}

	logs, err := st.LogsSearch(ctx, 10, 0, "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].ConsumerEmail != user.Email || logs[0].ConsumerUserID == nil {
		t.Fatalf("consumer logs were not searchable: %#v", logs)
	}
	report, err := st.ModelTokenDetails(ctx, "user", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scope != "user" || report.Name != user.Email || report.Totals.Requests != 2 || report.Totals.InputTokens != 107 {
		t.Fatalf("unexpected user report: %#v", report)
	}
}

func TestStoreLogsPagination(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	for i, model := range []string{"log-1", "log-2", "log-3"} {
		if err := st.InsertRequestLog(ctx, RequestLog{
			Protocol:    "openai",
			Model:       model,
			StatusCode:  200,
			LatencyMS:   int64(i + 1),
			InputTokens: int64(i + 10),
		}); err != nil {
			t.Fatal(err)
		}
	}

	count, err := st.LogCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("unexpected log count: %d", count)
	}

	firstPage, err := st.Logs(ctx, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage) != 2 || firstPage[0].Model != "log-3" || firstPage[1].Model != "log-2" {
		t.Fatalf("unexpected first page: %#v", firstPage)
	}

	secondPage, err := st.Logs(ctx, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage) != 1 || secondPage[0].Model != "log-1" {
		t.Fatalf("unexpected second page: %#v", secondPage)
	}
}

func TestStoreLogsSearch(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	openai, err := st.CreateProvider(ctx, ProviderInput{Name: "OpenAI Primary", Protocol: "openai", BaseAPI: "https://openai.example/v1", APIKeyCipher: "cipher", DefaultModel: "gpt-4.1", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	anthropic, err := st.CreateProvider(ctx, ProviderInput{Name: "Anthropic Backup", Protocol: "anthropic", BaseAPI: "https://anthropic.example", APIKeyCipher: "cipher", DefaultModel: "claude", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	clientA, err := st.CreateDistributionKey(ctx, "Client A", "sk-a", "hash-a")
	if err != nil {
		t.Fatal(err)
	}
	clientB, err := st.CreateDistributionKey(ctx, "Client B", "sk-b", "hash-b")
	if err != nil {
		t.Fatal(err)
	}
	openaiID, anthropicID := openai.ID, anthropic.ID
	clientAID, clientBID := clientA.ID, clientB.ID
	for _, log := range []RequestLog{
		{Protocol: "openai", Model: "gpt-4.1", ProviderID: &openaiID, DistributionKeyID: &clientAID, StatusCode: 200, LatencyMS: 1},
		{Protocol: "openai", Model: "gpt-4.1-mini", ProviderID: &openaiID, DistributionKeyID: &clientBID, StatusCode: 200, LatencyMS: 1},
		{Protocol: "anthropic", Model: "claude-3-5-sonnet", ProviderID: &anthropicID, DistributionKeyID: &clientAID, StatusCode: 200, LatencyMS: 1},
		{Protocol: "openai", Model: "literal%model", ProviderID: &openaiID, DistributionKeyID: &clientAID, StatusCode: 200, LatencyMS: 1},
	} {
		if err := st.InsertRequestLog(ctx, log); err != nil {
			t.Fatal(err)
		}
	}

	logs, err := st.LogsSearch(ctx, 20, 0, "Client B")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Model != "gpt-4.1-mini" {
		t.Fatalf("key search returned unexpected logs: %#v", logs)
	}

	logs, err = st.LogsSearch(ctx, 20, 0, "OpenAI Primary & gpt-4.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].ProviderName != "OpenAI Primary" || !strings.Contains(logs[0].Model, "gpt-4.1") || !strings.Contains(logs[1].Model, "gpt-4.1") {
		t.Fatalf("AND search returned unexpected logs: %#v", logs)
	}

	logs, err = st.LogsSearch(ctx, 20, 0, "Client B | claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("OR search returned unexpected logs: %#v", logs)
	}
	count, err := st.LogCountSearch(ctx, "Client B | claude")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("unexpected OR search count: %d", count)
	}

	logs, err = st.LogsSearch(ctx, 20, 0, "%")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Model != "literal%model" {
		t.Fatalf("LIKE escaping returned unexpected logs: %#v", logs)
	}
}

func TestStoreModelTokenDetails(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	provider, err := st.CreateProvider(ctx, ProviderInput{
		Name:         "primary",
		Protocol:     "openai",
		BaseAPI:      "https://api.example.test/v1",
		APIKeyCipher: "cipher",
		DefaultModel: "fallback",
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	otherProvider, err := st.CreateProvider(ctx, ProviderInput{
		Name:         "secondary",
		Protocol:     "anthropic",
		BaseAPI:      "https://api2.example.test",
		APIKeyCipher: "cipher",
		DefaultModel: "other",
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	emptyProvider, err := st.CreateProvider(ctx, ProviderInput{
		Name:         "empty",
		Protocol:     "openai",
		BaseAPI:      "https://empty.example.test/v1",
		APIKeyCipher: "cipher",
		DefaultModel: "empty",
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	key, err := st.CreateDistributionKey(ctx, "client-a", "sk-a", "hash-a")
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := st.CreateDistributionKey(ctx, "client-b", "sk-b", "hash-b")
	if err != nil {
		t.Fatal(err)
	}
	providerID, otherProviderID := provider.ID, otherProvider.ID
	keyID, otherKeyID := key.ID, otherKey.ID
	for _, log := range []RequestLog{
		{Protocol: "openai", Model: "model-a", ProviderID: &providerID, DistributionKeyID: &keyID, StatusCode: 200, LatencyMS: 1, InputTokens: 10, OutputTokens: 5, CacheReadTokens: 4, CacheCreationTokens: 1},
		{Protocol: "openai", Model: "model-a", ProviderID: &providerID, DistributionKeyID: &keyID, StatusCode: 200, LatencyMS: 1, InputTokens: 4, OutputTokens: 1, CacheReadTokens: 2, CacheCreationTokens: 3},
		{Protocol: "openai", Model: "model-b", ProviderID: &providerID, DistributionKeyID: &keyID, StatusCode: 200, LatencyMS: 1, InputTokens: 40, OutputTokens: 10, CacheReadTokens: 5, CacheCreationTokens: 2},
		{Protocol: "anthropic", Model: "model-c", ProviderID: &otherProviderID, DistributionKeyID: &keyID, StatusCode: 200, LatencyMS: 1, InputTokens: 30, OutputTokens: 7, CacheReadTokens: 6, CacheCreationTokens: 4},
		{Protocol: "openai", Model: "model-d", ProviderID: &providerID, DistributionKeyID: &otherKeyID, StatusCode: 500, LatencyMS: 1},
	} {
		if err := st.InsertRequestLog(ctx, log); err != nil {
			t.Fatal(err)
		}
	}

	report, err := st.ModelTokenDetails(ctx, "provider", provider.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scope != "provider" || report.ID != provider.ID || report.Name != "primary" {
		t.Fatalf("unexpected provider report identity: %#v", report)
	}
	if len(report.Items) != 3 || report.Items[0].Model != "model-b" || report.Items[1].Model != "model-a" || report.Items[2].Model != "model-d" {
		t.Fatalf("provider items were not grouped and sorted: %#v", report.Items)
	}
	if report.Items[1].Requests != 2 || report.Items[1].InputTokens != 14 || report.Items[1].OutputTokens != 6 || report.Items[1].CacheReadTokens != 6 || report.Items[1].CacheCreationTokens != 4 {
		t.Fatalf("model-a stats were not summed: %#v", report.Items[1])
	}
	if report.Totals.Requests != 4 || report.Totals.TotalTokens != 70 || report.Totals.InputTokens != 54 || report.Totals.OutputTokens != 16 || report.Totals.CacheReadTokens != 11 || report.Totals.CacheCreationTokens != 6 {
		t.Fatalf("provider totals were not summed: %#v", report.Totals)
	}
	if report.Totals.CacheHitRate < 0.20 || report.Totals.CacheHitRate > 0.21 {
		t.Fatalf("unexpected provider cache hit rate: %#v", report.Totals)
	}

	report, err = st.ModelTokenDetails(ctx, "key", key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scope != "key" || report.ID != key.ID || report.Name != "client-a" {
		t.Fatalf("unexpected key report identity: %#v", report)
	}
	if len(report.Items) != 3 || report.Items[0].Model != "model-b" || report.Items[1].Model != "model-c" || report.Items[2].Model != "model-a" {
		t.Fatalf("key items were not grouped and sorted: %#v", report.Items)
	}
	if report.Totals.Requests != 4 || report.Totals.InputTokens != 84 || report.Totals.OutputTokens != 23 || report.Totals.CacheReadTokens != 17 || report.Totals.CacheCreationTokens != 10 {
		t.Fatalf("key totals were not summed: %#v", report.Totals)
	}

	empty, err := st.ModelTokenDetails(ctx, "provider", emptyProvider.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Items) != 0 || empty.Totals.Requests != 0 || empty.Name != "empty" {
		t.Fatalf("unexpected empty report: %#v", empty)
	}
	if _, err := st.ModelTokenDetails(ctx, "bad", provider.ID); err != ErrInvalidModelScope {
		t.Fatalf("unexpected invalid scope error: %v", err)
	}
	if _, err := st.ModelTokenDetails(ctx, "provider", 9999); err != ErrNotFound {
		t.Fatalf("unexpected missing provider error: %v", err)
	}
}

func TestStoreTokenUsageBuckets(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	insertRequestLogAt(t, st, time.Date(2026, 6, 5, 1, 15, 0, 0, time.UTC), 100, 40, 25, 5)
	insertRequestLogAt(t, st, time.Date(2026, 6, 5, 1, 55, 0, 0, time.UTC), 7, 3, 2, 1)
	insertRequestLogAt(t, st, time.Date(2026, 6, 5, 10, 10, 0, 0, time.UTC), 3, 4, 1, 0)
	insertRequestLogAt(t, st, time.Date(2026, 6, 4, 10, 59, 0, 0, time.UTC), 999, 999, 999, 999)

	now := time.Date(2026, 6, 5, 10, 45, 0, 0, time.UTC)
	report, err := st.tokenUsageAt(ctx, "24h", 480, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Range != "24h" || report.Granularity != "hour" || report.TimezoneOffsetMinutes != 480 || len(report.Points) != 24 {
		t.Fatalf("unexpected 24h report metadata: %#v", report)
	}
	if want := time.Date(2026, 6, 4, 11, 0, 0, 0, time.UTC); !report.Points[0].BucketStart.Equal(want) {
		t.Fatalf("unexpected first 24h bucket: got %s want %s", report.Points[0].BucketStart, want)
	}
	if report.Points[0].Requests != 0 || report.Points[0].InputTokens != 0 {
		t.Fatalf("empty bucket was not preserved: %#v", report.Points[0])
	}
	hourBucket := report.Points[14]
	if want := time.Date(2026, 6, 5, 1, 0, 0, 0, time.UTC); !hourBucket.BucketStart.Equal(want) {
		t.Fatalf("unexpected summed hour bucket: got %s want %s", hourBucket.BucketStart, want)
	}
	if hourBucket.Requests != 2 || hourBucket.InputTokens != 107 || hourBucket.OutputTokens != 43 || hourBucket.CacheReadTokens != 27 || hourBucket.CacheCreationTokens != 6 {
		t.Fatalf("hourly token usage was not summed: %#v", hourBucket)
	}
	if report.Points[23].Requests != 1 || report.Points[23].InputTokens != 3 || report.Points[23].OutputTokens != 4 {
		t.Fatalf("current hour bucket was not returned: %#v", report.Points[23])
	}

	report, err = st.tokenUsageAt(ctx, "7d", 480, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Range != "7d" || report.Granularity != "day" || len(report.Points) != 7 {
		t.Fatalf("unexpected 7d report metadata: %#v", report)
	}
	if want := time.Date(2026, 5, 29, 16, 0, 0, 0, time.UTC); !report.Points[0].BucketStart.Equal(want) {
		t.Fatalf("unexpected first 7d bucket: got %s want %s", report.Points[0].BucketStart, want)
	}
	if report.Points[5].Requests != 1 || report.Points[5].InputTokens != 999 || report.Points[5].CacheCreationTokens != 999 {
		t.Fatalf("timezone shifted daily bucket was not returned: %#v", report.Points[5])
	}
	if report.Points[6].Requests != 3 || report.Points[6].InputTokens != 110 || report.Points[6].OutputTokens != 47 || report.Points[6].CacheReadTokens != 28 || report.Points[6].CacheCreationTokens != 6 {
		t.Fatalf("current local day bucket was not summed: %#v", report.Points[6])
	}

	if _, err := st.tokenUsageAt(ctx, "30d", 480, now); err != ErrInvalidUsageRange {
		t.Fatalf("unexpected invalid range error: %v", err)
	}
}

func insertRequestLogAt(t *testing.T, st *Store, createdAt time.Time, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int64) {
	t.Helper()
	_, err := st.db.ExecContext(context.Background(), `INSERT INTO request_logs(protocol, model, status_code, latency_ms, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, stream, created_at)
		VALUES('openai', 'model', 200, 1, ?, ?, ?, ?, 0, ?)`,
		inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens, createdAt.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatal(err)
	}
}
