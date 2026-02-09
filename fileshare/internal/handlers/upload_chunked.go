package handlers

import (
	"fileshare/internal/templates"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ChunkedUploadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			targetDir := r.URL.Query().Get("dir")
			if targetDir == ""{
				targetDir = "/"
			}
			data := struct{ ReturnLink string }{ReturnLink: targetDir}
			t, err := template.New("upload").Parse(templates.UploadTpl)
			if err != nil {
				http.Error(w, "Template error", http.StatusInternalServerError)
				return
			}
			t.Execute(w, data)

		case http.MethodPost:
		relDir := filepath.Clean(r.URL.Query().Get("dir"))

		if strings.Contains(relDir, "..") {
			http.Error(w, "Invalid directory", http.StatusForbidden)
			return
		}

		fileName := r.Header.Get("X-File-Name")
		if fileName == "" {
			http.Error(w, "Missing X-File-Name header", http.StatusBadRequest)
			return
		}

		safeFileName := filepath.Base(fileName)

		offsetStr := r.Header.Get("X-Chunk-Offset")
		offset, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid X-Chunk-Offset", http.StatusBadRequest)
			return
		}
		isFinal := r.Header.Get("X-Final-Chunk") == "true"

		baseDir, _ := os.Getwd()
		absUploadDir := filepath.Join(baseDir, relDir)

		tmpPath := filepath.Join(absUploadDir, "." + safeFileName + ".partial")
		finalPath := filepath.Join(absUploadDir, safeFileName)

		file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY, 0664)
		if err != nil {
			log.Printf("Failed to open temp file: %v", err)
			http.Error(w, "Failed to create file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		_, err = file.Seek(offset, 0)
		if err != nil {
			log.Printf("Failed to seek: %v", err)
			http.Error(w, "Failed to seek in file", http.StatusInternalServerError)
			return
		}

		written, err := io.Copy(file, r.Body)
		if err != nil {
			log.Printf("Failed to write chunk: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Chunk written: %s offset=%d size=%d", safeFileName, offset, written)

		if isFinal {
			defer file.Sync()
			defer file.Close()

			if err := os.Rename(tmpPath, finalPath); err != nil {
				log.Printf("Failed to finalize: %v", err)
				http.Error(w, "Failed to finalize", http.StatusInternalServerError)
				return
			}
			defer log.Printf("Upload Complete: %s", safeFileName)
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%d" , written)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
