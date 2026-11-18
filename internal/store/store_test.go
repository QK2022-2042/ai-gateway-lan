package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigrateLegacySchema simulates a database created before user-level
// quotas and usage_daily.user_id existed.
func TestMigrateLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`
CREATE TABLE users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	display_name  TEXT NOT NULL DEFAULT '',
	role          TEXT NOT NULL DEFAULT 'user',
	status        TEXT NOT NULL DEFAULT 'active',
	note          TEXT NOT NULL DEFAULT '',
	allowed_models TEXT NOT NULL DEFAULT '["*"]',
	created_at    INTEGER NOT NULL,
	updated_at    INTEGER NOT NULL,
	last_login_at INTEGER
);
CREATE TABLE usage_daily (
	key_id            INTEGER NOT NULL,
	day               TEXT NOT NULL,
	requests          INTEGER NOT NULL DEFAULT 0,
	prompt_tokens     INTEGER NOT NULL DEFAULT 0,
	completion_tokens INTEGER NOT NULL DEFAULT 0,
	cost              REAL NOT NULL DEFAULT 0,
	PRIMARY KEY (key_id, day)
);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	u := &User{
		Username: "legacy", PasswordHash: "x", Role: "user", Status: "active",
		AllowedModels: []string{"*"}, RPMLimit: 5, DailyRequestLimit: 9,
		MonthlyTokenLimit: 1000, ConcurrencyLimit: 2,
	}
	if err := st.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	got, err := st.UserByID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RPMLimit != 5 || got.DailyRequestLimit != 9 || got.MonthlyTokenLimit != 1000 ||
		got.ConcurrencyLimit != 2 {
		t.Fatalf("limits not persisted: %+v", got)
	}

	if _, err := st.DB().Exec(`INSERT INTO usage_daily
		(key_id, user_id, day, requests, prompt_tokens, completion_tokens, cost)
		VALUES (1, ?, '2026-11-15', 2, 10, 20, 0.1)`, u.ID); err != nil {
		t.Fatal(err)
	}
	usage, err := st.UserUsageOnDay(u.ID, "2026-11-15")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Requests != 2 || usage.Tokens != 30 {
		t.Fatalf("usage=%+v", usage)
	}
}
