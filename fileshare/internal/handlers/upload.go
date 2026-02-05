package handlers

import (
	"bufio"
	"fileshare/internal/templates"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)


func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		targetDir := r.URL.Query().Get("dir")
		if targetDir == "" {
			targetDir = "/"
		}
		data := struct{ ReturnLink string }{ReturnLink: targetDir}
		t, _ := template.New("upload").Parse(templates.UploadTpl)
		t.Execute(w, data)
		return
	}
	baseDir, err := os.Getwd()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	relDir := filepath.Clean(r.URL.Query().Get("dir"))
	if strings.Contains(relDir, "..") {
		http.Error(w, "Invalid Directory", http.StatusForbidden)
	}

	absUploadDir := filepath.Join(baseDir, relDir)
	if _, err = os.Stat(absUploadDir); os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	targetDir := filepath.Clean(r.URL.Query().Get("dir"))
	if strings.Contains(targetDir, "..") {
		http.Error(w, "Invalid Directory", http.StatusForbidden)
		return
	}
	log.Printf("Content-Type received: %s", r.Header.Get("Content-Type"))
	reader, err := r.MultipartReader()
	if err != nil {
		log.Printf("Multipart error: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	fileCount := 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF { break }
		if err != nil { return }

		if part.FileName() == "" { continue }

		safeFileName := filepath.Base(part.FileName())
		tmpPath := filepath.Join(absUploadDir, "." + safeFileName + ".partial")
		dstPath := filepath.Join(absUploadDir, safeFileName)

		dst, err := os.Create(tmpPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			continue
		}
		bw := bufio.NewWriterSize(dst, transferBufferSize)

		buf := make([]byte, transferBufferSize)
		_, copyErr := io.CopyBuffer(bw, part, buf)

		flushErr := bw.Flush()
		closeErr := dst.Close()


		if copyErr != nil || flushErr != nil || closeErr != nil {
			log.Printf("Upload failed for %s", safeFileName)
			os.Remove(tmpPath)
			http.Error(w, "Upload interrupted", http.StatusInternalServerError)
			return
		}

		if err := os.Rename(tmpPath, dstPath); err != nil {
			log.Printf("Rename error: %v", err)
			os.Remove(tmpPath)
			http.Error(w, "Error finalizing file", http.StatusInternalServerError)
			return
		}
		fileCount++
		log.Printf("Uploaded: %s -> %s", safeFileName, relDir)
	}
	w.WriteHeader(http.StatusOK)
	fmt.Print(w, "Successfully uploaded %d files", fileCount)
}
