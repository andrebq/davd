package drive

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type (
	Bindings map[string]string

	handler struct {
		muxer    *http.ServeMux
		bindings Bindings
	}

	dirData struct {
		Path      string
		Localpath string
		Basename  string
		Files     []string
		Dirs      []string
	}
)

//go:embed assets/js/* assets/css/*
var assetsStatic embed.FS

func AssetsHandler() http.Handler {
	sub, err := fs.Sub(assetsStatic, "assets")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

func NewHandler(bindings Bindings) (http.Handler, error) {
	muxer := http.NewServeMux()
	h := handler{
		muxer:    muxer,
		bindings: bindings,
	}
	for bind, localPath := range bindings {
		fn, err := h.serveBind(bind)
		if err != nil {
			return nil, fmt.Errorf("unable to setup handler for bind %v (%v): %w", bind, bindings[bind], err)
		}
		muxer.Handle(fmt.Sprintf("POST /%v/", bind), http.StripPrefix(fmt.Sprintf("/%v", bind), h.handlePost(bind, localPath)))
		muxer.HandleFunc(fmt.Sprintf("GET /%v/", bind), fn)
	}

	return muxer, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.muxer.ServeHTTP(w, r)
}

func (h *handler) serveBind(bind string) (func(w http.ResponseWriter, r *http.Request), error) {
	localPath := h.bindings[bind]
	var err error
	localPath, err = filepath.Abs(localPath)
	if err != nil {
		return nil, fmt.Errorf("unable to get absolute path for bind %v (%v): %w", bind, localPath, err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleFileRequest(path.Clean(fmt.Sprintf("/%v/", bind)), localPath, w, r)
	}, nil
}

func (h *handler) handleFileRequest(bindPrefix string, localPath string, w http.ResponseWriter, r *http.Request) {
	localAbs := filepath.Join(localPath, filepath.FromSlash(strings.TrimPrefix(path.Clean(r.URL.Path), bindPrefix)))
	stat, err := os.Lstat(localAbs)
	if err != nil {
		slog.Debug("File not found", "localAbs", localAbs)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	if stat.IsDir() {
		h.renderDir(stat, localAbs, w, r)
		return
	}
	h.renderFile(stat, localAbs, w, r)
}

func (h *handler) renderDir(stat os.FileInfo, localAbs string, w http.ResponseWriter, r *http.Request) {
	dd := dirData{
		Path:      r.URL.Path,
		Basename:  filepath.Base(localAbs),
		Localpath: localAbs,
		Files:     []string{},
		Dirs:      []string{},
	}
	err := filepath.WalkDir(localAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == localAbs {
			return nil
		}
		if d.IsDir() {
			dd.Dirs = append(dd.Dirs, filepath.Base(d.Name()))
			return filepath.SkipDir
		} else {
			dd.Files = append(dd.Files, filepath.Base(d.Name()))
		}
		return nil
	})
	if err != nil {
		slog.Error("Failed to read directory", "localAbs", localAbs, "error", err)
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}
	buf := &strings.Builder{}
	err = templates.ExecuteTemplate(buf, "page/directory", dd)
	if err != nil {
		slog.Error("Failed to render template", "localAbs", localAbs, "error", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	_, err = w.Write([]byte(buf.String()))
	if err != nil {
		slog.Error("Failed to write response", "localAbs", localAbs, "error", err)
		return
	}
}

func (h *handler) renderFile(stat os.FileInfo, localAbs string, w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("download") == "true" {
		var mtime time.Time
		st, err := os.Lstat(localAbs)
		if err == nil {
			mtime = st.ModTime()
		}
		fd, err := os.Open(localAbs)
		if err != nil {
			slog.Error("Failed to open file for download", "localAbs", localAbs, "error", err)
			http.Error(w, "Failed to open file for download", http.StatusInternalServerError)
			return
		}
		defer fd.Close()
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(localAbs)))
		http.ServeContent(w, r, filepath.Base(localAbs), mtime, fd)
		return
	}
	var err error
	buf := &strings.Builder{}
	err = templates.ExecuteTemplate(buf, "page/file", stat)
	if err != nil {
		slog.Error("Failed to render template", "localAbs", localAbs, "error", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	_, err = w.Write([]byte(buf.String()))
	if err != nil {
		slog.Error("Failed to write response", "localAbs", localAbs, "error", err)
		return
	}
}
