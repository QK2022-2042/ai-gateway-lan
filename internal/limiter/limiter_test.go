package limiter

import "testing"

func TestRPMAndConcurrency(t *testing.T) {
	l := New()
	release1, reason, _ := l.Admit(1, 2, 0)
	if reason != "" {
		t.Fatalf("first admission rejected: %s", reason)
	}
	if _, reason, _ = l.Admit(1, 2, 0); reason != "" {
		t.Fatalf("second admission rejected: %s", reason)
	}
	if _, reason, retry := l.Admit(1, 2, 0); reason != "rpm" || retry <= 0 {
		t.Fatalf("third admission should hit rpm, got %q retry=%d", reason, retry)
	}
	release1()

	l2 := New()
	rel, reason, _ := l2.Admit(9, 0, 1)
	if reason != "" {
		t.Fatalf("first concurrency rejected: %s", reason)
	}
	if _, reason, _ = l2.Admit(9, 0, 1); reason != "concurrency" {
		t.Fatalf("expected concurrency rejection, got %q", reason)
	}
	rel()
	if _, reason, _ = l2.Admit(9, 0, 1); reason != "" {
		t.Fatalf("after release should admit, got %q", reason)
	}
}

func TestTokenWindow(t *testing.T) {
	l := New()
	l.RecordTokens(5, 100)
	l.RecordTokens(5, 50)
	if got := l.TokensInWindow(5); got != 150 {
		t.Fatalf("tokens=%d", got)
	}
	if got := l.TokensInWindow(6); got != 0 {
		t.Fatalf("unexpected tokens for other key: %d", got)
	}
}

func TestLoginLimiter(t *testing.T) {
	l := NewLoginLimiter(2, 60_000_000_000)
	if ok, _ := l.Allowed("1.2.3.4"); !ok {
		t.Fatal("first attempt should be allowed")
	}
	l.Fail("1.2.3.4")
	l.Fail("1.2.3.4")
	if ok, _ := l.Allowed("1.2.3.4"); ok {
		t.Fatal("third attempt should be blocked")
	}
	l.Reset("1.2.3.4")
	if ok, _ := l.Allowed("1.2.3.4"); !ok {
		t.Fatal("reset should unblock")
	}
}
