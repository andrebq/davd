package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/andrebq/davd/internal/auth"
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
		permissions, err := auth.CheckCredential(r.Context(), db, user, pwd)
		if auth.IsInvalidCredential(err) {
			slog.Warn("Invalid access attempt", "user", user)
			w.Header().Add("WWW-Authenticate", "Basic realm=\"DAVD Server\"")
			w.WriteHeader(http.StatusUnauthorized)
			return
		} else if err != nil {
			slog.Error("Error while checking user authentication", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r = r.WithContext(auth.WithPermissions(r.Context(), *permissions))
		authorize(w, r, next)
	})
}

func authorize(w http.ResponseWriter, r *http.Request, next http.Handler) {
	perm := auth.GetPermissions(r.Context())
	if !hasPermissions(perm, r.URL, r.Method) {
		slog.Error("User attempted to access a resources but lacks permission", "url", r.URL, "method", r.Method)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	next.ServeHTTP(w, r)
}

func hasPermissions(perm auth.Permissions, url *url.URL, method string) bool {
	canAccess := false
	for _, v := range perm.Allowed {
		if strings.HasSuffix(v, "/") {
			v = fmt.Sprintf("%v/", v)
		}
		if strings.HasPrefix(url.Path, v) {
			canAccess = true
			break
		}
	}
	if !canAccess {
		return false
	}

	switch method {
	case "GET", "PROPFIND":
		// writers can also read data, therefore we dont need to check if CanWrite is true
		return true
	}
	return perm.CanWrite
}
