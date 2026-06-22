package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type AdminUser struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}

const (
	ConsumerStatusPending  = "pending"
	ConsumerStatusEnabled  = "enabled"
	ConsumerStatusDisabled = "disabled"
)

type ConsumerUser struct {
	ID                   int64      `json:"id"`
	Email                string     `json:"email"`
	PasswordHash         string     `json:"-"`
	Status               string     `json:"status"`
	QuotaTotalTokens     int64      `json:"quota_total_tokens"`
	QuotaUsedTokens      int64      `json:"quota_used_tokens"`
	QuotaRemainingTokens int64      `json:"quota_remaining_tokens"`
	RequestCount         int64      `json:"request_count"`
	InputTokens          int64      `json:"input_tokens"`
	CacheReadTokens      int64      `json:"cache_read_tokens"`
	OutputTokens         int64      `json:"output_tokens"`
	LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type Provider struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Protocol        string     `json:"protocol"`
	BaseAPI         string     `json:"base_api"`
	APIKeyCipher    string     `json:"-"`
	DefaultModel    string     `json:"default_model"`
	Models          []string   `json:"models"`
	Enabled         bool       `json:"enabled"`
	IsDefault       bool       `json:"is_default"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	PlainAPIKey     string     `json:"api_key,omitempty"`
	HasAPIKey       bool       `json:"has_api_key"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	RequestCount    int64      `json:"request_count"`
	InputTokens     int64      `json:"input_tokens"`
	OutputTokens    int64      `json:"output_tokens"`
	CacheReadTokens int64      `json:"cache_read_tokens"`
}

type ModelMapping struct {
	ID            int64     `json:"id"`
	ClientModel   string    `json:"client_model"`
	ProviderID    int64     `json:"provider_id"`
	ProviderName  string    `json:"provider_name"`
	UpstreamModel string    `json:"upstream_model"`
	CreatedAt     time.Time `json:"created_at"`
}

type DistributionKey struct {
	ID              int64      `json:"id"`
	ConsumerUserID  *int64     `json:"consumer_user_id,omitempty"`
	ConsumerEmail   string     `json:"consumer_email,omitempty"`
	Name            string     `json:"name"`
	Prefix          string     `json:"prefix"`
	KeyHash         string     `json:"-"`
	Enabled         bool       `json:"enabled"`
	RequestCount    int64      `json:"request_count"`
	InputTokens     int64      `json:"input_tokens"`
	CacheReadTokens int64      `json:"cache_read_tokens"`
	OutputTokens    int64      `json:"output_tokens"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	PlainKey        string     `json:"plain_key,omitempty"`
}

type RequestLog struct {
	ID                  int64     `json:"id"`
	Protocol            string    `json:"protocol"`
	Model               string    `json:"model"`
	ProviderID          *int64    `json:"provider_id,omitempty"`
	ProviderName        string    `json:"provider_name,omitempty"`
	DistributionKeyID   *int64    `json:"distribution_key_id,omitempty"`
	DistributionKeyName string    `json:"distribution_key_name,omitempty"`
	ConsumerUserID      *int64    `json:"consumer_user_id,omitempty"`
	ConsumerEmail       string    `json:"consumer_email,omitempty"`
	StatusCode          int       `json:"status_code"`
	LatencyMS           int64     `json:"latency_ms"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CacheHitRate        float64   `json:"cache_hit_rate"`
	Stream              bool      `json:"stream"`
	CreatedAt           time.Time `json:"created_at"`
}

type Stats struct {
	TotalRequests int64 `json:"total_requests"`
	InputTokens   int64 `json:"input_tokens"`
	OutputTokens  int64 `json:"output_tokens"`
	ActiveKeys    int64 `json:"active_keys"`
	Providers     int64 `json:"providers"`
	ActiveUsers   int64 `json:"active_users"`
	PendingUsers  int64 `json:"pending_users"`
}

type TokenUsageReport struct {
	Range                 string            `json:"range"`
	Granularity           string            `json:"granularity"`
	TimezoneOffsetMinutes int               `json:"timezone_offset_minutes"`
	Points                []TokenUsagePoint `json:"points"`
}

type TokenUsagePoint struct {
	BucketStart         time.Time `json:"bucket_start"`
	Requests            int64     `json:"requests"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
}

type ModelTokenDetailReport struct {
	Scope  string                 `json:"scope"`
	ID     int64                  `json:"id"`
	Name   string                 `json:"name"`
	Totals ModelTokenDetailTotals `json:"totals"`
	Items  []ModelTokenDetailItem `json:"items"`
}

type ModelTokenDetailTotals struct {
	Requests            int64   `json:"requests"`
	TotalTokens         int64   `json:"total_tokens"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheHitRate        float64 `json:"cache_hit_rate"`
}

