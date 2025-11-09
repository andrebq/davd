package drive

import (
	"embed"
	"fmt"
	"html/template"
	"time"
)

//go:embed templates/*.html
var templateAssets embed.FS

var (
	templates = template.Must(loadTemplates())
)

func loadTemplates() (*template.Template, error) {
	funcs := template.FuncMap{
		"time_ago": func(t interface{}) string {
			switch v := t.(type) {
			case int64:
				return humanizeTimeAgo(time.Duration(v))
			case time.Time:
				return humanizeTimeAgo(time.Since(v))
			default:
				return ""
			}
		},
	}
	return template.New("root").Funcs(funcs).ParseFS(templateAssets, "templates/*.html")
}

func humanizeTimeAgo(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		} else if days < 7 {
			return fmt.Sprintf("%d days ago", days)
		} else if days < 30 {
			weeks := days / 7
			if weeks == 1 {
				return "1 week ago"
			}
			return fmt.Sprintf("%d weeks ago", weeks)
		} else if days < 365 {
			months := days / 30
			if months == 1 {
				return "1 month ago"
			}
			return fmt.Sprintf("%d months ago", months)
		}
		years := days / 365
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
