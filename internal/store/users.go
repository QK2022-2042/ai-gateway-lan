package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")

type User struct {
	ID                int64    `json:"id"`
	Username          string   `json:"username"`
	PasswordHash      string   `json:"-"`
	DisplayName       string   `json:"display_name"`
	Role              string   `json:"role"`
	Status            string   `json:"status"`
	Note              string   `json:"note"`
	AllowedModels     []string `json:"allowed_models"`
	RPMLimit          int      `json:"rpm_limit"`
	TPMLimit          int      `json:"tpm_limit"`
	DailyRequestLimit int      `json:"daily_request_limit"`
	DailyTokenLimit   int      `json:"daily_token_limit"`
	MonthlyTokenLimit int      `json:"monthly_token_limit"`
	ConcurrencyLimit  int      `json:"concurrency_limit"`
	CreatedAt         int64    `json:"created_at"`
	UpdatedAt         int64    `json:"updated_at"`
	LastLoginAt       *int64   `json:"last_login_at"`
}

func scanUser(row interface {
	Scan(dest ...any) error
}) (*User, error) {
	var u User
	var allowed string
	var last sql.NullInt64
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role,
		&u.Status, &u.Note, &allowed, &u.RPMLimit, &u.TPMLimit, &u.DailyRequestLimit,
		&u.DailyTokenLimit, &u.MonthlyTokenLimit, &u.ConcurrencyLimit,
		&u.CreatedAt, &u.UpdatedAt, &last)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.AllowedModels = decodeStrings(allowed)
	u.LastLoginAt = ptrInt64(last)
	return &u, nil
}

const userCols = `id, username, password_hash, display_name, role, status, note, allowed_models,
	rpm_limit, tpm_limit, daily_request_limit, daily_token_limit, monthly_token_limit, concurrency_limit,
	created_at, updated_at, last_login_at`

func (s *Store) UserByID(id int64) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

func (s *Store) UserByUsername(username string) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE username = ?`, username))
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active'`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(u *User) error {
	t := now()
	if u.Status == "" {
		u.Status = "active"
	}
	if u.Role == "" {
		u.Role = "user"
	}
	if u.AllowedModels == nil {
		u.AllowedModels = []string{"*"}
	}
	res, err := s.db.Exec(`INSERT INTO users
		(username, password_hash, display_name, role, status, note, allowed_models,
		 rpm_limit, tpm_limit, daily_request_limit, daily_token_limit, monthly_token_limit,
		 concurrency_limit, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.DisplayName, u.Role, u.Status, u.Note,
		encodeStrings(u.AllowedModels), u.RPMLimit, u.TPMLimit, u.DailyRequestLimit,
		u.DailyTokenLimit, u.MonthlyTokenLimit, u.ConcurrencyLimit, t, t)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	u.CreatedAt, u.UpdatedAt = t, t
	return nil
}

func (s *Store) UpdateUser(u *User) error {
	u.UpdatedAt = now()
	_, err := s.db.Exec(`UPDATE users SET display_name = ?, role = ?, status = ?, note = ?,
		allowed_models = ?, rpm_limit = ?, tpm_limit = ?, daily_request_limit = ?,
		daily_token_limit = ?, monthly_token_limit = ?, concurrency_limit = ?,
		updated_at = ? WHERE id = ?`,
		u.DisplayName, u.Role, u.Status, u.Note, encodeStrings(u.AllowedModels),
		u.RPMLimit, u.TPMLimit, u.DailyRequestLimit, u.DailyTokenLimit,
		u.MonthlyTokenLimit, u.ConcurrencyLimit, u.UpdatedAt, u.ID)
	return err
}

func (s *Store) UpdatePassword(id int64, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, hash, now(), id)
	return err
}

func (s *Store) TouchLogin(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, now(), id)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListUsers returns users, optionally filtered by a fuzzy keyword.
func (s *Store) ListUsers(keyword string) ([]User, error) {
	q := `SELECT ` + userCols + ` FROM users`
	var args []any
	if kw := strings.TrimSpace(keyword); kw != "" {
		q += ` WHERE username LIKE ? OR display_name LIKE ? OR note LIKE ?`
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	q += ` ORDER BY id ASC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	if out == nil {
		out = []User{}
	}
	return out, rows.Err()
}

// UserUsageRow is a per-user aggregate used by the admin user list.
type UserUsageRow struct {
	UserID       int64   `json:"user_id"`
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	KeyCount     int     `json:"key_count"`
	LastActiveAt *int64  `json:"last_active_at"`
}

// UsageByUser aggregates successful usage for a local day. It deliberately
// reads usage_daily (not request_logs) so the numbers line up with the quotas
// enforced by the gateway; rejected requests do not consume quota.
func (s *Store) UsageByUser(day string) (map[int64]UserUsageRow, error) {
	rows, err := s.db.Query(`SELECT user_id, COALESCE(SUM(requests),0),
		COALESCE(SUM(prompt_tokens + completion_tokens),0), COALESCE(SUM(cost),0)
		FROM usage_daily WHERE day = ? AND user_id > 0 GROUP BY user_id`, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]UserUsageRow{}
	for rows.Next() {
		var r UserUsageRow
		if err := rows.Scan(&r.UserID, &r.Requests, &r.Tokens, &r.Cost); err != nil {
			return nil, err
		}
		out[r.UserID] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows2, err := s.db.Query(`SELECT user_id, COUNT(*) FROM api_keys GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var uid int64
		var n int
		if err := rows2.Scan(&uid, &n); err != nil {
			return nil, err
		}
		r := out[uid]
		r.UserID = uid
		r.KeyCount = n
		out[uid] = r
	}
	return out, rows2.Err()
}

func (s *Store) ensureAdminExists() (bool, error) {
	n, err := s.CountUsers()
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

var _ = fmt.Sprintf