type ModelTokenDetailItem struct {
	Model               string  `json:"model"`
	Requests            int64   `json:"requests"`
	TotalTokens         int64   `json:"total_tokens"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheHitRate        float64 `json:"cache_hit_rate"`
}

type Route struct {
	Provider      Provider
	UpstreamModel string
	ClientModel   string
}

type ProviderInput struct {
	Name         string
	Protocol     string
	BaseAPI      string
	APIKeyCipher string
	DefaultModel string
	Models       []string
	Enabled      bool
	IsDefault    bool
}

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidUsageRange = errors.New("invalid token usage range")
	ErrInvalidModelScope = errors.New("invalid model token detail scope")
	ErrInvalidUserStatus = errors.New("invalid consumer user status")
	ErrQuotaExceeded     = errors.New("quota exceeded")
)

func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", filepath.ToSlash(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS admin_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS consumer_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE COLLATE NOCASE,
			password_hash TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('pending', 'enabled', 'disabled')) DEFAULT 'pending',
			quota_total_tokens INTEGER NOT NULL DEFAULT 0,
			quota_used_tokens INTEGER NOT NULL DEFAULT 0,
			request_count INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			last_used_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			protocol TEXT NOT NULL CHECK(protocol IN ('openai', 'anthropic')),
			base_api TEXT NOT NULL,
			api_key_cipher TEXT NOT NULL,
			default_model TEXT NOT NULL,
			models TEXT NOT NULL DEFAULT '[]',
			enabled INTEGER NOT NULL DEFAULT 1,
			is_default INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS model_mappings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_model TEXT NOT NULL UNIQUE,
			provider_id INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
			upstream_model TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS distribution_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			consumer_user_id INTEGER REFERENCES consumer_users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			prefix TEXT NOT NULL,
			key_hash TEXT NOT NULL UNIQUE,
			enabled INTEGER NOT NULL DEFAULT 1,
			request_count INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			last_used_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			protocol TEXT NOT NULL,
			model TEXT NOT NULL,
			provider_id INTEGER REFERENCES providers(id) ON DELETE SET NULL,
			distribution_key_id INTEGER REFERENCES distribution_keys(id) ON DELETE SET NULL,
			distribution_key_name TEXT NOT NULL DEFAULT '',
			consumer_user_id INTEGER REFERENCES consumer_users(id) ON DELETE SET NULL,
			consumer_user_email TEXT NOT NULL DEFAULT '',
			status_code INTEGER NOT NULL,
			latency_ms INTEGER NOT NULL,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
			stream INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs(created_at DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if err := s.ensureProviderModelsColumn(ctx); err != nil {
		return err
	}
	if err := s.ensureRequestLogCacheColumns(ctx); err != nil {
		return err
	}
	if err := s.ensureDistributionKeyConsumerColumn(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureDistributionKeyConsumerColumn(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(distribution_keys)`)
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !columns["cache_read_tokens"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE distribution_keys ADD COLUMN cache_read_tokens INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columns["consumer_user_id"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE distribution_keys ADD COLUMN consumer_user_id INTEGER REFERENCES consumer_users(id) ON DELETE CASCADE`); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_distribution_keys_consumer_user_id ON distribution_keys(consumer_user_id)`); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureRequestLogCacheColumns(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(request_logs)`)
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !columns["cache_read_tokens"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE request_logs ADD COLUMN cache_read_tokens INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columns["cache_creation_tokens"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE request_logs ADD COLUMN cache_creation_tokens INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columns["distribution_key_id"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE request_logs ADD COLUMN distribution_key_id INTEGER REFERENCES distribution_keys(id) ON DELETE SET NULL`); err != nil {
			return err
		}
	}
	if !columns["distribution_key_name"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE request_logs ADD COLUMN distribution_key_name TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columns["consumer_user_id"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE request_logs ADD COLUMN consumer_user_id INTEGER REFERENCES consumer_users(id) ON DELETE SET NULL`); err != nil {
			return err
		}
	}
	if !columns["consumer_user_email"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE request_logs ADD COLUMN consumer_user_email TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_request_logs_distribution_key_id ON request_logs(distribution_key_id)`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_request_logs_consumer_user_id ON request_logs(consumer_user_id)`); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureProviderModelsColumn(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(providers)`)
	if err != nil {
		return err
	}
	hasModels := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "models" {
			hasModels = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !hasModels {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE providers ADD COLUMN models TEXT NOT NULL DEFAULT '[]'`); err != nil {
			return err
		}
	}

	providers, err := s.db.QueryContext(ctx, `SELECT id, default_model, models FROM providers`)
	if err != nil {
		return err
	}
	defer providers.Close()
	type update struct {
		id     int64
		models string
	}
	var updates []update
	for providers.Next() {
		var id int64
		var defaultModel, raw string
		if err := providers.Scan(&id, &defaultModel, &raw); err != nil {
			return err
		}
		models := decodeModels(raw)
		if len(models) == 0 && strings.TrimSpace(defaultModel) != "" {
			updates = append(updates, update{id: id, models: encodeModels(nil, defaultModel)})
		}
	}
	if err := providers.Err(); err != nil {
		return err
	}
	for _, item := range updates {
		if _, err := s.db.ExecContext(ctx, `UPDATE providers SET models = ? WHERE id = ?`, item.models, item.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) HasAdmin(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) CreateAdmin(ctx context.Context, username, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO admin_users(username, password_hash) VALUES(?, ?)`, username, passwordHash)
	return err
}

func (s *Store) AdminByUsername(ctx context.Context, username string) (AdminUser, error) {
	var user AdminUser
	err := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash FROM admin_users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return user, ErrNotFound
	}
	return user, err
}

func (s *Store) CreateConsumerUser(ctx context.Context, email, passwordHash string) (ConsumerUser, error) {
	email = normalizeEmail(email)
	res, err := s.db.ExecContext(ctx, `INSERT INTO consumer_users(email, password_hash, status) VALUES(?, ?, ?)`, email, passwordHash, ConsumerStatusPending)
	if err != nil {
		return ConsumerUser{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ConsumerUser{}, err
	}
	return s.ConsumerUser(ctx, id)
}

func (s *Store) ConsumerUser(ctx context.Context, id int64) (ConsumerUser, error) {
	var user ConsumerUser
	var last sql.NullString
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `SELECT id, email, password_hash, status, quota_total_tokens, quota_used_tokens, request_count, input_tokens, cache_read_tokens, output_tokens, last_used_at, created_at, updated_at
		FROM consumer_users WHERE id = ?`, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Status, &user.QuotaTotalTokens, &user.QuotaUsedTokens, &user.RequestCount, &user.InputTokens, &user.CacheReadTokens, &user.OutputTokens, &last, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return user, ErrNotFound
	}
	if err != nil {
		return user, err
	}
	user = hydrateConsumerUser(user, last, createdAt, updatedAt)
	return user, nil
}

