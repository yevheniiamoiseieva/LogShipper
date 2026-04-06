package resolve

import "context"

// Resolver maps a network address (IP or hostname) to a logical service name.
type Resolver interface {
	Resolve(ctx context.Context, host string) (serviceName string, ok bool)
}