// Package handlers
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
			// Serve the upload page
			targetDir := r.URL.Query().Get("dir")
			if targetDir == "" {
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

			// Clean the path but preserve subdirectories
			cleanName := filepath.Clean(fileName)

			// reject path traversal
			if strings.Contains(cleanName, "..") {
				http.Error(w, "Invalid filename", http.StatusForbidden)
				return
			}

			// Parse chunk metadata
			offsetStr := r.Header.Get("X-Chunk-Offset")
			offset, err := strconv.ParseInt(offsetStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid X-Chunk-Offset", http.StatusBadRequest)
				return
			}

			isFinal := r.Header.Get("X-Final-Chunk") == "true"

			baseDir, _ := os.Getwd()
			absUploadDir := filepath.Join(baseDir, relDir)

			// For folder uploads, create subdirectories
			fullFilePath := filepath.Join(absUploadDir, cleanName)
			fileDir := filepath.Dir(fullFilePath)

			// Create subdirectories if needed
			if err := os.MkdirAll(fileDir, 0755); err != nil {
				log.Printf("Failed to create directory: %v", err)
				http.Error(w, "Failed to create directory", http.StatusInternalServerError)
				return
			}

			// Temp file uses base name only for the .partial
			baseName := filepath.Base(cleanName)
			tmpPath := filepath.Join(fileDir, "."+baseName+".partial")
			finalPath := fullFilePath

			if isFinal {
				if err := os.Rename(tmpPath, finalPath); err != nil {
					log.Printf("Failed to finalize: %v", err)
					http.Error(w, "Failed to finalize", http.StatusInternalServerError)
					return
				}
				log.Printf("Upload complete: %s", cleanName)
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "0")
				return
			}

			// Write chunk data
			file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Printf("Failed to open temp file: %v", err)
				http.Error(w, "Failed to create file", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			// Seek to correct offset fro parallel write operation
			_, err = file.Seek(offset, 0)
			if err != nil {
				log.Printf("Failed to seek: %v", err)
				http.Error(w, "Failed to seek in file", http.StatusInternalServerError)
				return
			}

			// Stream chunk body to file without memory buffering
			written, err := io.Copy(file, r.Body)
			if err != nil {
				log.Printf("Failed to write chunk: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Sync to ensure data hits disk before responding OK
			file.Sync()

			log.Printf("Chunk written: %s offset=%d size=%d", cleanName, offset, written)

			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%d", written)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