func (s *Store) ConsumerUserByEmail(ctx context.Context, email string) (ConsumerUser, error) {
	var user ConsumerUser
	var last sql.NullString
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `SELECT id, email, password_hash, status, quota_total_tokens, quota_used_tokens, request_count, input_tokens, cache_read_tokens, output_tokens, last_used_at, created_at, updated_at
		FROM consumer_users WHERE email = ?`, normalizeEmail(email)).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Status, &user.QuotaTotalTokens, &user.QuotaUsedTokens, &user.RequestCount, &user.InputTokens, &user.CacheReadTokens, &user.OutputTokens, &last, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return user, ErrNotFound
	}
	if err != nil {
		return user, err
	}
	user = hydrateConsumerUser(user, last, createdAt, updatedAt)
	return user, nil
}

func (s *Store) ConsumerUsers(ctx context.Context) ([]ConsumerUser, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, email, password_hash, status, quota_total_tokens, quota_used_tokens, request_count, input_tokens, cache_read_tokens, output_tokens, last_used_at, created_at, updated_at
		FROM consumer_users
		ORDER BY CASE status WHEN 'pending' THEN 0 WHEN 'enabled' THEN 1 ELSE 2 END, created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []ConsumerUser
	for rows.Next() {
		var user ConsumerUser
		var last sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Status, &user.QuotaTotalTokens, &user.QuotaUsedTokens, &user.RequestCount, &user.InputTokens, &user.CacheReadTokens, &user.OutputTokens, &last, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		users = append(users, hydrateConsumerUser(user, last, createdAt, updatedAt))
	}
	if users == nil {
		users = []ConsumerUser{}
	}
	return users, rows.Err()
}

func (s *Store) UpdateConsumerUser(ctx context.Context, id int64, status string, quotaTotalTokens int64) (ConsumerUser, error) {
	status = strings.TrimSpace(status)
	if !validConsumerStatus(status) {
		return ConsumerUser{}, ErrInvalidUserStatus
	}
	if quotaTotalTokens < 0 {
		quotaTotalTokens = 0
	}
	_, err := s.db.ExecContext(ctx, `UPDATE consumer_users SET status = ?, quota_total_tokens = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, quotaTotalTokens, id)
	if err != nil {
		return ConsumerUser{}, err
	}
	return s.ConsumerUser(ctx, id)
}

func (s *Store) UpdateConsumerUsage(ctx context.Context, userID, inputTokens, cacheReadTokens, outputTokens int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE consumer_users SET request_count = request_count + 1, input_tokens = input_tokens + ?, cache_read_tokens = cache_read_tokens + ?, output_tokens = output_tokens + ?, quota_used_tokens = quota_used_tokens + ?, last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		inputTokens, cacheReadTokens, outputTokens, inputTokens+outputTokens, userID)
	return err
}

func (s *Store) CreateProvider(ctx context.Context, input ProviderInput) (Provider, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Provider{}, err
	}
	defer tx.Rollback()
	if input.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE providers SET is_default = 0`); err != nil {
			return Provider{}, err
		}
	}
	models := encodeModels(input.Models, input.DefaultModel)
	res, err := tx.ExecContext(ctx, `INSERT INTO providers(name, protocol, base_api, api_key_cipher, default_model, models, enabled, is_default)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Name, input.Protocol, input.BaseAPI, input.APIKeyCipher, input.DefaultModel, models, boolInt(input.Enabled), boolInt(input.IsDefault))
	if err != nil {
		return Provider{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Provider{}, err
	}
	if err := tx.Commit(); err != nil {
		return Provider{}, err
	}
	return s.Provider(ctx, id)
}

func (s *Store) UpdateProvider(ctx context.Context, id int64, input ProviderInput) (Provider, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Provider{}, err
	}
	defer tx.Rollback()
	if input.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE providers SET is_default = 0 WHERE id <> ?`, id); err != nil {
			return Provider{}, err
		}
	}
	models := encodeModels(input.Models, input.DefaultModel)
	if input.APIKeyCipher != "" {
		_, err = tx.ExecContext(ctx, `UPDATE providers SET name = ?, protocol = ?, base_api = ?, api_key_cipher = ?, default_model = ?, models = ?, enabled = ?, is_default = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			input.Name, input.Protocol, input.BaseAPI, input.APIKeyCipher, input.DefaultModel, models, boolInt(input.Enabled), boolInt(input.IsDefault), id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE providers SET name = ?, protocol = ?, base_api = ?, default_model = ?, models = ?, enabled = ?, is_default = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			input.Name, input.Protocol, input.BaseAPI, input.DefaultModel, models, boolInt(input.Enabled), boolInt(input.IsDefault), id)
	}
	if err != nil {
		return Provider{}, err
	}
	if err := tx.Commit(); err != nil {
		return Provider{}, err
	}
	return s.Provider(ctx, id)
}

func (s *Store) DeleteProvider(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM providers WHERE id = ?`, id)
	return err
}

