package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the embedded SQLite database. It is safe for concurrent use.
type Store struct {
	db *sql.DB
}

const schema = `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	display_name  TEXT NOT NULL DEFAULT '',
	role          TEXT NOT NULL DEFAULT 'user',
	status        TEXT NOT NULL DEFAULT 'active',
	note          TEXT NOT NULL DEFAULT '',
	allowed_models TEXT NOT NULL DEFAULT '["*"]',
	rpm_limit           INTEGER NOT NULL DEFAULT 0,
	tpm_limit           INTEGER NOT NULL DEFAULT 0,
	daily_request_limit INTEGER NOT NULL DEFAULT 0,
	daily_token_limit   INTEGER NOT NULL DEFAULT 0,
	monthly_token_limit INTEGER NOT NULL DEFAULT 0,
	concurrency_limit   INTEGER NOT NULL DEFAULT 0,
	created_at    INTEGER NOT NULL,
	updated_at    INTEGER NOT NULL,
	last_login_at INTEGER
);

CREATE TABLE IF NOT EXISTS api_keys (
	id                  INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id             INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name                TEXT NOT NULL,
	key_hash            TEXT NOT NULL UNIQUE,
	key_prefix          TEXT NOT NULL,
	status              TEXT NOT NULL DEFAULT 'active',
	allowed_models      TEXT NOT NULL DEFAULT '[]',
	rpm_limit           INTEGER NOT NULL DEFAULT 0,
	tpm_limit           INTEGER NOT NULL DEFAULT 0,
	daily_token_limit   INTEGER NOT NULL DEFAULT 0,
	monthly_token_limit INTEGER NOT NULL DEFAULT 0,
	daily_request_limit INTEGER NOT NULL DEFAULT 0,
	concurrency_limit   INTEGER NOT NULL DEFAULT 0,
	expires_at          INTEGER,
	created_at          INTEGER NOT NULL,
	last_used_at        INTEGER
);
CREATE INDEX IF NOT EXISTS idx_keys_user ON api_keys(user_id);

CREATE TABLE IF NOT EXISTS providers (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	name          TEXT NOT NULL,
	type          TEXT NOT NULL DEFAULT 'openai',
	base_url      TEXT NOT NULL,
	keys_enc      TEXT NOT NULL DEFAULT '',
	extra_headers TEXT NOT NULL DEFAULT '{}',
	timeout_sec   INTEGER NOT NULL DEFAULT 120,
	priority      INTEGER NOT NULL DEFAULT 10,
	weight        INTEGER NOT NULL DEFAULT 1,
	status        TEXT NOT NULL DEFAULT 'enabled',
	health        TEXT NOT NULL DEFAULT 'unknown',
	health_msg    TEXT NOT NULL DEFAULT '',
	checked_at    INTEGER,
	created_at    INTEGER NOT NULL,
	updated_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS model_aliases (
	alias        TEXT PRIMARY KEY,
	display_name TEXT NOT NULL DEFAULT '',
	input_price  REAL NOT NULL DEFAULT 0,
	output_price REAL NOT NULL DEFAULT 0,
	enabled      INTEGER NOT NULL DEFAULT 1,
	note         TEXT NOT NULL DEFAULT '',
	created_at   INTEGER NOT NULL,
	updated_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS model_routes (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	alias          TEXT NOT NULL REFERENCES model_aliases(alias) ON DELETE CASCADE,
	provider_id    INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
	upstream_model TEXT NOT NULL,
	priority       INTEGER NOT NULL DEFAULT 10,
	weight         INTEGER NOT NULL DEFAULT 1,
	enabled        INTEGER NOT NULL DEFAULT 1,
	created_at     INTEGER NOT NULL,
	updated_at     INTEGER NOT NULL,
	UNIQUE (alias, provider_id, upstream_model)
);
CREATE INDEX IF NOT EXISTS idx_routes_alias ON model_routes(alias);
CREATE INDEX IF NOT EXISTS idx_routes_provider ON model_routes(provider_id);

CREATE TABLE IF NOT EXISTS request_logs (
	id               INTEGER PRIMARY KEY AUTOINCREMENT,
	ts               INTEGER NOT NULL,
	request_id       TEXT NOT NULL DEFAULT '',
	user_id          INTEGER,
	key_id           INTEGER,
	provider_id      INTEGER,
	model            TEXT NOT NULL DEFAULT '',
	upstream_model   TEXT NOT NULL DEFAULT '',
	status           INTEGER NOT NULL DEFAULT 0,
	error            TEXT NOT NULL DEFAULT '',
	prompt_tokens    INTEGER NOT NULL DEFAULT 0,
	completion_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens     INTEGER NOT NULL DEFAULT 0,
	cost             REAL NOT NULL DEFAULT 0,
	latency_ms       INTEGER NOT NULL DEFAULT 0,
	first_token_ms   INTEGER NOT NULL DEFAULT 0,
	stream           INTEGER NOT NULL DEFAULT 0,
	retries          INTEGER NOT NULL DEFAULT 0,
	client_ip        TEXT NOT NULL DEFAULT '',
	user_agent       TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_logs_ts ON request_logs(ts);
CREATE INDEX IF NOT EXISTS idx_logs_user_ts ON request_logs(user_id, ts);
CREATE INDEX IF NOT EXISTS idx_logs_model_ts ON request_logs(model, ts);
CREATE INDEX IF NOT EXISTS idx_logs_key_ts ON request_logs(key_id, ts);

CREATE TABLE IF NOT EXISTS usage_daily (
	key_id            INTEGER NOT NULL,
	user_id           INTEGER NOT NULL DEFAULT 0,
	day               TEXT NOT NULL,
	requests          INTEGER NOT NULL DEFAULT 0,
	prompt_tokens     INTEGER NOT NULL DEFAULT 0,
	completion_tokens INTEGER NOT NULL DEFAULT 0,
	cost              REAL NOT NULL DEFAULT 0,
	PRIMARY KEY (key_id, day)
);
CREATE INDEX IF NOT EXISTS idx_usage_day ON usage_daily(day);

CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	ip         TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS audit_logs (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	ts         INTEGER NOT NULL,
	actor_id   INTEGER,
	actor_name TEXT NOT NULL DEFAULT '',
	action     TEXT NOT NULL,
	target     TEXT NOT NULL DEFAULT '',
	detail     TEXT NOT NULL DEFAULT '',
	ip         TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_logs(ts);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// Open opens (and bootstraps) the database at path.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	// SQLite URI paths use forward slashes on every platform; filepath.ToSlash
	// keeps Windows paths (C:\...) valid.
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	st := &Store{db: db}
	if err := st.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return st, nil
}

// migrate adds columns introduced after the first release. CREATE TABLE IF NOT
// EXISTS does not alter existing tables, so upgrades need this.
func (s *Store) migrate() error {
	if err := s.ensureColumns("users", [][2]string{
		{"rpm_limit", "rpm_limit INTEGER NOT NULL DEFAULT 0"},
		{"tpm_limit", "tpm_limit INTEGER NOT NULL DEFAULT 0"},
		{"daily_request_limit", "daily_request_limit INTEGER NOT NULL DEFAULT 0"},
		{"daily_token_limit", "daily_token_limit INTEGER NOT NULL DEFAULT 0"},
		{"monthly_token_limit", "monthly_token_limit INTEGER NOT NULL DEFAULT 0"},
		{"concurrency_limit", "concurrency_limit INTEGER NOT NULL DEFAULT 0"},
	}); err != nil {
		return err
	}
	if err := s.ensureColumns("usage_daily", [][2]string{
		{"user_id", "user_id INTEGER NOT NULL DEFAULT 0"},
	}); err != nil {
		return err
	}
	// Backfill user_id for rows written before it existed.
	if _, err := s.db.Exec(`UPDATE usage_daily SET user_id =
		COALESCE((SELECT user_id FROM api_keys WHERE api_keys.id = usage_daily.key_id), 0)
		WHERE user_id = 0`); err != nil {
		return err
	}
	_, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_usage_user_day ON usage_daily(user_id, day)`)
	return err
}

func (s *Store) ensureColumns(table string, columns [][2]string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		existing[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, c := range columns {
		if existing[c[0]] {
			continue
		}
		if _, err := s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + c[1]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func now() int64 { return time.Now().Unix() }

func encodeStrings(v []string) string {
	if v == nil {
		v = []string{}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func decodeStrings(s string) []string {
	out := []string{}
	if s == "" {
		return out
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func nullInt64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func ptrInt64(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}
