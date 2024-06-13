package server

import (
	"log/slog"
	"net/http"

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
		err := auth.CheckCredential(r.Context(), db, user, pwd)
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

		next.ServeHTTP(w, r)
	})

}
