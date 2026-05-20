package web

import (
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	"github.com/joelhelbling/kkullm/web/markdown"
)

var tmpl *template.Template

var funcMap = template.FuncMap{
	"projectColor": projectColor,
	"tagBg":        tagBg,
	"tagColor":     tagColor,
	"joinStrings":  joinStrings,
	"timeAgo":      timeAgo,
	"renderBody":   markdown.RenderBody,
	"renderTitle":  markdown.RenderTitle,
}

func initTemplates() {
	var err error
	tmpl, err = template.New("").Funcs(funcMap).ParseFS(content, "templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}
}

var projectColors = []string{
	"#0969da", "#1a7f37", "#9a6700", "#cf222e", "#8250df",
	"#bf3989", "#0550ae", "#116329", "#7d4e00", "#a40e26",
}

func projectColor(name string) string {
	h := 0
	for _, c := range name {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return projectColors[h%len(projectColors)]
}

var tagColorMap = map[string][2]string{
	"bug":         {"#ffebe9", "#cf222e"},
	"feature":     {"#dafbe1", "#1a7f37"},
	"enhancement": {"#ddf4ff", "#0969da"},
	"docs":        {"#dafbe1", "#1a7f37"},
	"rfc":         {"#fff8c5", "#9a6700"},
	"infra":       {"#dafbe1", "#1a7f37"},
	"urgent":      {"#ffebe9", "#cf222e"},
}

var defaultTagColors = [2]string{"#ddf4ff", "#0969da"}

func tagBg(tag string) string {
	if colors, ok := tagColorMap[tag]; ok {
		return colors[0]
	}
	return defaultTagColors[0]
}

func tagColor(tag string) string {
	if colors, ok := tagColorMap[tag]; ok {
		return colors[1]
	}
	return defaultTagColors[1]
}

func joinStrings(strs []string, sep string) string {
	return strings.Join(strs, sep)
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
