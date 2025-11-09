package drive

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

func (h *handler) handlePost(bind, localPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		err := r.ParseMultipartForm(100_000_000)
		if err != nil {
			http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
			return
		}

		if newDir := r.FormValue("newdir"); newDir != "" {
			createNewDir(filepath.Join(localPath), path.Clean(r.URL.Path), newDir, w)
			return
		}

		finalFilePath := filepath.Join(localPath, path.Clean(r.URL.Path))
		fileID := r.Header.Get("uploader-file-id")
		chunkNum := r.Header.Get("uploader-chunk-number")
		if fileID == "" || chunkNum == "" {
			http.Error(w, "Missing uploader-file-id or uploader-chunk-number header", http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Missing file part: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		chunkFilename := fmt.Sprintf("%s.%s.%s.chunk", filepath.Base(finalFilePath), fileID, chunkNum)
		chunkPath := filepath.Join(path.Dir(finalFilePath), chunkFilename)

		out, err := createFile(chunkPath)
		if err != nil {
			http.Error(w, "Failed to create chunk file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer out.Close()

		_, err = io.Copy(out, file)
		if err != nil {
			http.Error(w, "Failed to save chunk: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Check if this is the last chunk
		totalChunksHeader := r.Header.Get("uploader-chunks-total")
		if totalChunksHeader != "" {
			var totalChunks, thisChunk int
			fmt.Sscanf(totalChunksHeader, "%d", &totalChunks)
			fmt.Sscanf(chunkNum, "%d", &thisChunk)
			if totalChunks > 0 && (thisChunk+1) == totalChunks {
				// Last chunk received, combine
				err := combineChunks(finalFilePath, fileID, totalChunks)
				if err != nil {
					http.Error(w, "Failed to combine chunks: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func createNewDir(baseFilePath, urlPath, newDir string, w http.ResponseWriter) {
	dstDir := filepath.Join(baseFilePath, path.Clean(urlPath))
	_, err := os.Lstat(dstDir)
	if err != nil && os.IsNotExist(err) {
		slog.Error("Directory does not exist, POST method called from invalid URL Path", "dstDir", dstDir)
		http.Error(w, "Inivalid directory path", http.StatusBadRequest)
		return
	} else if err != nil {
		slog.Error("Failed to check existing directory", "dstDir", dstDir, "error", err)
		http.Error(w, "Internal server error... try again later", http.StatusInternalServerError)
		return
	}
	newDir = path.Clean(newDir)
	if newDir == "." || newDir == "/" || newDir == "" {
		http.Error(w, "Invalid directory name", http.StatusBadRequest)
		return
	}

	dirPath := filepath.Join(dstDir, newDir)
	err = os.MkdirAll(dirPath, 0755)
	if err != nil {
		slog.Error("Failed to create directory", "dirPath", dirPath, "error", err)
		http.Error(w, "Unable to create directory", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func combineChunks(finalFilePath, fileID string, totalChunks int) error {
	dir := filepath.Dir(finalFilePath)
	base := filepath.Base(finalFilePath)
	out, err := createFile(finalFilePath)
	if err != nil {
		return fmt.Errorf("create final file: %w", err)
	}
	defer out.Close()

	for i := 0; i < totalChunks; i++ {
		chunkFilename := fmt.Sprintf("%s.%s.%d.chunk", base, fileID, i)
		chunkPath := filepath.Join(dir, chunkFilename)
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return fmt.Errorf("open chunk %d: %w", i, err)
		}
		_, err = io.Copy(out, chunkFile)
		chunkFile.Close()
		if err != nil {
			return fmt.Errorf("copy chunk %d: %w", i, err)
		}
		// Remove chunk after copying
		if err := os.Remove(chunkPath); err != nil {
			return fmt.Errorf("remove chunk %d: %w", i, err)
		}
	}
	return nil
}

// createFile creates a file and its parent directories if needed
func createFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	return os.Create(path)
}
