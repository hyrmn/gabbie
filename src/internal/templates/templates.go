// Package templates provides Go template parsing and rendering helpers.
package templates

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
)

// Engine holds the parsed template set.
type Engine struct {
	templates *template.Template
}

// New parses all templates from the embedded filesystem.
func New(assets embed.FS) (*Engine, error) {
	funcs := template.FuncMap{
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call: odd number of arguments")
			}
			dict := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
	}

	tmpl, err := template.New("").Funcs(funcs).ParseFS(assets, "templates/*.html", "templates/components/*.html")
	if err != nil {
		return nil, err
	}

	return &Engine{templates: tmpl}, nil
}

// Render executes a named template and writes the result to the ResponseWriter.
func (e *Engine) Render(w http.ResponseWriter, status int, name string, data any) {
	t := e.templates.Lookup(name)
	if t == nil {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := t.Execute(w, data); err != nil {
		// Log the real error but don't leak internal details to the client
		slog.Default().Error("template execution failed", "name", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// MustLookup returns a template by name or panics.
func (e *Engine) MustLookup(name string) *template.Template {
	t := e.templates.Lookup(name)
	if t == nil {
		panic("template not found: " + name)
	}
	return t
}

// List returns all template names.
func (e *Engine) List() []string {
	var names []string
	for _, t := range e.templates.Templates() {
		names = append(names, t.Name())
	}
	return names
}

// TemplateDir creates a filesystem from the templates subdirectory.
func TemplateDir(assets embed.FS) (fs.FS, error) {
	return fs.Sub(assets, "templates")
}

// AssetDir returns the basename list of a subdirectory in the embedded filesystem.
func AssetDir(assets embed.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(assets, dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, filepath.Base(e.Name()))
		}
	}
	return names, nil
}