func (s *Store) Provider(ctx context.Context, id int64) (Provider, error) {
	var p Provider
	var createdAt, updatedAt string
	var modelsRaw string
	err := s.db.QueryRowContext(ctx, `SELECT id, name, protocol, base_api, api_key_cipher, default_model, models, enabled, is_default, created_at, updated_at FROM providers WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Protocol, &p.BaseAPI, &p.APIKeyCipher, &p.DefaultModel, &modelsRaw, &p.Enabled, &p.IsDefault, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	p.CreatedAt = parseDBTime(createdAt)
	p.UpdatedAt = parseDBTime(updatedAt)
	p.Models = normalizeModels(decodeModels(modelsRaw), p.DefaultModel)
	p.HasAPIKey = p.APIKeyCipher != ""
	return p, err
}

func (s *Store) Providers(ctx context.Context) ([]Provider, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.name, p.protocol, p.base_api, p.api_key_cipher, p.default_model, p.models, p.enabled, p.is_default, p.created_at, p.updated_at,
		COUNT(l.id), COALESCE(SUM(l.input_tokens), 0), COALESCE(SUM(l.output_tokens), 0), COALESCE(SUM(l.cache_read_tokens), 0), MAX(l.created_at)
		FROM providers p
		LEFT JOIN request_logs l ON l.provider_id = p.id
		GROUP BY p.id
		ORDER BY p.is_default DESC, p.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var providers []Provider
	for rows.Next() {
		var p Provider
		var createdAt, updatedAt string
		var modelsRaw string
		var last sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Protocol, &p.BaseAPI, &p.APIKeyCipher, &p.DefaultModel, &modelsRaw, &p.Enabled, &p.IsDefault, &createdAt, &updatedAt,
			&p.RequestCount, &p.InputTokens, &p.OutputTokens, &p.CacheReadTokens, &last); err != nil {
			return nil, err
		}
		p.CreatedAt = parseDBTime(createdAt)
		p.UpdatedAt = parseDBTime(updatedAt)
		p.Models = normalizeModels(decodeModels(modelsRaw), p.DefaultModel)
		p.HasAPIKey = p.APIKeyCipher != ""
		if last.Valid {
			t := parseDBTime(last.String)
			p.LastUsedAt = &t
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

func (s *Store) ResolveRoute(ctx context.Context, clientModel string) (Route, error) {
	var r Route
	var p Provider
	var createdAt, updatedAt string
	var modelsRaw string
	err := s.db.QueryRowContext(ctx, `SELECT p.id, p.name, p.protocol, p.base_api, p.api_key_cipher, p.default_model, p.models, p.enabled, p.is_default, p.created_at, p.updated_at, m.upstream_model
		FROM model_mappings m
		JOIN providers p ON p.id = m.provider_id
		WHERE m.client_model = ? AND p.enabled = 1`, clientModel).
		Scan(&p.ID, &p.Name, &p.Protocol, &p.BaseAPI, &p.APIKeyCipher, &p.DefaultModel, &modelsRaw, &p.Enabled, &p.IsDefault, &createdAt, &updatedAt, &r.UpstreamModel)
	if err == nil {
		p.CreatedAt = parseDBTime(createdAt)
		p.UpdatedAt = parseDBTime(updatedAt)
		p.Models = normalizeModels(decodeModels(modelsRaw), p.DefaultModel)
		p.HasAPIKey = p.APIKeyCipher != ""
		r.Provider = p
		r.ClientModel = clientModel
		return r, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return r, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, protocol, base_api, api_key_cipher, default_model, models, enabled, is_default, created_at, updated_at
		FROM providers
		WHERE enabled = 1
		ORDER BY is_default DESC, id ASC`)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	var fallback *Provider
	for rows.Next() {
		var item Provider
		var itemCreatedAt, itemUpdatedAt, itemModelsRaw string
		if err := rows.Scan(&item.ID, &item.Name, &item.Protocol, &item.BaseAPI, &item.APIKeyCipher, &item.DefaultModel, &itemModelsRaw, &item.Enabled, &item.IsDefault, &itemCreatedAt, &itemUpdatedAt); err != nil {
			return r, err
		}
		item.CreatedAt = parseDBTime(itemCreatedAt)
		item.UpdatedAt = parseDBTime(itemUpdatedAt)
		item.Models = normalizeModels(decodeModels(itemModelsRaw), item.DefaultModel)
		item.HasAPIKey = item.APIKeyCipher != ""
		if fallback == nil {
			copy := item
			fallback = &copy
		}
		if clientModel != "" && modelSupported(item, clientModel) {
			r.Provider = item
			r.ClientModel = clientModel
			r.UpstreamModel = clientModel
			return r, nil
		}
	}
	if err := rows.Err(); err != nil {
		return r, err
	}
	if fallback == nil {
		return r, ErrNotFound
	}
	r.Provider = *fallback
	r.ClientModel = clientModel
	r.UpstreamModel = fallback.DefaultModel
	return r, nil
}

func (s *Store) CreateMapping(ctx context.Context, clientModel string, providerID int64, upstreamModel string) (ModelMapping, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO model_mappings(client_model, provider_id, upstream_model) VALUES(?, ?, ?)`, clientModel, providerID, upstreamModel)
	if err != nil {
		return ModelMapping{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ModelMapping{}, err
	}
	return s.Mapping(ctx, id)
}

func (s *Store) UpdateMapping(ctx context.Context, id int64, clientModel string, providerID int64, upstreamModel string) (ModelMapping, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE model_mappings SET client_model = ?, provider_id = ?, upstream_model = ? WHERE id = ?`, clientModel, providerID, upstreamModel, id)
	if err != nil {
		return ModelMapping{}, err
	}
	return s.Mapping(ctx, id)
}

func (s *Store) DeleteMapping(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM model_mappings WHERE id = ?`, id)
	return err
}

func (s *Store) Mapping(ctx context.Context, id int64) (ModelMapping, error) {
	var m ModelMapping
	var createdAt string
	err := s.db.QueryRowContext(ctx, `SELECT m.id, m.client_model, m.provider_id, p.name, m.upstream_model, m.created_at
		FROM model_mappings m JOIN providers p ON p.id = m.provider_id WHERE m.id = ?`, id).
		Scan(&m.ID, &m.ClientModel, &m.ProviderID, &m.ProviderName, &m.UpstreamModel, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrNotFound
	}
	m.CreatedAt = parseDBTime(createdAt)
	return m, err
}

func (s *Store) Mappings(ctx context.Context) ([]ModelMapping, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.id, m.client_model, m.provider_id, p.name, m.upstream_model, m.created_at
		FROM model_mappings m JOIN providers p ON p.id = m.provider_id ORDER BY m.client_model ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var mappings []ModelMapping
	for rows.Next() {
		var m ModelMapping
		var createdAt string
		if err := rows.Scan(&m.ID, &m.ClientModel, &m.ProviderID, &m.ProviderName, &m.UpstreamModel, &createdAt); err != nil {
			return nil, err
		}
		m.CreatedAt = parseDBTime(createdAt)
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

func (s *Store) CreateDistributionKey(ctx context.Context, name, prefix, hash string) (DistributionKey, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO distribution_keys(name, prefix, key_hash, enabled) VALUES(?, ?, ?, 1)`, name, prefix, hash)
	if err != nil {
		return DistributionKey{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return DistributionKey{}, err
	}
	return s.DistributionKey(ctx, id)
}

