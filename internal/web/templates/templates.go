package templates

import (
    "embed"
    "html/template"
    "strings"

    domain "best_trade_logs/internal/domain/trade"
)

//go:embed *.gohtml
var templateFS embed.FS

// New parses the embedded templates with helper functions configured.
func New() (*template.Template, error) {
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

    return template.New("base").Funcs(funcMap).ParseFS(templateFS, "*.gohtml")
}
