// Package cleanup
package cleanup

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StartCleanupRoutine : a goroutine that cleans up old .partial files
func StartCleanupRoutine(baseDir string, maxAge time.Duration, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		cleanPartialFiles(baseDir, maxAge)
		for range ticker.C {
			cleanPartialFiles(baseDir, maxAge)
		}
	}()
	log.Printf("Cleanup routine started: Checking every %v for files older than %v", interval, maxAge)
}

func cleanPartialFiles(baseDir string, maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)

	filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".partial") {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err == nil {
				log.Printf("Cleaned up partial: %s (age: %v)", info.Name(), time.Since(info.ModTime()))
			}
		}
		return nil
	})
}
