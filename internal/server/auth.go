package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/andrebq/davd/internal/config"
)

func Protect(db *config.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pwd, found := r.BasicAuth()
		if !found {
			slog.Debug("Access without proper credentials", "path", r.URL.Path)
			w.Header().Add("WWW-Authenticate", "Basic realm=\"DAVD Server\"")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, userdata, err := db.PasswordLogin(user, pwd)
		if err != nil {
			slog.Error("Error while checking user authentication", "err", err)
			w.Header().Add("WWW-Authenticate", "Basic realm=\"DAVD Server\"")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		r = r.WithContext(config.WithUser(r.Context(), userdata))
		authorize(w, r, next)
	})
}

func authorize(w http.ResponseWriter, r *http.Request, next http.Handler) {
	user := config.UserFromContext(r.Context())
	if !hasPermissions(user.Permissions, r.URL, r.Method) {
		slog.Error("User attempted to access a resources but lacks permission", "url", r.URL, "method", r.Method)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	next.ServeHTTP(w, r)
}

func hasPermissions(perm []config.Permission, url *url.URL, method string) bool {
	var assigned config.Permission
	for _, v := range perm {
		prefix := v.Prefix
		if !strings.HasSuffix(prefix, "/") {
			prefix = fmt.Sprintf("%v/", v)
		}
		if strings.HasPrefix(url.Path, prefix) && (v.Writer || v.Reader || v.Execute) {
			assigned = v
			break
		}
	}
	if assigned.Prefix == "" {
		return false
	}

	switch method {
	case "GET", "PROPFIND":
		// writers can also read data, therefore we dont need to check if CanWrite is true
		return assigned.Reader

	}
	return assigned.Writer
}