func (s *Store) CreateConsumerDistributionKey(ctx context.Context, consumerUserID int64, name, prefix, hash string) (DistributionKey, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO distribution_keys(consumer_user_id, name, prefix, key_hash, enabled) VALUES(?, ?, ?, ?, 1)`, consumerUserID, name, prefix, hash)
	if err != nil {
		return DistributionKey{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return DistributionKey{}, err
	}
	return s.DistributionKey(ctx, id)
}

func (s *Store) UpdateDistributionKey(ctx context.Context, id int64, name string, enabled bool) (DistributionKey, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE distribution_keys SET name = ?, enabled = ? WHERE id = ?`, name, boolInt(enabled), id)
	if err != nil {
		return DistributionKey{}, err
	}
	return s.DistributionKey(ctx, id)
}

func (s *Store) UpdateConsumerDistributionKey(ctx context.Context, consumerUserID, id int64, name string, enabled bool) (DistributionKey, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE distribution_keys SET name = ?, enabled = ? WHERE id = ? AND consumer_user_id = ?`, name, boolInt(enabled), id, consumerUserID)
	if err != nil {
		return DistributionKey{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return DistributionKey{}, err
	}
	if affected == 0 {
		return DistributionKey{}, ErrNotFound
	}
	return s.DistributionKey(ctx, id)
}

func (s *Store) ResetDistributionKey(ctx context.Context, id int64, prefix, hash string) (DistributionKey, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE distribution_keys SET prefix = ?, key_hash = ? WHERE id = ?`, prefix, hash, id)
	if err != nil {
		return DistributionKey{}, err
	}
	return s.DistributionKey(ctx, id)
}

func (s *Store) ResetConsumerDistributionKey(ctx context.Context, consumerUserID, id int64, prefix, hash string) (DistributionKey, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE distribution_keys SET prefix = ?, key_hash = ? WHERE id = ? AND consumer_user_id = ?`, prefix, hash, id, consumerUserID)
	if err != nil {
		return DistributionKey{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return DistributionKey{}, err
	}
	if affected == 0 {
		return DistributionKey{}, ErrNotFound
	}
	return s.DistributionKey(ctx, id)
}

func (s *Store) ResetDistributionKeyStats(ctx context.Context, id int64) (DistributionKey, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE distribution_keys SET request_count = 0, input_tokens = 0, cache_read_tokens = 0, output_tokens = 0, last_used_at = NULL WHERE id = ?`, id)
	if err != nil {
		return DistributionKey{}, err
	}
	return s.DistributionKey(ctx, id)
}

func (s *Store) DeleteDistributionKey(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM distribution_keys WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteConsumerDistributionKey(ctx context.Context, consumerUserID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM distribution_keys WHERE id = ? AND consumer_user_id = ?`, id, consumerUserID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DistributionKey(ctx context.Context, id int64) (DistributionKey, error) {
	var k DistributionKey
	var last sql.NullString
	var createdAt string
	var consumerUserID sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT k.id, k.consumer_user_id, COALESCE(u.email, ''), k.name, k.prefix, k.key_hash, k.enabled, k.request_count, k.input_tokens, k.cache_read_tokens, k.output_tokens, k.last_used_at, k.created_at
		FROM distribution_keys k
		LEFT JOIN consumer_users u ON u.id = k.consumer_user_id
		WHERE k.id = ?`, id).
		Scan(&k.ID, &consumerUserID, &k.ConsumerEmail, &k.Name, &k.Prefix, &k.KeyHash, &k.Enabled, &k.RequestCount, &k.InputTokens, &k.CacheReadTokens, &k.OutputTokens, &last, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return k, ErrNotFound
	}
	if err != nil {
		return k, err
	}
	if consumerUserID.Valid {
		id := consumerUserID.Int64
		k.ConsumerUserID = &id
	}
	k.CreatedAt = parseDBTime(createdAt)
	if last.Valid {
		t := parseDBTime(last.String)
		k.LastUsedAt = &t
	}
	return k, nil
}

