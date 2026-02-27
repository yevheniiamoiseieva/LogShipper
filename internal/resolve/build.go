package resolve

import (
	"fmt"
	"time"

	"collector/internal/config"
)

// FromConfig builds a Resolver from the resolve section of config.
// Returns nil if no resolvers are configured.
func FromConfig(cfg config.ResolveConfig) (Resolver, error) {
	var resolvers []Resolver

	if len(cfg.Static) > 0 {
		resolvers = append(resolvers, NewStaticResolver(cfg.Static))
	}

	if cfg.Docker {
		dr, err := NewDockerResolver()
		if err != nil {
			return nil, fmt.Errorf("resolve: docker: %w", err)
		}
		resolvers = append(resolvers, dr)
	}

	if len(resolvers) == 0 {
		return nil, nil
	}

	var r Resolver
	if len(resolvers) == 1 {
		r = resolvers[0]
	} else {
		r = NewChain(resolvers...)
	}

	ttl := 30 * time.Second
	if cfg.Cache.TTL != "" {
		if d, err := time.ParseDuration(cfg.Cache.TTL); err == nil {
			ttl = d
		}
	}

	return NewCachingResolver(r, ttl, cfg.Cache.MaxSize), nil
}