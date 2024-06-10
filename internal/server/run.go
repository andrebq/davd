package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/andrebq/davd/internal/config"

	"golang.org/x/net/webdav"
)

type (
	Environ struct {
		Entries func() []string
		Expand  func(string) string
	}
)

func Run(ctx context.Context, db *config.DB, env Environ) error {
	cfg, err := LoadConfig(ctx, db)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(cfg.RootDir)
	if err != nil {
		return err
	}
	dir := webdav.Dir(abs)

	scratch, err := os.MkdirTemp("", "davd-scratch")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)

	handlers := map[string]webdav.FileSystem{
		"default": dir,
	}

	bindings, err := UpdateDynamicBinds(ctx, db, env.Entries, env.Expand)
	if err != nil {
		return err
	}

	for name, fp := range bindings.Entries {
		handlers[name] = webdav.Dir(fp)
	}

	mux := http.NewServeMux()
	for k, v := range handlers {
		urlpath := fmt.Sprintf("%v/", filepath.Join("/", "binds", k))
		h := webdav.Handler{
			Prefix:     urlpath,
			FileSystem: v,
			// TODO: here we are basically allowing users to use up all memory by creating a bunch of useless locks
			// fix this in the future
			LockSystem: webdav.NewMemLS(),
			Logger: func(r *http.Request, err error) {
				if err != nil {
					slog.Error("Failed request", "path", r.URL.Path, "error", err)
				}
				slog.Info("Request", "method", r.Method, "path", r.URL.Path)
			},
		}
		mux.Handle(urlpath, &h)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "OK")
	})

	srv := http.Server{
		Addr:           net.JoinHostPort(cfg.Address, strconv.FormatUint(uint64(cfg.Port), 10)),
		MaxHeaderBytes: 1000,
		Handler:        mux,
		BaseContext:    func(l net.Listener) context.Context { return ctx },
	}

	errch := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		defer close(errch)
		slog.Info("Starting HTTP server", "addr", srv.Addr)
		err = srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		errch <- err
	}()
	<-ctx.Done()
	shutdownServer(&srv)
	return <-errch
}

func shutdownServer(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	srv.Shutdown(ctx)
}
