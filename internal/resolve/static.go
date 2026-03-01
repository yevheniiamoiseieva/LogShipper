package resolve

import (
	"context"
	"path"
	"strings"
)

// StaticResolver resolves hosts from a fixed map defined in config.
// Keys support wildcard patterns like "*.redis.svc".
type StaticResolver struct {
	exact    map[string]string // lowercase host → service
	wildcards []wildcard
}

type wildcard struct {
	pattern string // e.g. "*.redis.svc"
	service string
}

// NewStaticResolver builds a StaticResolver from the config map.
func NewStaticResolver(m map[string]string) *StaticResolver {
	r := &StaticResolver{
		exact: make(map[string]string, len(m)),
	}
	for host, svc := range m {
		lower := strings.ToLower(host)
		if strings.Contains(lower, "*") {
			r.wildcards = append(r.wildcards, wildcard{pattern: lower, service: svc})
		} else {
			r.exact[lower] = svc
		}
	}
	return r
}

func (r *StaticResolver) Resolve(_ context.Context, host string) (string, bool) {
	lower := strings.ToLower(host)

	if svc, ok := r.exact[lower]; ok {
		return svc, true
	}

	for _, w := range r.wildcards {
		if matched, _ := path.Match(w.pattern, lower); matched {
			return w.service, true
		}
	}

	return "", false
}