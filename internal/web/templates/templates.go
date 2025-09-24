package templates

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"strings"
	domain "best_trade_logs/internal/domain/trade"
)

//go:embed *.gohtml
var templateFS embed.FS

// Engine encapsulates parsed templates keyed by page name.
type Engine struct {
	templates map[string]*template.Template
}

// New parses the embedded templates with helper functions configured.
func New() (*Engine, error) {
	funcMap := template.FuncMap{
		"ptrValue": func(v *float64) float64 {
			if v == nil {
				return 0
			}
			return *v
		},
		"join": func(values []string, sep string) string {
			return strings.Join(values, sep)
		},
		"followUpChange": func(tr *domain.Trade, fu domain.FollowUp) float64 {
			if tr == nil {
				return 0
			}
			if pct, ok := tr.FollowUpChangePercent(fu.DaysAfter); ok {
				return pct
			}
			return 0
		},
	}

	base, err := template.New("layout.gohtml").Funcs(funcMap).ParseFS(templateFS, "layout.gohtml")
	if err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(templateFS, ".")
	if err != nil {
		return nil, err
	}

	tmpls := make(map[string]*template.Template)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "layout.gohtml" {
			continue
		}

		clone, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := clone.ParseFS(templateFS, name); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		tmpls[name] = clone
	}

	return &Engine{templates: tmpls}, nil
}

// ExecuteTemplate renders the named template into the writer.
func (e *Engine) ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	tmpl, ok := e.templates[name]
	if !ok {
		return fmt.Errorf("template %s not found", name)
	}
	return tmpl.ExecuteTemplate(w, name, data)
}
