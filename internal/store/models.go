package store

import (
	"database/sql"
	"errors"
)

type ModelAlias struct {
	Alias       string  `json:"alias"`
	DisplayName string  `json:"display_name"`
	InputPrice  float64 `json:"input_price"`  // per 1M input tokens
	OutputPrice float64 `json:"output_price"` // per 1M output tokens
	Enabled     bool    `json:"enabled"`
	Note        string  `json:"note"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

type ModelRoute struct {
	ID            int64  `json:"id"`
	Alias         string `json:"alias"`
	ProviderID    int64  `json:"provider_id"`
	ProviderName  string `json:"provider_name,omitempty"`
	UpstreamModel string `json:"upstream_model"`
	Priority      int    `json:"priority"`
	Weight        int    `json:"weight"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

func (s *Store) ListAliases() ([]ModelAlias, error) {
	rows, err := s.db.Query(`SELECT alias, display_name, input_price, output_price, enabled, note,
		created_at, updated_at FROM model_aliases ORDER BY alias ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelAlias
	for rows.Next() {
		var m ModelAlias
		var enabled int
		if err := rows.Scan(&m.Alias, &m.DisplayName, &m.InputPrice, &m.OutputPrice, &enabled,
			&m.Note, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Enabled = enabled != 0
		out = append(out, m)
	}
	if out == nil {
		out = []ModelAlias{}
	}
	return out, rows.Err()
}

func (s *Store) GetAlias(alias string) (*ModelAlias, error) {
	var m ModelAlias
	var enabled int
	err := s.db.QueryRow(`SELECT alias, display_name, input_price, output_price, enabled, note,
		created_at, updated_at FROM model_aliases WHERE alias = ?`, alias).
		Scan(&m.Alias, &m.DisplayName, &m.InputPrice, &m.OutputPrice, &enabled, &m.Note,
			&m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	m.Enabled = enabled != 0
	return &m, nil
}

func (s *Store) UpsertAlias(m *ModelAlias) error {
	t := now()
	enabled := 0
	if m.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(`INSERT INTO model_aliases
		(alias, display_name, input_price, output_price, enabled, note, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(alias) DO UPDATE SET display_name = excluded.display_name,
			input_price = excluded.input_price, output_price = excluded.output_price,
			enabled = excluded.enabled, note = excluded.note, updated_at = excluded.updated_at`,
		m.Alias, m.DisplayName, m.InputPrice, m.OutputPrice, enabled, m.Note, t, t)
	if err != nil {
		return err
	}
	m.CreatedAt, m.UpdatedAt = t, t
	return nil
}

func (s *Store) DeleteAlias(alias string) error {
	res, err := s.db.Exec(`DELETE FROM model_aliases WHERE alias = ?`, alias)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) EnabledAliases() ([]string, error) {
	rows, err := s.db.Query(`SELECT alias FROM model_aliases WHERE enabled = 1 ORDER BY alias`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// RouteTarget is a route joined with its provider, used by the proxy.
type RouteTarget struct {
	Route    ModelRoute
	Provider Provider
}

func (s *Store) RoutesByAlias(alias string) ([]ModelRoute, error) {
	rows, err := s.db.Query(`SELECT r.id, r.alias, r.provider_id, p.name, r.upstream_model,
		r.priority, r.weight, r.enabled, r.created_at, r.updated_at
		FROM model_routes r JOIN providers p ON p.id = r.provider_id
		WHERE r.alias = ? ORDER BY r.priority ASC, r.id ASC`, alias)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelRoute
	for rows.Next() {
		var r ModelRoute
		var enabled int
		if err := rows.Scan(&r.ID, &r.Alias, &r.ProviderID, &r.ProviderName, &r.UpstreamModel,
			&r.Priority, &r.Weight, &enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	if out == nil {
		out = []ModelRoute{}
	}
	return out, rows.Err()
}

// AllRoutes returns every route with its provider name (admin listing).
func (s *Store) AllRoutes() ([]ModelRoute, error) {
	rows, err := s.db.Query(`SELECT r.id, r.alias, r.provider_id, p.name, r.upstream_model,
		r.priority, r.weight, r.enabled, r.created_at, r.updated_at
		FROM model_routes r JOIN providers p ON p.id = r.provider_id
		ORDER BY r.alias ASC, r.priority ASC, r.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelRoute
	for rows.Next() {
		var r ModelRoute
		var enabled int
		if err := rows.Scan(&r.ID, &r.Alias, &r.ProviderID, &r.ProviderName, &r.UpstreamModel,
			&r.Priority, &r.Weight, &enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	if out == nil {
		out = []ModelRoute{}
	}
	return out, rows.Err()
}

func (s *Store) GetRoute(id int64) (*ModelRoute, error) {
	var r ModelRoute
	var enabled int
	err := s.db.QueryRow(`SELECT id, alias, provider_id, upstream_model, priority, weight,
		enabled, created_at, updated_at FROM model_routes WHERE id = ?`, id).
		Scan(&r.ID, &r.Alias, &r.ProviderID, &r.UpstreamModel, &r.Priority, &r.Weight,
			&enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	r.Enabled = enabled != 0
	return &r, nil
}

func (s *Store) CreateRoute(r *ModelRoute) error {
	t := now()
	if r.Weight <= 0 {
		r.Weight = 1
	}
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	res, err := s.db.Exec(`INSERT INTO model_routes
		(alias, provider_id, upstream_model, priority, weight, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Alias, r.ProviderID, r.UpstreamModel, r.Priority, r.Weight, enabled, t, t)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	r.CreatedAt, r.UpdatedAt = t, t
	return nil
}

func (s *Store) UpdateRoute(r *ModelRoute) error {
	r.UpdatedAt = now()
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(`UPDATE model_routes SET provider_id = ?, upstream_model = ?, priority = ?,
		weight = ?, enabled = ?, updated_at = ? WHERE id = ?`,
		r.ProviderID, r.UpstreamModel, r.Priority, r.Weight, enabled, r.UpdatedAt, r.ID)
	return err
}

func (s *Store) DeleteRoute(id int64) error {
	res, err := s.db.Exec(`DELETE FROM model_routes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TargetsForAlias returns enabled routes whose provider is enabled, joined with provider data.
func (s *Store) TargetsForAlias(alias string) ([]RouteTarget, error) {
	rows, err := s.db.Query(`SELECT r.id, r.alias, r.provider_id, r.upstream_model, r.priority,
		r.weight, r.enabled, r.created_at, r.updated_at,
		p.id, p.name, p.type, p.base_url, p.keys_enc, p.extra_headers, p.timeout_sec,
		p.priority, p.weight, p.status, p.health, p.health_msg, p.checked_at, p.created_at, p.updated_at
		FROM model_routes r JOIN providers p ON p.id = r.provider_id
		WHERE r.alias = ? AND r.enabled = 1 AND p.status = 'enabled'
		ORDER BY r.priority ASC, p.priority ASC, r.id ASC`, alias)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RouteTarget
	for rows.Next() {
		var rt RouteTarget
		var rEnabled int
		var checked sql.NullInt64
		p := &rt.Provider
		if err := rows.Scan(&rt.Route.ID, &rt.Route.Alias, &rt.Route.ProviderID, &rt.Route.UpstreamModel,
			&rt.Route.Priority, &rt.Route.Weight, &rEnabled, &rt.Route.CreatedAt, &rt.Route.UpdatedAt,
			&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.KeysEnc, &p.ExtraHeaders, &p.TimeoutSec,
			&p.Priority, &p.Weight, &p.Status, &p.Health, &p.HealthMsg, &checked, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		rt.Route.Enabled = rEnabled != 0
		p.CheckedAt = ptrInt64(checked)
		out = append(out, rt)
	}
	if out == nil {
		out = []RouteTarget{}
	}
	return out, rows.Err()
}

// ModelsWithRouteCounts returns aliases with the number of routes bound to them.
func (s *Store) ModelsWithRouteCounts() ([]struct {
	ModelAlias
	RouteCount int `json:"route_count"`
}, error) {
	aliases, err := s.ListAliases()
	if err != nil {
		return nil, err
	}
	routes, err := s.AllRoutes()
	if err != nil {
		return nil, err
	}
	count := map[string]int{}
	for _, r := range routes {
		count[r.Alias]++
	}
	out := make([]struct {
		ModelAlias
		RouteCount int `json:"route_count"`
	}, 0, len(aliases))
	for _, a := range aliases {
		out = append(out, struct {
			ModelAlias
			RouteCount int `json:"route_count"`
		}{a, count[a.Alias]})
	}
	return out, nil
}
