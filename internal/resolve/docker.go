package resolve

import (
	"context"
	"regexp"
	"strings"

	"github.com/docker/docker/client"
)

// DockerResolver resolves container hostnames and IPs via the Docker API.
// It extracts service names from the com.docker.compose.service label,
// falling back to the container name with replica suffixes stripped.
type DockerResolver struct {
	client *client.Client
}

// NewDockerResolver creates a DockerResolver using the default Docker socket.
func NewDockerResolver() (*DockerResolver, error) {
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerResolver{client: c}, nil
}

var replicaSuffix = regexp.MustCompile(`[-_]\d+$`)

func (r *DockerResolver) Resolve(ctx context.Context, host string) (string, bool) {
	info, err := r.client.ContainerInspect(ctx, host)
	if err != nil {
		return "", false
	}

	if svc, ok := info.Config.Labels["com.docker.compose.service"]; ok && svc != "" {
		return svc, true
	}

	name := strings.TrimPrefix(info.Name, "/")
	name = replicaSuffix.ReplaceAllString(name, "")
	if name != "" {
		return name, true
	}

	return "", false
}