func (s *Store) DistributionKeyByHash(ctx context.Context, hash string) (DistributionKey, error) {
	var k DistributionKey
	var last sql.NullString
	var createdAt string
	var consumerUserID sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT k.id, k.consumer_user_id, COALESCE(u.email, ''), k.name, k.prefix, k.key_hash, k.enabled, k.request_count, k.input_tokens, k.cache_read_tokens, k.output_tokens, k.last_used_at, k.created_at
		FROM distribution_keys k
		LEFT JOIN consumer_users u ON u.id = k.consumer_user_id
		WHERE k.key_hash = ?`, hash).
		Scan(&k.ID, &consumerUserID, &k.ConsumerEmail, &k.Name, &k.Prefix, &k.KeyHash, &k.Enabled, &k.RequestCount, &k.InputTokens, &k.CacheReadTokens, &k.OutputTokens, &last, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return k, ErrNotFound
	}
	if err != nil {
		return k, err
	}
	if consumerUserID.Valid {
		id := consumerUserID.Int64
		k.ConsumerUserID = &id
	}
	k.CreatedAt = parseDBTime(createdAt)
	if last.Valid {
		t := parseDBTime(last.String)
		k.LastUsedAt = &t
	}
	return k, nil
}

func (s *Store) DistributionKeys(ctx context.Context) ([]DistributionKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT k.id, k.consumer_user_id, COALESCE(u.email, ''), k.name, k.prefix, k.key_hash, k.enabled, k.request_count, k.input_tokens, k.cache_read_tokens, k.output_tokens, k.last_used_at, k.created_at
		FROM distribution_keys k
		LEFT JOIN consumer_users u ON u.id = k.consumer_user_id
		ORDER BY k.created_at DESC, k.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []DistributionKey
	for rows.Next() {
		var k DistributionKey
		var last sql.NullString
		var createdAt string
		var consumerUserID sql.NullInt64
		if err := rows.Scan(&k.ID, &consumerUserID, &k.ConsumerEmail, &k.Name, &k.Prefix, &k.KeyHash, &k.Enabled, &k.RequestCount, &k.InputTokens, &k.CacheReadTokens, &k.OutputTokens, &last, &createdAt); err != nil {
			return nil, err
		}
		if consumerUserID.Valid {
			id := consumerUserID.Int64
			k.ConsumerUserID = &id
		}
		k.CreatedAt = parseDBTime(createdAt)
		if last.Valid {
			t := parseDBTime(last.String)
			k.LastUsedAt = &t
		}
		keys = append(keys, k)
	}
	if keys == nil {
		keys = []DistributionKey{}
	}
	return keys, rows.Err()
}

func (s *Store) ConsumerDistributionKeys(ctx context.Context, consumerUserID int64) ([]DistributionKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT k.id, k.consumer_user_id, COALESCE(u.email, ''), k.name, k.prefix, k.key_hash, k.enabled, k.request_count, k.input_tokens, k.cache_read_tokens, k.output_tokens, k.last_used_at, k.created_at
		FROM distribution_keys k
		LEFT JOIN consumer_users u ON u.id = k.consumer_user_id
		WHERE k.consumer_user_id = ?
		ORDER BY k.created_at DESC, k.id DESC`, consumerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []DistributionKey
	for rows.Next() {
		var k DistributionKey
		var last sql.NullString
		var createdAt string
		var scannedConsumerUserID sql.NullInt64
		if err := rows.Scan(&k.ID, &scannedConsumerUserID, &k.ConsumerEmail, &k.Name, &k.Prefix, &k.KeyHash, &k.Enabled, &k.RequestCount, &k.InputTokens, &k.CacheReadTokens, &k.OutputTokens, &last, &createdAt); err != nil {
			return nil, err
		}
		if scannedConsumerUserID.Valid {
			id := scannedConsumerUserID.Int64
			k.ConsumerUserID = &id
		}
		k.CreatedAt = parseDBTime(createdAt)
		if last.Valid {
			t := parseDBTime(last.String)
			k.LastUsedAt = &t
		}
		keys = append(keys, k)
	}
	if keys == nil {
		keys = []DistributionKey{}
	}
	return keys, rows.Err()
}

func (s *Store) UpdateKeyStats(ctx context.Context, keyID int64, inputTokens, cacheReadTokens, outputTokens int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE distribution_keys SET request_count = request_count + 1, input_tokens = input_tokens + ?, cache_read_tokens = cache_read_tokens + ?, output_tokens = output_tokens + ?, last_used_at = CURRENT_TIMESTAMP WHERE id = ?`,
		inputTokens, cacheReadTokens, outputTokens, keyID)
	return err
}

func (s *Store) InsertRequestLog(ctx context.Context, log RequestLog) error {
	var providerID any
	if log.ProviderID != nil {
		providerID = *log.ProviderID
	}
	var distributionKeyID any
	if log.DistributionKeyID != nil {
		distributionKeyID = *log.DistributionKeyID
	}
	var consumerUserID any
	if log.ConsumerUserID != nil {
		consumerUserID = *log.ConsumerUserID
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO request_logs(protocol, model, provider_id, distribution_key_id, distribution_key_name, consumer_user_id, consumer_user_email, status_code, latency_ms, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, stream)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.Protocol, log.Model, providerID, distributionKeyID, log.DistributionKeyName, consumerUserID, log.ConsumerEmail, log.StatusCode, log.LatencyMS, log.InputTokens, log.OutputTokens, log.CacheReadTokens, log.CacheCreationTokens, boolInt(log.Stream))
	return err
}

func (s *Store) RecordRequest(ctx context.Context, log RequestLog) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var providerID any
	if log.ProviderID != nil {
		providerID = *log.ProviderID
	}
	var distributionKeyID any
	if log.DistributionKeyID != nil {
		distributionKeyID = *log.DistributionKeyID
	}
	var consumerUserID any
	if log.ConsumerUserID != nil {
		consumerUserID = *log.ConsumerUserID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO request_logs(protocol, model, provider_id, distribution_key_id, distribution_key_name, consumer_user_id, consumer_user_email, status_code, latency_ms, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, stream)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.Protocol, log.Model, providerID, distributionKeyID, log.DistributionKeyName, consumerUserID, log.ConsumerEmail, log.StatusCode, log.LatencyMS, log.InputTokens, log.OutputTokens, log.CacheReadTokens, log.CacheCreationTokens, boolInt(log.Stream)); err != nil {
		return err
	}
	if log.StatusCode >= 200 && log.StatusCode < 300 {
		if log.DistributionKeyID != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE distribution_keys SET request_count = request_count + 1, input_tokens = input_tokens + ?, cache_read_tokens = cache_read_tokens + ?, output_tokens = output_tokens + ?, last_used_at = CURRENT_TIMESTAMP WHERE id = ?`,
				log.InputTokens, log.CacheReadTokens, log.OutputTokens, *log.DistributionKeyID); err != nil {
				return err
			}
		}
		if log.ConsumerUserID != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE consumer_users SET request_count = request_count + 1, input_tokens = input_tokens + ?, cache_read_tokens = cache_read_tokens + ?, output_tokens = output_tokens + ?, quota_used_tokens = quota_used_tokens + ?, last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
				log.InputTokens, log.CacheReadTokens, log.OutputTokens, log.InputTokens+log.OutputTokens, *log.ConsumerUserID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) Logs(ctx context.Context, limit, offset int) ([]RequestLog, error) {
	return s.LogsSearch(ctx, limit, offset, "")
}

