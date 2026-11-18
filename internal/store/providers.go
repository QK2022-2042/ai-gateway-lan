package store

import (
	"database/sql"
	"errors"
)

type Provider struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	BaseURL      string `json:"base_url"`
	KeysEnc      string `json:"-"`
	ExtraHeaders string `json:"extra_headers"`
	TimeoutSec   int    `json:"timeout_sec"`
	Priority     int    `json:"priority"`
	Weight       int    `json:"weight"`
	Status       string `json:"status"`
	Health       string `json:"health"`
	HealthMsg    string `json:"health_msg"`
	CheckedAt    *int64 `json:"checked_at"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

const providerCols = `id, name, type, base_url, keys_enc, extra_headers, timeout_sec,
	priority, weight, status, health, health_msg, checked_at, created_at, updated_at`

func scanProvider(row interface {
	Scan(dest ...any) error
}) (*Provider, error) {
	var p Provider
	var checked sql.NullInt64
	err := row.Scan(&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.KeysEnc, &p.ExtraHeaders,
		&p.TimeoutSec, &p.Priority, &p.Weight, &p.Status, &p.Health, &p.HealthMsg,
		&checked, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.CheckedAt = ptrInt64(checked)
	return &p, nil
}

func (s *Store) ListProviders() ([]Provider, error) {
	rows, err := s.db.Query(`SELECT ` + providerCols + ` FROM providers ORDER BY priority ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Provider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if out == nil {
		out = []Provider{}
	}
	return out, rows.Err()
}

func (s *Store) GetProvider(id int64) (*Provider, error) {
	return scanProvider(s.db.QueryRow(`SELECT `+providerCols+` FROM providers WHERE id = ?`, id))
}

func (s *Store) CreateProvider(p *Provider) error {
	t := now()
	if p.Type == "" {
		p.Type = "openai"
	}
	if p.Status == "" {
		p.Status = "enabled"
	}
	if p.Health == "" {
		p.Health = "unknown"
	}
	if p.TimeoutSec <= 0 {
		p.TimeoutSec = 120
	}
	if p.Weight <= 0 {
		p.Weight = 1
	}
	res, err := s.db.Exec(`INSERT INTO providers
		(name, type, base_url, keys_enc, extra_headers, timeout_sec, priority, weight,
		 status, health, health_msg, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Type, p.BaseURL, p.KeysEnc, p.ExtraHeaders, p.TimeoutSec, p.Priority,
		p.Weight, p.Status, p.Health, p.HealthMsg, t, t)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	p.CreatedAt, p.UpdatedAt = t, t
	return nil
}

func (s *Store) UpdateProvider(p *Provider) error {
	p.UpdatedAt = now()
	_, err := s.db.Exec(`UPDATE providers SET name = ?, type = ?, base_url = ?, keys_enc = ?,
		extra_headers = ?, timeout_sec = ?, priority = ?, weight = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		p.Name, p.Type, p.BaseURL, p.KeysEnc, p.ExtraHeaders, p.TimeoutSec, p.Priority,
		p.Weight, p.Status, p.UpdatedAt, p.ID)
	return err
}

func (s *Store) DeleteProvider(id int64) error {
	res, err := s.db.Exec(`DELETE FROM providers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetProviderHealth(id int64, health, msg string) error {
	_, err := s.db.Exec(`UPDATE providers SET health = ?, health_msg = ?, checked_at = ? WHERE id = ?`,
		health, msg, now(), id)
	return err
}
