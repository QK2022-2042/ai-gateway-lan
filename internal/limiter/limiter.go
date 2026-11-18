// Package limiter implements in-memory per-API-key rate limiting:
// requests per minute, tokens per minute and concurrent requests.
package limiter

import (
	"sync"
	"time"
)

type tokenEvent struct {
	at     time.Time
	tokens int64
}

type window struct {
	reqs     []time.Time
	tokens   []tokenEvent
	inflight int
	lastSeen time.Time
}

type Limiter struct {
	mu     sync.Mutex
	wins   map[int64]*window
	window time.Duration
}

func New() *Limiter {
	return &Limiter{wins: map[int64]*window{}, window: time.Minute}
}

// Admit reserves a request slot. It enforces the per-minute request limit and
// the concurrency limit atomically. The returned release function must be
// called exactly once when the request finishes (it is safe to call twice).
func (l *Limiter) Admit(keyID int64, rpm, concurrency int) (release func(), reason string, retryAfter int) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.wins[keyID]
	if w == nil {
		w = &window{}
		l.wins[keyID] = w
	}
	w.lastSeen = now
	cutoff := now.Add(-l.window)
	if len(w.reqs) > 0 {
		i := 0
		for i < len(w.reqs) && !w.reqs[i].After(cutoff) {
			i++
		}
		if i > 0 {
			w.reqs = append(w.reqs[:0], w.reqs[i:]...)
		}
	}
	if rpm > 0 && len(w.reqs) >= rpm {
		retry := int(l.window.Seconds()) - int(now.Sub(w.reqs[0]).Seconds())
		if retry < 1 {
			retry = 1
		}
		return nil, "rpm", retry
	}
	if concurrency > 0 && w.inflight >= concurrency {
		return nil, "concurrency", 1
	}
	w.reqs = append(w.reqs, now)
	w.inflight++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			if cur := l.wins[keyID]; cur != nil && cur.inflight > 0 {
				cur.inflight--
			}
			l.mu.Unlock()
		})
	}, "", 0
}

// TokensInWindow returns tokens recorded for keyID in the last minute.
func (l *Limiter) TokensInWindow(keyID int64) int64 {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.wins[keyID]
	if w == nil {
		return 0
	}
	cutoff := now.Add(-l.window)
	var total int64
	i := 0
	for i < len(w.tokens) && !w.tokens[i].at.After(cutoff) {
		i++
	}
	if i > 0 {
		w.tokens = append(w.tokens[:0], w.tokens[i:]...)
	}
	for _, t := range w.tokens {
		total += t.tokens
	}
	return total
}

func (l *Limiter) RecordTokens(keyID, tokens int64) {
	if tokens <= 0 {
		return
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.wins[keyID]
	if w == nil {
		w = &window{}
		l.wins[keyID] = w
	}
	w.lastSeen = now
	w.tokens = append(w.tokens, tokenEvent{at: now, tokens: tokens})
}

// Inflight reports the current number of in-flight requests for a key.
func (l *Limiter) Inflight(keyID int64) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if w := l.wins[keyID]; w != nil {
		return w.inflight
	}
	return 0
}

// Cleanup drops idle windows. Call periodically.
func (l *Limiter) Cleanup() {
	cutoff := time.Now().Add(-10 * time.Minute)
	l.mu.Lock()
	defer l.mu.Unlock()
	for id, w := range l.wins {
		if w.inflight == 0 && w.lastSeen.Before(cutoff) && len(w.reqs) == 0 && len(w.tokens) == 0 {
			delete(l.wins, id)
		}
	}
}

// LoginLimiter throttles failed login attempts per client address.
type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	window   time.Duration
	max      int
}

func NewLoginLimiter(max int, window time.Duration) *LoginLimiter {
	if max <= 0 {
		max = 10
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &LoginLimiter{attempts: map[string][]time.Time{}, window: window, max: max}
}

// Allowed reports whether another attempt may be made.
func (l *LoginLimiter) Allowed(ip string) (bool, int) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	list := l.attempts[ip]
	cutoff := now.Add(-l.window)
	i := 0
	for i < len(list) && !list[i].After(cutoff) {
		i++
	}
	list = list[i:]
	l.attempts[ip] = list
	if len(list) >= l.max {
		retry := int(l.window.Seconds()) - int(now.Sub(list[0]).Seconds())
		if retry < 1 {
			retry = 1
		}
		return false, retry
	}
	return true, 0
}

func (l *LoginLimiter) Fail(ip string) {
	l.mu.Lock()
	l.attempts[ip] = append(l.attempts[ip], time.Now())
	l.mu.Unlock()
}

func (l *LoginLimiter) Reset(ip string) {
	l.mu.Lock()
	delete(l.attempts, ip)
	l.mu.Unlock()
}

func (l *LoginLimiter) Cleanup() {
	cutoff := time.Now().Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, list := range l.attempts {
		i := 0
		for i < len(list) && !list[i].After(cutoff) {
			i++
		}
		list = list[i:]
		if len(list) == 0 {
			delete(l.attempts, ip)
		} else {
			l.attempts[ip] = list
		}
	}
}
