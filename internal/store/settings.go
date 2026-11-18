package store

import (
	"database/sql"
	"errors"
)

func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (s *Store) AllSettings() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

type Session struct {
	TokenHash string
	UserID    int64
	CreatedAt int64
	ExpiresAt int64
	IP        string
	UserAgent string
}

func (s *Store) CreateSession(sess *Session) error {
	_, err := s.db.Exec(`INSERT INTO sessions (token_hash, user_id, created_at, expires_at, ip, user_agent)
		VALUES (?, ?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.CreatedAt, sess.ExpiresAt, sess.IP, sess.UserAgent)
	return err
}

func (s *Store) SessionByHash(hash string) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(`SELECT token_hash, user_id, created_at, expires_at, ip, user_agent
		FROM sessions WHERE token_hash = ?`, hash).
		Scan(&sess.TokenHash, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt, &sess.IP, &sess.UserAgent)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &sess, nil
}

func (s *Store) DeleteSession(hash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hash)
	return err
}

func (s *Store) DeleteSessionsForUser(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Store) DeleteExpiredSessions(now int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now)
	return err
}

type AuditLog struct {
	ID        int64  `json:"id"`
	Ts        int64  `json:"ts"`
	ActorID   *int64 `json:"actor_id"`
	ActorName string `json:"actor_name"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Detail    string `json:"detail"`
	IP        string `json:"ip"`
}

func (s *Store) AddAudit(actorID *int64, actorName, action, target, detail, ip string) error {
	_, err := s.db.Exec(`INSERT INTO audit_logs (ts, actor_id, actor_name, action, target, detail, ip)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, now(), nullInt64(actorID), actorName, action, target, detail, ip)
	return err
}

func (s *Store) ListAudit(limit, offset int) ([]AuditLog, int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT id, ts, actor_id, actor_name, action, target, detail, ip
		FROM audit_logs ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AuditLog{}
	for rows.Next() {
		var a AuditLog
		var actor sql.NullInt64
		if err := rows.Scan(&a.ID, &a.Ts, &actor, &a.ActorName, &a.Action, &a.Target, &a.Detail, &a.IP); err != nil {
			return nil, 0, err
		}
		a.ActorID = ptrInt64(actor)
		out = append(out, a)
	}
	return out, total, rows.Err()
}
