package sinks

import (
	"bytes"
	"text/template"
)

func init() { Register("file", renderFile) }

func renderFile(req Request) (Output, error) {
	path, err := resolvePath(req.Root, req.RunID, req.Sink)
	if err != nil {
		return Output{}, err
	}
	tmpl, err := template.New("file sink").Option("missingkey=error").Parse(req.Sink.Body)
	if err != nil {
		return Output{}, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, req.Data); err != nil {
		return Output{}, err
	}
	return Output{Path: path, Body: buf.String()}, nil
}