func (s *Store) LogsSearch(ctx context.Context, limit, offset int, search string) ([]RequestLog, error) {
	limit, offset = normalizeLogPagination(limit, offset)
	where, args := logSearchWhere(search)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT l.id, l.protocol, l.model, l.provider_id, COALESCE(p.name, ''), l.distribution_key_id, COALESCE(NULLIF(l.distribution_key_name, ''), k.name, ''), l.consumer_user_id, COALESCE(NULLIF(l.consumer_user_email, ''), u.email, ''), l.status_code, l.latency_ms, l.input_tokens, l.output_tokens, l.cache_read_tokens, l.cache_creation_tokens, l.stream, l.created_at
		FROM request_logs l
		LEFT JOIN providers p ON p.id = l.provider_id
		LEFT JOIN distribution_keys k ON k.id = l.distribution_key_id
		LEFT JOIN consumer_users u ON u.id = l.consumer_user_id
		`+where+`
		ORDER BY l.created_at DESC, l.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []RequestLog
	for rows.Next() {
		var l RequestLog
		var providerID sql.NullInt64
		var distributionKeyID sql.NullInt64
		var consumerUserID sql.NullInt64
		var createdAt string
		if err := rows.Scan(&l.ID, &l.Protocol, &l.Model, &providerID, &l.ProviderName, &distributionKeyID, &l.DistributionKeyName, &consumerUserID, &l.ConsumerEmail, &l.StatusCode, &l.LatencyMS, &l.InputTokens, &l.OutputTokens, &l.CacheReadTokens, &l.CacheCreationTokens, &l.Stream, &createdAt); err != nil {
			return nil, err
		}
		l.CreatedAt = parseDBTime(createdAt)
		if l.InputTokens > 0 && l.CacheReadTokens > 0 {
			l.CacheHitRate = float64(l.CacheReadTokens) / float64(l.InputTokens)
		}
		if providerID.Valid {
			id := providerID.Int64
			l.ProviderID = &id
		}
		if distributionKeyID.Valid {
			id := distributionKeyID.Int64
			l.DistributionKeyID = &id
		}
		if consumerUserID.Valid {
			id := consumerUserID.Int64
			l.ConsumerUserID = &id
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (s *Store) LogCount(ctx context.Context) (int64, error) {
	return s.LogCountSearch(ctx, "")
}

func (s *Store) LogCountSearch(ctx context.Context, search string) (int64, error) {
	var total int64
	where, args := logSearchWhere(search)
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)
		FROM request_logs l
		LEFT JOIN providers p ON p.id = l.provider_id
		LEFT JOIN distribution_keys k ON k.id = l.distribution_key_id
		LEFT JOIN consumer_users u ON u.id = l.consumer_user_id
		`+where, args...).Scan(&total)
	return total, err
}

func (s *Store) ModelTokenDetails(ctx context.Context, scope string, id int64) (ModelTokenDetailReport, error) {
	if id <= 0 {
		return ModelTokenDetailReport{}, ErrNotFound
	}
	report := ModelTokenDetailReport{Scope: strings.TrimSpace(scope), ID: id}
	var filterColumn string
	switch report.Scope {
	case "provider":
		provider, err := s.Provider(ctx, id)
		if err != nil {
			return ModelTokenDetailReport{}, err
		}
		report.Name = provider.Name
		filterColumn = "provider_id"
	case "key":
		key, err := s.DistributionKey(ctx, id)
		if err != nil {
			return ModelTokenDetailReport{}, err
		}
		report.Name = key.Name
		filterColumn = "distribution_key_id"
	case "user":
		user, err := s.ConsumerUser(ctx, id)
		if err != nil {
			return ModelTokenDetailReport{}, err
		}
		report.Name = user.Email
		filterColumn = "consumer_user_id"
	default:
		return ModelTokenDetailReport{}, ErrInvalidModelScope
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT model, COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cache_read_tokens), 0),
			COALESCE(SUM(cache_creation_tokens), 0)
		FROM request_logs
		WHERE %s = ?
		GROUP BY model
		ORDER BY COALESCE(SUM(input_tokens), 0) + COALESCE(SUM(output_tokens), 0) DESC, COUNT(*) DESC, model ASC`, filterColumn), id)
	if err != nil {
		return ModelTokenDetailReport{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ModelTokenDetailItem
		if err := rows.Scan(&item.Model, &item.Requests, &item.InputTokens, &item.OutputTokens, &item.CacheReadTokens, &item.CacheCreationTokens); err != nil {
			return ModelTokenDetailReport{}, err
		}
		item.TotalTokens = item.InputTokens + item.OutputTokens
		if item.InputTokens > 0 && item.CacheReadTokens > 0 {
			item.CacheHitRate = float64(item.CacheReadTokens) / float64(item.InputTokens)
		}
		report.Totals.Requests += item.Requests
		report.Totals.InputTokens += item.InputTokens
		report.Totals.OutputTokens += item.OutputTokens
		report.Totals.CacheReadTokens += item.CacheReadTokens
		report.Totals.CacheCreationTokens += item.CacheCreationTokens
		report.Items = append(report.Items, item)
	}
	if err := rows.Err(); err != nil {
		return ModelTokenDetailReport{}, err
	}
	report.Totals.TotalTokens = report.Totals.InputTokens + report.Totals.OutputTokens
	if report.Totals.InputTokens > 0 && report.Totals.CacheReadTokens > 0 {
		report.Totals.CacheHitRate = float64(report.Totals.CacheReadTokens) / float64(report.Totals.InputTokens)
	}
	return report, nil
}

func (s *Store) TokenUsage(ctx context.Context, usageRange string, tzOffsetMinutes int) (TokenUsageReport, error) {
	return s.tokenUsageAt(ctx, usageRange, tzOffsetMinutes, time.Now().UTC())
}

func (s *Store) tokenUsageAt(ctx context.Context, usageRange string, tzOffsetMinutes int, now time.Time) (TokenUsageReport, error) {
	normalizedRange, granularity, bucketCount, step, err := tokenUsageSpec(usageRange)
	if err != nil {
		return TokenUsageReport{}, err
	}
	tzOffsetMinutes = normalizeTimezoneOffset(tzOffsetMinutes)
	now = now.UTC()
	currentBucket := floorUsageBucket(now, step, tzOffsetMinutes)
	start := currentBucket.Add(-time.Duration(bucketCount-1) * step)
	end := currentBucket.Add(step)

	report := TokenUsageReport{
		Range:                 normalizedRange,
		Granularity:           granularity,
		TimezoneOffsetMinutes: tzOffsetMinutes,
		Points:                make([]TokenUsagePoint, bucketCount),
	}
	for i := range report.Points {
		report.Points[i].BucketStart = start.Add(time.Duration(i) * step).UTC()
	}

	stepSeconds := int64(step / time.Second)
	offsetSeconds := int64(tzOffsetMinutes) * 60
	rows, err := s.db.QueryContext(ctx, `SELECT
			CAST((CAST(strftime('%s', created_at) AS INTEGER) + ?) / ? AS INTEGER) * ? - ? AS bucket_epoch,
			COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cache_read_tokens), 0),
			COALESCE(SUM(cache_creation_tokens), 0)
		FROM request_logs
		WHERE CAST(strftime('%s', created_at) AS INTEGER) >= ?
			AND CAST(strftime('%s', created_at) AS INTEGER) < ?
		GROUP BY bucket_epoch
		ORDER BY bucket_epoch`,
		offsetSeconds, stepSeconds, stepSeconds, offsetSeconds, start.Unix(), end.Unix())
	if err != nil {
		return TokenUsageReport{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var bucketEpoch int64
		var point TokenUsagePoint
		if err := rows.Scan(&bucketEpoch, &point.Requests, &point.InputTokens, &point.OutputTokens, &point.CacheReadTokens, &point.CacheCreationTokens); err != nil {
			return TokenUsageReport{}, err
		}
		point.BucketStart = time.Unix(bucketEpoch, 0).UTC()
		idx := int(point.BucketStart.Sub(start) / step)
		if idx >= 0 && idx < len(report.Points) {
			report.Points[idx] = point
		}
	}
	return report, rows.Err()
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var stats Stats
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0) FROM request_logs`).
		Scan(&stats.TotalRequests, &stats.InputTokens, &stats.OutputTokens); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM distribution_keys WHERE enabled = 1`).Scan(&stats.ActiveKeys); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM providers WHERE enabled = 1`).Scan(&stats.Providers); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_users WHERE status = ?`, ConsumerStatusEnabled).Scan(&stats.ActiveUsers); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumer_users WHERE status = ?`, ConsumerStatusPending).Scan(&stats.PendingUsers); err != nil {
		return stats, err
	}
	return stats, nil
}

func normalizeLogPagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func logSearchWhere(search string) (string, []any) {
	groups := logSearchGroups(search)
	if len(groups) == 0 {
		return "", nil
	}
	var args []any
	var groupClauses []string
	for _, group := range groups {
		var termClauses []string
		for _, term := range group {
			pattern := "%" + escapeLike(term) + "%"
			termClauses = append(termClauses, `(l.model LIKE ? ESCAPE '\' OR COALESCE(p.name, '') LIKE ? ESCAPE '\' OR COALESCE(NULLIF(l.distribution_key_name, ''), k.name, '') LIKE ? ESCAPE '\' OR COALESCE(NULLIF(l.consumer_user_email, ''), u.email, '') LIKE ? ESCAPE '\')`)
			args = append(args, pattern, pattern, pattern, pattern)
		}
		if len(termClauses) > 0 {
			groupClauses = append(groupClauses, "("+strings.Join(termClauses, " AND ")+")")
		}
	}
	if len(groupClauses) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(groupClauses, " OR "), args
}

