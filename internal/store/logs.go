package store

import (
	"database/sql"
	"strings"
)

type RequestLog struct {
	ID               int64   `json:"id"`
	Ts               int64   `json:"ts"`
	RequestID        string  `json:"request_id"`
	UserID           *int64  `json:"user_id"`
	Username         string  `json:"username,omitempty"`
	KeyID            *int64  `json:"key_id"`
	KeyName          string  `json:"key_name,omitempty"`
	ProviderID       *int64  `json:"provider_id"`
	ProviderName     string  `json:"provider_name,omitempty"`
	Model            string  `json:"model"`
	UpstreamModel    string  `json:"upstream_model"`
	Status           int     `json:"status"`
	Error            string  `json:"error"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	LatencyMs        int64   `json:"latency_ms"`
	FirstTokenMs     int64   `json:"first_token_ms"`
	Stream           bool    `json:"stream"`
	Retries          int     `json:"retries"`
	ClientIP         string  `json:"client_ip"`
	UserAgent        string  `json:"user_agent"`
}

const insertLogSQL = `INSERT INTO request_logs
	(ts, request_id, user_id, key_id, provider_id, model, upstream_model, status, error,
	 prompt_tokens, completion_tokens, total_tokens, cost, latency_ms, first_token_ms,
	 stream, retries, client_ip, user_agent)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func (s *Store) insertLogArgs(l *RequestLog) []any {
	stream := 0
	if l.Stream {
		stream = 1
	}
	return []any{l.Ts, l.RequestID, nullInt64(l.UserID), nullInt64(l.KeyID), nullInt64(l.ProviderID),
		l.Model, l.UpstreamModel, l.Status, l.Error, l.PromptTokens, l.CompletionTokens,
		l.TotalTokens, l.Cost, l.LatencyMs, l.FirstTokenMs, stream, l.Retries,
		l.ClientIP, l.UserAgent}
}

// InsertLog records a request without touching quota counters (rejections).
func (s *Store) InsertLog(l *RequestLog) error {
	_, err := s.db.Exec(insertLogSQL, s.insertLogArgs(l)...)
	return err
}

// InsertLogAndUsage writes a request log and folds its usage into the daily
// per-key counters in one transaction.
func (s *Store) InsertLogAndUsage(l *RequestLog, day string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(insertLogSQL, s.insertLogArgs(l)...); err != nil {
		return err
	}
	if l.KeyID != nil {
		var userID int64
		if l.UserID != nil {
			userID = *l.UserID
		}
		if _, err := tx.Exec(`INSERT INTO usage_daily
			(key_id, user_id, day, requests, prompt_tokens, completion_tokens, cost)
			VALUES (?, ?, ?, 1, ?, ?, ?)
			ON CONFLICT(key_id, day) DO UPDATE SET
				user_id = excluded.user_id,
				requests = requests + 1,
				prompt_tokens = prompt_tokens + excluded.prompt_tokens,
				completion_tokens = completion_tokens + excluded.completion_tokens,
				cost = cost + excluded.cost`,
			*l.KeyID, userID, day, l.PromptTokens, l.CompletionTokens, l.Cost); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type LogFilter struct {
	From    int64
	To      int64
	UserID  *int64
	KeyID   *int64
	Model   string
	Status  string // "", "success", "error"
	Keyword string
	Limit   int
	Offset  int
}

func (f *LogFilter) where() (string, []any) {
	var parts []string
	var args []any
	if f.From > 0 {
		parts = append(parts, "l.ts >= ?")
		args = append(args, f.From)
	}
	if f.To > 0 {
		parts = append(parts, "l.ts < ?")
		args = append(args, f.To)
	}
	if f.UserID != nil {
		parts = append(parts, "l.user_id = ?")
		args = append(args, *f.UserID)
	}
	if f.KeyID != nil {
		parts = append(parts, "l.key_id = ?")
		args = append(args, *f.KeyID)
	}
	if m := strings.TrimSpace(f.Model); m != "" {
		parts = append(parts, "l.model = ?")
		args = append(args, m)
	}
	switch f.Status {
	case "success":
		parts = append(parts, "l.status >= 200 AND l.status < 400")
	case "error":
		parts = append(parts, "(l.status < 200 OR l.status >= 400)")
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		parts = append(parts, "(l.request_id LIKE ? OR l.error LIKE ? OR l.client_ip LIKE ? OR l.model LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

func (s *Store) QueryLogs(f *LogFilter) ([]RequestLog, int, error) {
	where, args := f.where()
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM request_logs l`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	q := `SELECT l.id, l.ts, l.request_id, l.user_id, COALESCE(u.username,''), l.key_id,
		COALESCE(k.name,''), l.provider_id, COALESCE(p.name,''), l.model, l.upstream_model,
		l.status, l.error, l.prompt_tokens, l.completion_tokens, l.total_tokens, l.cost,
		l.latency_ms, l.first_token_ms, l.stream, l.retries, l.client_ip, l.user_agent
		FROM request_logs l
		LEFT JOIN users u ON u.id = l.user_id
		LEFT JOIN api_keys k ON k.id = l.key_id
		LEFT JOIN providers p ON p.id = l.provider_id` + where +
		` ORDER BY l.id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, f.Offset)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []RequestLog{}
	for rows.Next() {
		var l RequestLog
		var uid, kid, pid sql.NullInt64
		var stream int
		if err := rows.Scan(&l.ID, &l.Ts, &l.RequestID, &uid, &l.Username, &kid, &l.KeyName,
			&pid, &l.ProviderName, &l.Model, &l.UpstreamModel, &l.Status, &l.Error,
			&l.PromptTokens, &l.CompletionTokens, &l.TotalTokens, &l.Cost, &l.LatencyMs,
			&l.FirstTokenMs, &stream, &l.Retries, &l.ClientIP, &l.UserAgent); err != nil {
			return nil, 0, err
		}
		l.UserID = ptrInt64(uid)
		l.KeyID = ptrInt64(kid)
		l.ProviderID = ptrInt64(pid)
		l.Stream = stream != 0
		out = append(out, l)
	}
	return out, total, rows.Err()
}

type Overview struct {
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Tokens           int64   `json:"tokens"`
	Cost             float64 `json:"cost"`
	Success          int64   `json:"success"`
	Errors           int64   `json:"errors"`
	SuccessRate      float64 `json:"success_rate"`
	AvgLatencyMs     int64   `json:"avg_latency_ms"`
	P95LatencyMs     int64   `json:"p95_latency_ms"`
	ActiveUsers      int64   `json:"active_users"`
	ActiveKeys       int64   `json:"active_keys"`
}

func (s *Store) Overview(from, to int64) (Overview, error) {
	var o Overview
	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(prompt_tokens),0),
		COALESCE(SUM(completion_tokens),0), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost),0),
		COALESCE(SUM(CASE WHEN status >= 200 AND status < 400 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status < 200 OR status >= 400 THEN 1 ELSE 0 END),0),
		CAST(COALESCE(AVG(latency_ms),0) AS INTEGER), COUNT(DISTINCT user_id), COUNT(DISTINCT key_id)
		FROM request_logs WHERE ts >= ? AND ts < ?`, from, to).
		Scan(&o.Requests, &o.PromptTokens, &o.CompletionTokens, &o.Tokens, &o.Cost,
			&o.Success, &o.Errors, &o.AvgLatencyMs, &o.ActiveUsers, &o.ActiveKeys)
	if err != nil {
		return o, err
	}
	if o.Requests > 0 {
		o.SuccessRate = float64(o.Success) / float64(o.Requests)
	}
	// p95 over successful requests
	if o.Success > 0 {
		offset := o.Success * 95 / 100
		var p95 sql.NullInt64
		_ = s.db.QueryRow(`SELECT latency_ms FROM request_logs
			WHERE ts >= ? AND ts < ? AND status >= 200 AND status < 400
			ORDER BY latency_ms ASC LIMIT 1 OFFSET ?`, from, to, offset).Scan(&p95)
		if p95.Valid {
			o.P95LatencyMs = p95.Int64
		}
	}
	return o, nil
}

type Point struct {
	Ts       int64   `json:"ts"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Errors   int64   `json:"errors"`
	Cost     float64 `json:"cost"`
}

func (s *Store) Timeseries(from, to, bucket int64) ([]Point, error) {
	if bucket <= 0 {
		bucket = 3600
	}
	rows, err := s.db.Query(`SELECT (ts / ?) * ? AS b, COUNT(*),
		COALESCE(SUM(total_tokens),0),
		COALESCE(SUM(CASE WHEN status < 200 OR status >= 400 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(cost),0)
		FROM request_logs WHERE ts >= ? AND ts < ? GROUP BY b ORDER BY b ASC`,
		bucket, bucket, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := map[int64]Point{}
	for rows.Next() {
		var p Point
		if err := rows.Scan(&p.Ts, &p.Requests, &p.Tokens, &p.Errors, &p.Cost); err != nil {
			return nil, err
		}
		got[p.Ts] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []Point{}
	for ts := (from / bucket) * bucket; ts < to; ts += bucket {
		p, ok := got[ts]
		if !ok {
			p = Point{Ts: ts}
		}
		out = append(out, p)
	}
	return out, nil
}

type BreakdownRow struct {
	Name     string  `json:"name"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
	Errors   int64   `json:"errors"`
	AvgMs    int64   `json:"avg_ms"`
}

func (s *Store) BreakdownByModel(from, to int64, limit int) ([]BreakdownRow, error) {
	return s.breakdown(`SELECT model, COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost),0),
		COALESCE(SUM(CASE WHEN status < 200 OR status >= 400 THEN 1 ELSE 0 END),0),
		CAST(COALESCE(AVG(latency_ms),0) AS INTEGER)
		FROM request_logs WHERE ts >= ? AND ts < ? AND model != ''
		GROUP BY model ORDER BY COUNT(*) DESC LIMIT ?`, from, to, limit)
}

func (s *Store) BreakdownByUser(from, to int64, limit int) ([]BreakdownRow, error) {
	return s.breakdown(`SELECT COALESCE(u.username, '#' || l.user_id), COUNT(*),
		COALESCE(SUM(l.total_tokens),0), COALESCE(SUM(l.cost),0),
		COALESCE(SUM(CASE WHEN l.status < 200 OR l.status >= 400 THEN 1 ELSE 0 END),0),
		CAST(COALESCE(AVG(l.latency_ms),0) AS INTEGER)
		FROM request_logs l LEFT JOIN users u ON u.id = l.user_id
		WHERE l.ts >= ? AND l.ts < ? AND l.user_id IS NOT NULL
		GROUP BY l.user_id ORDER BY COUNT(*) DESC LIMIT ?`, from, to, limit)
}

func (s *Store) BreakdownByProvider(from, to int64, limit int) ([]BreakdownRow, error) {
	return s.breakdown(`SELECT COALESCE(p.name, '未知'), COUNT(*),
		COALESCE(SUM(l.total_tokens),0), COALESCE(SUM(l.cost),0),
		COALESCE(SUM(CASE WHEN l.status < 200 OR l.status >= 400 THEN 1 ELSE 0 END),0),
		CAST(COALESCE(AVG(l.latency_ms),0) AS INTEGER)
		FROM request_logs l LEFT JOIN providers p ON p.id = l.provider_id
		WHERE l.ts >= ? AND l.ts < ? AND l.provider_id IS NOT NULL
		GROUP BY l.provider_id ORDER BY COUNT(*) DESC LIMIT ?`, from, to, limit)
}

func (s *Store) breakdown(q string, args ...any) ([]BreakdownRow, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BreakdownRow{}
	for rows.Next() {
		var r BreakdownRow
		if err := rows.Scan(&r.Name, &r.Requests, &r.Tokens, &r.Cost, &r.Errors, &r.AvgMs); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type KeyUsage struct {
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

func (s *Store) KeyUsageOnDay(keyID int64, day string) (KeyUsage, error) {
	var u KeyUsage
	err := s.db.QueryRow(`SELECT COALESCE(SUM(requests),0),
		COALESCE(SUM(prompt_tokens + completion_tokens),0), COALESCE(SUM(cost),0)
		FROM usage_daily WHERE key_id = ? AND day = ?`, keyID, day).
		Scan(&u.Requests, &u.Tokens, &u.Cost)
	return u, err
}

func (s *Store) KeyUsageOnMonth(keyID int64, monthPrefix string) (KeyUsage, error) {
	var u KeyUsage
	err := s.db.QueryRow(`SELECT COALESCE(SUM(requests),0),
		COALESCE(SUM(prompt_tokens + completion_tokens),0), COALESCE(SUM(cost),0)
		FROM usage_daily WHERE key_id = ? AND day LIKE ?`, keyID, monthPrefix+"-%").
		Scan(&u.Requests, &u.Tokens, &u.Cost)
	return u, err
}

// KeyUsageByDay returns usage for every key on a given local day.
func (s *Store) KeyUsageByDay(day string) (map[int64]KeyUsage, error) {
	rows, err := s.db.Query(`SELECT key_id, COALESCE(requests,0),
		COALESCE(prompt_tokens + completion_tokens,0), COALESCE(cost,0)
		FROM usage_daily WHERE day = ?`, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]KeyUsage{}
	for rows.Next() {
		var id int64
		var u KeyUsage
		if err := rows.Scan(&id, &u.Requests, &u.Tokens, &u.Cost); err != nil {
			return nil, err
		}
		out[id] = u
	}
	return out, rows.Err()
}

func (s *Store) UserUsageOnDay(userID int64, day string) (KeyUsage, error) {
	var u KeyUsage
	err := s.db.QueryRow(`SELECT COALESCE(SUM(requests),0),
		COALESCE(SUM(prompt_tokens + completion_tokens),0), COALESCE(SUM(cost),0)
		FROM usage_daily WHERE user_id = ? AND day = ?`, userID, day).
		Scan(&u.Requests, &u.Tokens, &u.Cost)
	return u, err
}

func (s *Store) UserUsageOnMonth(userID int64, monthPrefix string) (KeyUsage, error) {
	var u KeyUsage
	err := s.db.QueryRow(`SELECT COALESCE(SUM(requests),0),
		COALESCE(SUM(prompt_tokens + completion_tokens),0), COALESCE(SUM(cost),0)
		FROM usage_daily WHERE user_id = ? AND day LIKE ?`, userID, monthPrefix+"-%").
		Scan(&u.Requests, &u.Tokens, &u.Cost)
	return u, err
}

func (s *Store) RecentErrors(limit int) ([]RequestLog, error) {
	logs, _, err := s.QueryLogs(&LogFilter{Status: "error", Limit: limit})
	return logs, err
}

func (s *Store) DeleteLogsBefore(ts int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM request_logs WHERE ts < ?`, ts)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) DeleteUsageBefore(day string) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM usage_daily WHERE day < ?`, day)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CountLogs returns the number of retained request logs.
func (s *Store) CountLogs() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM request_logs`).Scan(&n)
	return n, err
}
