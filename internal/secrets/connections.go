package secrets

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	connectionProviderStaticToken      = "static_token"
	connectionProviderEnv              = "env"
	connectionProviderGitHubUserDevice = "github_user_device"
)

type ConnectionRef struct {
	Service string
	Name    string
}

type Connection struct {
	Service  string
	Name     string
	Provider string
}

type ConnectionEnvResolver interface {
	ResolveConnectionEnv(ctx context.Context, ref string, name string) (string, error)
	ConnectionConfigured(ref string) bool
	ListConnections() []Connection
	TestConnection(ctx context.Context, ref string) error
}

type TokenConnectionResolver interface {
	ResolveConnectionToken(ctx context.Context, ref string) (string, error)
}

type connectionEntry struct {
	Provider    string                        `yaml:"provider"`
	TokenSecret string                        `yaml:"tokenSecret"`
	Env         map[string]connectionEnvValue `yaml:"env"`
	ClientID    string                        `yaml:"clientID"`
	TokenFile   string                        `yaml:"tokenFile"`
}

type connectionEnvValue struct {
	Secret string `yaml:"secret"`
}

func (r *FileResolver) ResolveConnectionToken(ctx context.Context, ref string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	_, entry, err := r.connection(ref)
	if err != nil {
		return "", err
	}
	switch entry.Provider {
	case connectionProviderStaticToken:
		if entry.TokenSecret == "" {
			return "", fmt.Errorf("connection %q tokenSecret is required", ref)
		}
		value, err := r.Resolve(ctx, entry.TokenSecret)
		if err != nil {
			return "", fmt.Errorf("connection %q token: %w", ref, err)
		}
		return value, nil
	case connectionProviderGitHubUserDevice:
		return r.resolveGitHubUserDeviceToken(ctx, ref, entry)
	case connectionProviderEnv:
		return "", fmt.Errorf("connection %q provider %q does not resolve tokens", ref, entry.Provider)
	default:
		return "", unsupportedConnectionProviderError(ref, entry.Provider)
	}
}

func (r *FileResolver) ResolveConnectionEnv(ctx context.Context, ref string, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	_, entry, err := r.connection(ref)
	if err != nil {
		return "", err
	}
	switch entry.Provider {
	case connectionProviderEnv:
		env, ok := entry.Env[name]
		if !ok {
			return "", fmt.Errorf("connection %q does not configure env %q", ref, name)
		}
		if env.Secret == "" {
			return "", fmt.Errorf("connection %q env %q secret is required", ref, name)
		}
		value, err := r.Resolve(ctx, env.Secret)
		if err != nil {
			return "", fmt.Errorf("connection %q env %q: %w", ref, name, err)
		}
		return value, nil
	case connectionProviderStaticToken, connectionProviderGitHubUserDevice:
		return "", fmt.Errorf("connection %q provider %q does not resolve env %q", ref, entry.Provider, name)
	default:
		return "", unsupportedConnectionProviderError(ref, entry.Provider)
	}
}

func (r *FileResolver) ConnectionConfigured(ref string) bool {
	parsed, err := parseConnectionRef(ref)
	if err != nil {
		return false
	}
	byName, ok := r.config.Connections[parsed.Service]
	if !ok {
		return false
	}
	_, ok = byName[parsed.Name]
	return ok
}

func (r *FileResolver) ListConnections() []Connection {
	var out []Connection
	for service, byName := range r.config.Connections {
		for name, entry := range byName {
			out = append(out, Connection{
				Service:  service,
				Name:     name,
				Provider: entry.Provider,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i].Service + "." + out[i].Name
		right := out[j].Service + "." + out[j].Name
		return left < right
	})
	return out
}

func (r *FileResolver) TestConnection(ctx context.Context, ref string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, entry, err := r.connection(ref)
	if err != nil {
		return err
	}
	switch entry.Provider {
	case connectionProviderStaticToken:
		_, err := r.ResolveConnectionToken(ctx, ref)
		return err
	case connectionProviderGitHubUserDevice:
		_, err := r.ResolveConnectionToken(ctx, ref)
		return err
	case connectionProviderEnv:
		if len(entry.Env) == 0 {
			return fmt.Errorf("connection %q env must configure at least one variable", ref)
		}
		names := make([]string, 0, len(entry.Env))
		for name := range entry.Env {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, err := r.ResolveConnectionEnv(ctx, ref, name); err != nil {
				return err
			}
		}
		return nil
	default:
		return unsupportedConnectionProviderError(ref, entry.Provider)
	}
}

func (r *FileResolver) connection(ref string) (ConnectionRef, connectionEntry, error) {
	parsed, err := parseConnectionRef(ref)
	if err != nil {
		return ConnectionRef{}, connectionEntry{}, err
	}
	byName, ok := r.config.Connections[parsed.Service]
	if !ok {
		return ConnectionRef{}, connectionEntry{}, fmt.Errorf("connection %q is not configured", ref)
	}
	entry, ok := byName[parsed.Name]
	if !ok {
		return ConnectionRef{}, connectionEntry{}, fmt.Errorf("connection %q is not configured", ref)
	}
	return parsed, entry, nil
}

func parseConnectionRef(ref string) (ConnectionRef, error) {
	service, name, ok := strings.Cut(ref, ".")
	if !ok || service == "" || name == "" || strings.Contains(name, ".") {
		return ConnectionRef{}, fmt.Errorf("connection ref %q must be service.name", ref)
	}
	return ConnectionRef{Service: service, Name: name}, nil
}

func unsupportedConnectionProviderError(ref, provider string) error {
	return fmt.Errorf("connection %q provider %q is unsupported", ref, provider)
}