func logSearchGroups(search string) [][]string {
	var groups [][]string
	for _, rawGroup := range strings.Split(search, "|") {
		var terms []string
		for _, rawTerm := range strings.Split(rawGroup, "&") {
			if term := strings.TrimSpace(rawTerm); term != "" {
				terms = append(terms, term)
			}
		}
		if len(terms) > 0 {
			groups = append(groups, terms)
		}
	}
	return groups
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func tokenUsageSpec(usageRange string) (string, string, int, time.Duration, error) {
	switch strings.TrimSpace(usageRange) {
	case "", "24h":
		return "24h", "hour", 24, time.Hour, nil
	case "7d":
		return "7d", "day", 7, 24 * time.Hour, nil
	default:
		return "", "", 0, 0, ErrInvalidUsageRange
	}
}

func normalizeTimezoneOffset(minutes int) int {
	const maxOffsetMinutes = 14 * 60
	if minutes < -maxOffsetMinutes || minutes > maxOffsetMinutes {
		return 0
	}
	return minutes
}

func floorUsageBucket(t time.Time, step time.Duration, tzOffsetMinutes int) time.Time {
	offset := time.Duration(tzOffsetMinutes) * time.Minute
	return t.UTC().Add(offset).Truncate(step).Add(-offset).UTC()
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func parseDBTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func hydrateConsumerUser(user ConsumerUser, last sql.NullString, createdAt, updatedAt string) ConsumerUser {
	user.CreatedAt = parseDBTime(createdAt)
	user.UpdatedAt = parseDBTime(updatedAt)
	user.QuotaRemainingTokens = user.QuotaTotalTokens - user.QuotaUsedTokens
	if last.Valid {
		t := parseDBTime(last.String)
		user.LastUsedAt = &t
	}
	return user
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validConsumerStatus(status string) bool {
	switch status {
	case ConsumerStatusPending, ConsumerStatusEnabled, ConsumerStatusDisabled:
		return true
	default:
		return false
	}
}

func encodeModels(models []string, defaultModel string) string {
	normalized := normalizeModels(models, defaultModel)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func decodeModels(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var models []string
	if err := json.Unmarshal([]byte(raw), &models); err == nil {
		return models
	}
	return splitModels(raw)
}

func normalizeModels(models []string, defaultModel string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(model string) {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			return
		}
		seen[model] = true
		out = append(out, model)
	}
	add(defaultModel)
	for _, model := range models {
		for _, item := range splitModels(model) {
			add(item)
		}
	}
	return out
}

func splitModels(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t'
	})
	var out []string
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func modelSupported(provider Provider, model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	for _, supported := range provider.Models {
		if supported == model {
			return true
		}
	}
	return false
}
