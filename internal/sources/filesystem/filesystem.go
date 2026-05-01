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

	"github.com/fsnotify/fsnotify"

	"runloop/internal/sources"
)

const (
	Type              = "filesystem"
	defaultGlob       = "*"
	defaultEntityType = "file_item"
	maxInlineBytes    = 64 * 1024
)

func init() {
	sources.Register(Type, func(id string, cfg map[string]any, opts sources.BuildOptions) (sources.Source, error) {
		return New(id, cfg)
	})
}

type Source struct {
	id         string
	directory  string
	glob       string
	entityType string
	newWatcher watcherFactory
}

type watcherFactory func() (fileWatcher, error)

type fileWatcher interface {
	Add(name string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

type fsnotifyWatcher struct {
	watcher *fsnotify.Watcher
}

func newFSNotifyWatcher() (fileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &fsnotifyWatcher{watcher: watcher}, nil
}

func (w *fsnotifyWatcher) Add(name string) error         { return w.watcher.Add(name) }
func (w *fsnotifyWatcher) Close() error                  { return w.watcher.Close() }
func (w *fsnotifyWatcher) Events() <-chan fsnotify.Event { return w.watcher.Events }
func (w *fsnotifyWatcher) Errors() <-chan error          { return w.watcher.Errors }

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
	return &Source{id: id, directory: dir, glob: glob, entityType: entityType, newWatcher: newFSNotifyWatcher}, nil
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

func (s *Source) WaitForChange(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.Test(ctx); err != nil {
		return err
	}
	newWatcher := s.newWatcher
	if newWatcher == nil {
		newWatcher = newFSNotifyWatcher
	}
	watcher, err := newWatcher()
	if err != nil {
		return fmt.Errorf("filesystem source %q: create watcher: %w", s.id, err)
	}
	defer func() { _ = watcher.Close() }()
	if err := watcher.Add(s.directory); err != nil {
		return fmt.Errorf("filesystem source %q: watch %s: %w", s.id, s.directory, err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events():
			if !ok {
				return fmt.Errorf("filesystem source %q: watcher closed", s.id)
			}
			if s.matchesEvent(event) {
				return nil
			}
		case err, ok := <-watcher.Errors():
			if !ok {
				return fmt.Errorf("filesystem source %q: watcher closed", s.id)
			}
			return fmt.Errorf("filesystem source %q: watch error: %w", s.id, err)
		}
	}
}

func (s *Source) matchesEvent(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
		return false
	}
	if filepath.Dir(event.Name) != s.directory {
		return false
	}
	ok, err := filepath.Match(s.glob, filepath.Base(event.Name))
	return err == nil && ok
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
