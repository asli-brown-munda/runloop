package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"runloop/internal/sources"
)

const (
	Type              = "filesystem"
	defaultGlob       = "*"
	defaultEntityType = "file_item"
	maxInlineBytes    = 64 * 1024
)

func init() {
	sources.Register(Type, func(id string, cfg map[string]any) (sources.Source, error) {
		return New(id, cfg)
	})
}

type Source struct {
	id         string
	directory  string
	glob       string
	entityType string
}

func New(id string, cfg map[string]any) (*Source, error) {
	dir, _ := cfg["directory"].(string)
	if dir == "" {
		return nil, fmt.Errorf("filesystem source %q requires config.directory", id)
	}
	dir = expandHome(dir)
	glob, _ := cfg["glob"].(string)
	if glob == "" {
		glob = defaultGlob
	}
	entityType, _ := cfg["entityType"].(string)
	if entityType == "" {
		entityType = defaultEntityType
	}
	if _, err := filepath.Match(glob, "probe"); err != nil {
		return nil, fmt.Errorf("filesystem source %q invalid glob %q: %w", id, glob, err)
	}
	return &Source{id: id, directory: dir, glob: glob, entityType: entityType}, nil
}

func (s *Source) ID() string   { return s.id }
func (s *Source) Type() string { return Type }

func (s *Source) Test(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(s.directory)
	if err != nil {
		return fmt.Errorf("filesystem source %q: %w", s.id, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("filesystem source %q: %s is not a directory", s.id, s.directory)
	}
	return nil
}

func (s *Source) Sync(ctx context.Context, cursor sources.Cursor) ([]sources.InboxCandidate, sources.Cursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, cursor, err
	}
	since := time.Time{}
	if !cursor.IsZero() {
		t, err := time.Parse(time.RFC3339Nano, cursor.Value)
		if err != nil {
			return nil, cursor, fmt.Errorf("filesystem source %q: invalid cursor %q: %w", s.id, cursor.Value, err)
		}
		since = t
	}
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return nil, cursor, fmt.Errorf("filesystem source %q: %w", s.id, err)
	}
	matches := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ok, err := filepath.Match(s.glob, entry.Name())
		if err != nil {
			return nil, cursor, err
		}
		if ok {
			matches = append(matches, entry)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Name() < matches[j].Name() })

	candidates := make([]sources.InboxCandidate, 0, len(matches))
	maxMtime := since
	for _, entry := range matches {
		if err := ctx.Err(); err != nil {
			return nil, cursor, err
		}
		path := filepath.Join(s.directory, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, cursor, err
		}
		mtime := info.ModTime().UTC()
		if !mtime.After(since) {
			continue
		}
		if mtime.After(maxMtime) {
			maxMtime = mtime
		}
		payload := map[string]any{
			"path":       path,
			"name":       entry.Name(),
			"size":       info.Size(),
			"modifiedAt": mtime.Format(time.RFC3339Nano),
		}
		if info.Size() <= maxInlineBytes {
			data, err := os.ReadFile(path)
			if err == nil && utf8.Valid(data) {
				payload["content"] = string(data)
			}
		}
		candidates = append(candidates, sources.InboxCandidate{
			SourceID:   s.id,
			ExternalID: entry.Name(),
			EntityType: s.entityType,
			Title:      entry.Name(),
			RawPayload: payload,
			Normalized: payload,
			ObservedAt: mtime,
		})
	}
	if len(candidates) == 0 {
		return nil, cursor, nil
	}
	return candidates, sources.Cursor{Value: maxMtime.Format(time.RFC3339Nano)}, nil
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
