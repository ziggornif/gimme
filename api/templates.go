package api

import (
	"html/template"
	"time"
)

// TemplateFuncs returns the custom Go template functions used across HTML templates.
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.Format("Jan 2, 2006 15:04")
		},
		"isExpired": func(t time.Time) bool {
			if t.IsZero() {
				return false
			}
			return t.Before(time.Now())
		},
		"isRevoked": func(t time.Time) bool {
			return !t.IsZero()
		},
	}
}
