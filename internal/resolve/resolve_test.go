package resolve

import (
	"context"
	"testing"
	"time"
)

var ctx = context.Background()

// ── StaticResolver ────────────────────────────────────────────────────────────

func TestStaticResolver_Exact(t *testing.T) {
	r := NewStaticResolver(map[string]string{
		"10.0.0.5":    "user-service",
		"db.internal": "postgres",
	})

	cases := []struct {
		host string
		want string
		ok   bool
	}{
		{"10.0.0.5", "user-service", true},
		{"db.internal", "postgres", true},
		{"DB.INTERNAL", "postgres", true}, // case-insensitive
		{"unknown", "", false},
	}

	for _, tc := range cases {
		svc, ok := r.Resolve(ctx, tc.host)
		if ok != tc.ok || svc != tc.want {
			t.Errorf("Resolve(%q): want (%q, %v), got (%q, %v)", tc.host, tc.want, tc.ok, svc, ok)
		}
	}
}

func TestStaticResolver_Wildcard(t *testing.T) {
	r := NewStaticResolver(map[string]string{
		"*.redis.svc":    "redis",
		"*.postgres.svc": "postgres",
	})

	cases := []struct {
		host string
		want string
		ok   bool
	}{
		{"master.redis.svc", "redis", true},
		{"replica-1.redis.svc", "redis", true},
		{"primary.postgres.svc", "postgres", true},
		{"unknown.mysql.svc", "", false},
	}

	for _, tc := range cases {
		svc, ok := r.Resolve(ctx, tc.host)
		if ok != tc.ok || svc != tc.want {
			t.Errorf("Resolve(%q): want (%q, %v), got (%q, %v)", tc.host, tc.want, tc.ok, svc, ok)
		}
	}
}

// ── ChainResolver ─────────────────────────────────────────────────────────────

func TestChainResolver(t *testing.T) {
	first := NewStaticResolver(map[string]string{"10.0.0.1": "service-a"})
	second := NewStaticResolver(map[string]string{"10.0.0.2": "service-b"})
	chain := NewChain(first, second)

	if svc, ok := chain.Resolve(ctx, "10.0.0.1"); !ok || svc != "service-a" {
		t.Errorf("expected service-a, got %q %v", svc, ok)
	}
	if svc, ok := chain.Resolve(ctx, "10.0.0.2"); !ok || svc != "service-b" {
		t.Errorf("expected service-b, got %q %v", svc, ok)
	}
	if _, ok := chain.Resolve(ctx, "10.0.0.3"); ok {
		t.Error("expected not found for 10.0.0.3")
	}
}

func TestChainResolver_FirstWins(t *testing.T) {
	first := NewStaticResolver(map[string]string{"host": "first"})
	second := NewStaticResolver(map[string]string{"host": "second"})
	chain := NewChain(first, second)

	svc, ok := chain.Resolve(ctx, "host")
	if !ok || svc != "first" {
		t.Errorf("expected first resolver to win, got %q", svc)
	}
}

// ── CachingResolver ───────────────────────────────────────────────────────────

func TestCachingResolver_HitsCache(t *testing.T) {
	calls := 0
	inner := &countingResolver{
		delegate: NewStaticResolver(map[string]string{"host": "svc"}),
		calls:    &calls,
	}

	cr := NewCachingResolver(inner, time.Minute, 100)

	cr.Resolve(ctx, "host")
	cr.Resolve(ctx, "host")
	cr.Resolve(ctx, "host")

	if calls != 1 {
		t.Errorf("expected 1 inner call, got %d", calls)
	}
}

func TestCachingResolver_TTLExpiry(t *testing.T) {
	calls := 0
	inner := &countingResolver{
		delegate: NewStaticResolver(map[string]string{"host": "svc"}),
		calls:    &calls,
	}

	cr := NewCachingResolver(inner, 10*time.Millisecond, 100)

	cr.Resolve(ctx, "host")
	time.Sleep(20 * time.Millisecond)
	cr.Resolve(ctx, "host")

	if calls != 2 {
		t.Errorf("expected 2 inner calls after TTL expiry, got %d", calls)
	}
}

func TestCachingResolver_Invalidate(t *testing.T) {
	calls := 0
	inner := &countingResolver{
		delegate: NewStaticResolver(map[string]string{"host": "svc"}),
		calls:    &calls,
	}

	cr := NewCachingResolver(inner, time.Minute, 100)

	cr.Resolve(ctx, "host")
	cr.Invalidate("host")
	cr.Resolve(ctx, "host")

	if calls != 2 {
		t.Errorf("expected 2 calls after invalidation, got %d", calls)
	}
}

func TestCachingResolver_MaxSize(t *testing.T) {
	r := NewStaticResolver(map[string]string{
		"a": "svc-a", "b": "svc-b", "c": "svc-c",
	})
	cr := NewCachingResolver(r, time.Minute, 2)

	cr.Resolve(ctx, "a")
	cr.Resolve(ctx, "b")
	cr.Resolve(ctx, "c") // should evict oldest

	if len(cr.cache) > 2 {
		t.Errorf("cache size %d exceeds maxSize 2", len(cr.cache))
	}
}

// ── helper ────────────────────────────────────────────────────────────────────

type countingResolver struct {
	delegate Resolver
	calls    *int
}

func (c *countingResolver) Resolve(ctx context.Context, host string) (string, bool) {
	*c.calls++
	return c.delegate.Resolve(ctx, host)
}