// Package handlers
package handlers

import (
	"bufio"
	"fileshare/internal/templates"
	"fileshare/internal/worker"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type UploadJob struct {
	Reader io.Reader
	TmpPath string
	DstPath string
	FileName string
	ResultChan chan error
}

func (j *UploadJob) Process() {
	defer close(j.ResultChan)

	dst, err := os.Create(j.TmpPath)
	if err != nil {
		j.ResultChan <- fmt.Errorf("failed to create tmp file: %v", err)
		return
	}

	bw := bufio.NewWriterSize(dst, transferBufferSize)

	_, copyErr := io.Copy(bw, j.Reader)

	flushErr := bw.Flush()
	closeErr := dst.Close()
	if copyErr != nil || flushErr != nil || closeErr != nil {
		os.Remove(j.TmpPath)
		j.ResultChan <- fmt.Errorf("write failed for %s", j.FileName)
		return
	}

	if err := os.Rename(j.TmpPath, j.DstPath); err != nil {
		os.Remove(j.TmpPath)
		j.ResultChan <- fmt.Errorf("rename failed: %v", err)
		return
	}
	j.ResultChan <- nil
}

func UploadHandlerFactory(wp *worker.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		reader, err := r.MultipartReader()
		if err != nil {
			log.Printf("Multipart error: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}

		var fileCount int
		var uploadErrors []error
		var results []chan error

		for {
			part, err := reader.NextPart()
			if err == io.EOF { break }
			if err != nil {
				log.Printf("Error reading part: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if part.FileName() == "" { continue }

			safeFileName := filepath.Base(part.FileName())
			tmpPath := filepath.Join(absUploadDir, "." + safeFileName + ".partial")
			dstPath := filepath.Join(absUploadDir, safeFileName)

			pr, pw := io.Pipe()

			resultChan := make(chan error, 1)
			results = append(results, resultChan)

			job := &UploadJob{
				Reader: pr,
				TmpPath: tmpPath,
				DstPath: dstPath,
				FileName: safeFileName,
				ResultChan: resultChan,
			}
			wp.Submit(job)

			buf := make([]byte, transferBufferSize)
			_, copyErr := io.CopyBuffer(pw, part, buf)

			pw.CloseWithError(copyErr)



			if copyErr != nil {
				log.Printf("Network read error for %s: %v", safeFileName, copyErr)
				http.Error(w, "Upload interrupted", http.StatusInternalServerError)
				return
			}
			fileCount++
			log.Printf("Uploaded: %s", safeFileName)
		}
		for _, rc := range results {
			if err := <-rc; err != nil {
				uploadErrors = append(uploadErrors, err)
			}
		}
		if len(uploadErrors) > 0 {
			msg := fmt.Sprintf("Uploaded %d files but %d failed", fileCount, len(uploadErrors))
			log.Println(msg)
			for _, e := range uploadErrors {
				log.Println(" - ", e)
			}
			http.Error(w, msg, http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Successfully uploaded %d files", fileCount)
	}
}
