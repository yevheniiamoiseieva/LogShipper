package resolve

import "context"

// ChainResolver tries each resolver in order, returning the first success.
type ChainResolver struct {
	resolvers []Resolver
}

// NewChain creates a ChainResolver from the given resolvers.
func NewChain(resolvers ...Resolver) *ChainResolver {
	return &ChainResolver{resolvers: resolvers}
}

func (c *ChainResolver) Resolve(ctx context.Context, host string) (string, bool) {
	for _, r := range c.resolvers {
		if svc, ok := r.Resolve(ctx, host); ok {
			return svc, true
		}
	}
	return "", false
}