package resolve

import (
	"context"
	"sync"
	"time"
)

type cacheEntry struct {
	service   string
	ok        bool
	expiresAt time.Time
}

// CachingResolver wraps any Resolver and caches results with a TTL.
type CachingResolver struct {
	inner   Resolver
	ttl     time.Duration
	maxSize int

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

// NewCachingResolver wraps r with a TTL cache.
func NewCachingResolver(r Resolver, ttl time.Duration, maxSize int) *CachingResolver {
	return &CachingResolver{
		inner:   r,
		ttl:     ttl,
		maxSize: maxSize,
		cache:   make(map[string]cacheEntry),
	}
}

func (c *CachingResolver) Resolve(ctx context.Context, host string) (string, bool) {
	c.mu.RLock()
	if e, ok := c.cache[host]; ok && time.Now().Before(e.expiresAt) {
		c.mu.RUnlock()
		return e.service, e.ok
	}
	c.mu.RUnlock()

	service, ok := c.inner.Resolve(ctx, host)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxSize > 0 && len(c.cache) >= c.maxSize {
		c.evictOldest()
	}
	c.cache[host] = cacheEntry{
		service:   service,
		ok:        ok,
		expiresAt: time.Now().Add(c.ttl),
	}

	return service, ok
}

// Invalidate removes a single host from the cache.
func (c *CachingResolver) Invalidate(host string) {
	c.mu.Lock()
	delete(c.cache, host)
	c.mu.Unlock()
}

func (c *CachingResolver) evictOldest() {
	var oldest string
	var oldestTime time.Time
	for host, e := range c.cache {
		if oldest == "" || e.expiresAt.Before(oldestTime) {
			oldest = host
			oldestTime = e.expiresAt
		}
	}
	if oldest != "" {
		delete(c.cache, oldest)
	}
}