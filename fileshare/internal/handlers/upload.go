// Package handlers
package handlers

import (
	"context"
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
	"sync"
)

const ChunkSize = 4 << 20

var bufferPool = sync.Pool{
	New: func() interface {} {
		b := make([]byte, ChunkSize)
		return &b
	},
}

type ChunkWriteJob struct {
	File *os.File
	Data []byte
	DataLen int
	Offset int64
	Wg *sync.WaitGroup
	ErrorChan chan error
	Ctx context.Context
}

func (j *ChunkWriteJob) Process() {
	defer j.Wg.Done()
	if j.Ctx != nil {
		select {
				case <-j.Ctx.Done():
						ptr := &j.Data
						bufferPool.Put(ptr)
						return
				default:
		}
	}
	_, err := j.File.WriteAt(j.Data[:j.DataLen], j.Offset)
	if err != nil {
		select {
		case j.ErrorChan <- err:
		default:
		}
	}
	ptr := &j.Data
	bufferPool.Put(ptr)

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

		reader, err := r.MultipartReader()
		if err != nil {
			log.Printf("Multipart error: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}

		fileCount := 0
		errChan := make(chan error, 200)
		ctx := r.Context()

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
			dstPath := filepath.Join(absUploadDir, safeFileName)

			tmpFileName := fmt.Sprintf(".%s.part", safeFileName)
			tmpPath := filepath.Join(absUploadDir, tmpFileName)

			tmpFile, err := os.Create(tmpPath)
			if err != nil {
				log.Printf("Create error: %v", err)
				continue
			}
			uploaded := false
			func() {
				defer func() {
					tmpFile.Close()
					if !uploaded {
						os.Remove(tmpPath)
						log.Printf("Cleaned up temp file: %s", tmpFileName)
					}
				}()

				var fileWg sync.WaitGroup
				var currentOffset int64 = 0
				uploadErr := false

				log.Printf("Pipleline started: %s", safeFileName)

				for {
					select {
					case <- ctx.Done():
						log.Printf("Upload cancelled by client: %s", safeFileName)
						uploadErr = true
						return
					default:
					}

					bufPtr := bufferPool.Get().(*[]byte)
					buf := *bufPtr

					n, readErr := io.ReadFull(part, buf)

					if n > 0 {
						fileWg.Add(1)

						job := &ChunkWriteJob{
							File: tmpFile,
							Data: buf,
							DataLen: n,
							Offset: currentOffset,
							Wg: &fileWg,
							ErrorChan: errChan,
							Ctx: ctx,
						}
						if !wp.TrySubmit(ctx, job) {
							bufferPool.Put(bufPtr)
							fileWg.Done()
							log.Printf("Upload queue full for: %s", safeFileName)
							uploadErr = true
							return
						}

						currentOffset += int64(n)
					} else {
						bufferPool.Put(bufPtr)
					}

					select {
					case e := <- errChan:
						log.Printf("Write error: %v", e)
						uploadErr = true
						fileWg.Wait()
						return
					default:
					}
					if readErr != nil {
						if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
							break
						}
						log.Printf("Network error: %v", err)
						uploadErr = true
						fileWg.Wait()
						return
					}
				}

				fileWg.Wait()

				select {
				case <- ctx.Done():
					uploadErr = true
				case e := <-errChan:
					log.Printf("Final write error: %v", e)
					uploadErr = true
				default:
				}
				if uploadErr || ctx.Err() != nil {
					return
				}
				tmpFile.Close()

				if err := os.Rename(tmpPath, dstPath); err != nil {
					log.Printf("Finalize error: %v", err)
					return
				}

				uploaded = true
				fileCount++
				log.Printf("Uploaded: %s (Size: %.2f MB)", safeFileName, float64(currentOffset)/1024/1024)
		}()
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Successfully uploaded %d files", fileCount)
	}
}
