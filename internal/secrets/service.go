package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Resolver interface {
	Resolve(ctx context.Context, id string) (string, error)
	ResolveProfileEnv(ctx context.Context, profile, name string) (string, error)
}

type ProfileInspector interface {
	ProfileEnvConfigured(profile, name string) bool
}

type Service struct{}

type FileResolver struct {
	configDir       string
	config          fileConfig
	githubRefreshMu sync.Mutex
}

type fileConfig struct {
	Secrets     map[string]secretEntry                `yaml:"secrets"`
	Profiles    map[string]profileEntry               `yaml:"profiles"`
	Connections map[string]map[string]connectionEntry `yaml:"connections"`
}

type secretEntry struct {
	File string `yaml:"file"`
}

type profileEntry struct {
	Env map[string]profileEnvValue `yaml:"env"`
}

type profileEnvValue struct {
	Secret string `yaml:"secret"`
}

func NewFileResolver(configDir string) (*FileResolver, error) {
	data, err := os.ReadFile(filepath.Join(configDir, "secrets.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return &FileResolver{configDir: configDir, config: fileConfig{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg fileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &FileResolver{configDir: configDir, config: cfg}, nil
}

func (r *FileResolver) Resolve(ctx context.Context, id string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	entry, ok := r.config.Secrets[id]
	if !ok {
		return "", fmt.Errorf("secret %q is not configured", id)
	}
	if entry.File == "" {
		return "", fmt.Errorf("secret %q file is required", id)
	}
	path, err := r.resolveSecretPath(entry.File)
	if err != nil {
		return "", fmt.Errorf("secret %q: %w", id, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("secret %q: %w", id, err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("secret %q file must not be group- or world-readable", id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("secret %q: %w", id, err)
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

func (r *FileResolver) ResolveProfileEnv(ctx context.Context, profile, name string) (string, error) {
	p, ok := r.config.Profiles[profile]
	if !ok {
		return "", fmt.Errorf("credential profile %q is not configured", profile)
	}
	entry, ok := p.Env[name]
	if !ok {
		return "", fmt.Errorf("credential profile %q does not configure env %q", profile, name)
	}
	if entry.Secret == "" {
		return "", fmt.Errorf("credential profile %q env %q secret is required", profile, name)
	}
	return r.Resolve(ctx, entry.Secret)
}

func (r *FileResolver) ProfileEnvConfigured(profile, name string) bool {
	p, ok := r.config.Profiles[profile]
	if !ok {
		return false
	}
	_, ok = p.Env[name]
	return ok
}

func (r *FileResolver) resolveSecretPath(raw string) (string, error) {
	if filepath.IsAbs(raw) {
		return "", fmt.Errorf("file %q must be relative to config dir", raw)
	}
	clean := filepath.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file %q escapes config dir", raw)
	}
	path := filepath.Join(r.configDir, clean)
	rel, err := filepath.Rel(r.configDir, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("file %q escapes config dir", raw)
	}
	return path, nil
}
