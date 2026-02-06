package handlers

import (
	"archive/zip"
	"bufio"
	"fileshare/internal/worker"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const transferBufferSize = 1 << 20

var compressedExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true, ".mp3": true, ".aac": true, ".flac": true, ".ogg": true,
	".zip": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true,
	".pdf": true,
}

type ZipJob struct {
	SourcePath string
	Writer     http.ResponseWriter
	Done       chan struct{}
}

func (z ZipJob) Process() {
	defer close(z.Done)

	fileName := filepath.Base(z.SourcePath) + ".zip"
	z.Writer.Header().Set("Content-Type", "application/octet-stream")
	z.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; fileName=\"%s\"", fileName))

	bw := bufio.NewWriterSize(z.Writer, transferBufferSize)
	defer bw.Flush()

	zipWriter := zip.NewWriter(bw)
	defer zipWriter.Close()

	err := filepath.Walk(z.SourcePath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(filepath.Dir(z.SourcePath), filePath)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath

		ext := strings.ToLower(filepath.Ext(filePath))
		if compressedExts[ext] {
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}

		zipFileEntry, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		fsFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		buf := make([]byte, transferBufferSize)
		_, err = io.CopyBuffer(zipFileEntry, fsFile, buf)
		return err
	})
	if err != nil {
		log.Printf("Zip error: %v", err)
	}
}

func ZipHandlerFactory(wp *worker.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relativePath := r.URL.Query().Get("path")
		if strings.Contains(relativePath, "..") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		baseDir, _ := os.Getwd()
		fullSourcePath := filepath.Join(baseDir, relativePath)

		doneChan := make(chan struct{})
		job := ZipJob{
			SourcePath: fullSourcePath,
			Writer:     w,
			Done:       doneChan,
		}
		ctx := r.Context()
		if !wp.TrySubmit(ctx, job) {
			http.Error(w, "Server busy, please try again", http.StatusServiceUnavailable)
			return
		}

		select {
		case <-doneChan:
		case <-ctx.Done():
			log.Printf("Client disconnected during zip: %s", relativePath)
		}
		<-doneChan
	}
}
