package sinks

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"runloop/internal/artifacts"
	"runloop/internal/workflows"
)

type Request struct {
	Root  string
	RunID int64
	Sink  workflows.Sink
	Data  map[string]any
}

type Output struct {
	Path string
	Body string
}

type Handler func(Request) (Output, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Handler{}
)

func Register(typ string, h Handler) {
	if typ == "" {
		panic("sinks: Register called with empty type")
	}
	if h == nil {
		panic("sinks: Register called with nil handler for type " + typ)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[typ]; dup {
		panic("sinks: duplicate registration for type " + typ)
	}
	registry[typ] = h
}

func lookupHandler(typ string) (Handler, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	h, ok := registry[typ]
	return h, ok
}

func IsRegistered(typ string) bool {
	_, ok := lookupHandler(typ)
	return ok
}

func Render(req Request) (Output, error) {
	h, ok := lookupHandler(req.Sink.Type)
	if !ok {
		return Output{}, fmt.Errorf("unsupported sink type %q", req.Sink.Type)
	}
	return h(req)
}

func resolvePath(root string, runID int64, sink workflows.Sink) (string, error) {
	if sink.Type == "json" {
		return filepath.Join(artifacts.SinkDir(root, runID), "report.json"), nil
	}
	if sink.Path == "" {
		return "", fmt.Errorf("sink path is required")
	}
	if filepath.IsAbs(sink.Path) {
		return "", fmt.Errorf("sink path %q must be relative", sink.Path)
	}
	clean := filepath.Clean(sink.Path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("sink path %q escapes sink directory", sink.Path)
	}
	sinkDir := artifacts.SinkDir(root, runID)
	path := filepath.Join(sinkDir, clean)
	rel, err := filepath.Rel(sinkDir, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("sink path %q escapes sink directory", sink.Path)
	}
	return path, nil
}

func renderJSON(req Request) (Output, error) {
	path, err := resolvePath(req.Root, req.RunID, req.Sink)
	if err != nil {
		return Output{}, err
	}
	data, err := json.MarshalIndent(req.Data, "", "  ")
	if err != nil {
		return Output{}, err
	}
	return Output{Path: path, Body: string(append(data, '\n'))}, nil
}

func renderMarkdown(req Request) (Output, error) {
	path, err := resolvePath(req.Root, req.RunID, req.Sink)
	if err != nil {
		return Output{}, err
	}
	content := "# Runloop Report\n\n"
	for key, value := range req.Data {
		content += fmt.Sprintf("- %s: %v\n", key, value)
	}
	return Output{Path: path, Body: content}, nil
}

func init() {
	Register("json", renderJSON)
	Register("markdown", renderMarkdown)
}
