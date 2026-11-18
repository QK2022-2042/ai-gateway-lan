package store

import (
	"database/sql"
	"errors"
	"strings"
)

type APIKey struct {
	ID                int64    `json:"id"`
	UserID            int64    `json:"user_id"`
	Username          string   `json:"username,omitempty"`
	Name              string   `json:"name"`
	KeyHash           string   `json:"-"`
	KeyPrefix         string   `json:"key_prefix"`
	Status            string   `json:"status"`
	AllowedModels     []string `json:"allowed_models"`
	RPMLimit          int      `json:"rpm_limit"`
	TPMLimit          int      `json:"tpm_limit"`
	DailyTokenLimit   int      `json:"daily_token_limit"`
	MonthlyTokenLimit int      `json:"monthly_token_limit"`
	DailyRequestLimit int      `json:"daily_request_limit"`
	ConcurrencyLimit  int      `json:"concurrency_limit"`
	ExpiresAt         *int64   `json:"expires_at"`
	CreatedAt         int64    `json:"created_at"`
	LastUsedAt        *int64   `json:"last_used_at"`
}

const keyCols = `k.id, k.user_id, k.name, k.key_hash, k.key_prefix, k.status, k.allowed_models,
	k.rpm_limit, k.tpm_limit, k.daily_token_limit, k.monthly_token_limit, k.daily_request_limit,
	k.concurrency_limit, k.expires_at, k.created_at, k.last_used_at`

func scanKey(row interface {
	Scan(dest ...any) error
}, withUsername bool) (*APIKey, error) {
	var k APIKey
	var allowed string
	var expires, last sql.NullInt64
	dest := []any{&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.Status, &allowed,
		&k.RPMLimit, &k.TPMLimit, &k.DailyTokenLimit, &k.MonthlyTokenLimit, &k.DailyRequestLimit,
		&k.ConcurrencyLimit, &expires, &k.CreatedAt, &last}
	if withUsername {
		dest = append(dest, &k.Username)
	}
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	k.AllowedModels = decodeStrings(allowed)
	k.ExpiresAt = ptrInt64(expires)
	k.LastUsedAt = ptrInt64(last)
	if k.AllowedModels == nil {
		k.AllowedModels = []string{}
	}
	return &k, nil
}

func (s *Store) CreateKey(k *APIKey) error {
	t := now()
	if k.Status == "" {
		k.Status = "active"
	}
	if k.AllowedModels == nil {
		k.AllowedModels = []string{}
	}
	res, err := s.db.Exec(`INSERT INTO api_keys
		(user_id, name, key_hash, key_prefix, status, allowed_models, rpm_limit, tpm_limit,
		 daily_token_limit, monthly_token_limit, daily_request_limit, concurrency_limit,
		 expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.UserID, k.Name, k.KeyHash, k.KeyPrefix, k.Status, encodeStrings(k.AllowedModels),
		k.RPMLimit, k.TPMLimit, k.DailyTokenLimit, k.MonthlyTokenLimit, k.DailyRequestLimit,
		k.ConcurrencyLimit, nullInt64(k.ExpiresAt), t)
	if err != nil {
		return err
	}
	k.ID, _ = res.LastInsertId()
	k.CreatedAt = t
	return nil
}

func (s *Store) UpdateKey(k *APIKey) error {
	_, err := s.db.Exec(`UPDATE api_keys SET name = ?, status = ?, allowed_models = ?,
		rpm_limit = ?, tpm_limit = ?, daily_token_limit = ?, monthly_token_limit = ?,
		daily_request_limit = ?, concurrency_limit = ?, expires_at = ? WHERE id = ?`,
		k.Name, k.Status, encodeStrings(k.AllowedModels),
		k.RPMLimit, k.TPMLimit, k.DailyTokenLimit, k.MonthlyTokenLimit,
		k.DailyRequestLimit, k.ConcurrencyLimit, nullInt64(k.ExpiresAt), k.ID)
	return err
}

func (s *Store) UpdateKeySecret(id int64, hash, prefix string) error {
	_, err := s.db.Exec(`UPDATE api_keys SET key_hash = ?, key_prefix = ?, last_used_at = NULL WHERE id = ?`,
		hash, prefix, id)
	return err
}

func (s *Store) DeleteKey(id int64) error {
	res, err := s.db.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) KeyByHash(hash string) (*APIKey, error) {
	return scanKey(s.db.QueryRow(`SELECT `+keyCols+` FROM api_keys k WHERE k.key_hash = ?`, hash), false)
}

func (s *Store) KeyByID(id int64) (*APIKey, error) {
	return scanKey(s.db.QueryRow(`SELECT `+keyCols+` FROM api_keys k WHERE k.id = ?`, id), false)
}

// ListKeys lists keys. userID nil means all users (admin view).
func (s *Store) ListKeys(userID *int64, keyword string) ([]APIKey, error) {
	q := `SELECT ` + keyCols + `, u.username FROM api_keys k JOIN users u ON u.id = k.user_id`
	var where []string
	var args []any
	if userID != nil {
		where = append(where, "k.user_id = ?")
		args = append(args, *userID)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		where = append(where, "(k.name LIKE ? OR k.key_prefix LIKE ? OR u.username LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY k.id DESC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		k, err := scanKey(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	if out == nil {
		out = []APIKey{}
	}
	return out, rows.Err()
}

func (s *Store) TouchKeyUse(id int64) error {
	_, err := s.db.Exec(`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, now(), id)
	return err
}
