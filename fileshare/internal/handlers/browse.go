package handlers

import (
	"fileshare/internal/templates"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type FileItem struct {
	Name string
	Path string
	IsDir bool
	Size string
	DownloadURL string
}

type BreadCrumb struct {
	Name string
	Link string
}

func FileServerHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)

		cleanPath := filepath.Clean(r.URL.Path)
		fullPath := filepath.Join(baseDir, cleanPath)

		info, err := os.Stat(fullPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if !info.IsDir() {
			http.ServeFile(w, r, fullPath)
		}

		entries, err := os.ReadDir(fullPath)
		if err != nil {
			http.Error(w, "Could not read directory", http.StatusInternalServerError)
			return
		}

		var breadcrumbs []BreadCrumb
		breadcrumbs = append(breadcrumbs, BreadCrumb{
			Name: "Home",
			Link: "/",
		})

		trimmedPath := strings.Trim(r.URL.Path, "/")
		if trimmedPath != "" {
			parts := strings.Split(trimmedPath, "/")
			accumulatedPath := ""
			for _, part := range parts {
				encodedPart := url.PathEscape(part)
				accumulatedPath = accumulatedPath + "/" + encodedPart
				breadcrumbs = append(breadcrumbs, BreadCrumb{
					Name: part,
					Link: accumulatedPath,
				})
			}
		}

		var items []FileItem
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") || strings.HasSuffix(entry.Name(), ".partial"){
				continue
			}

			size := ""
			info, err := entry.Info()
			if err == nil && !entry.IsDir() {
				size = fmt.Sprintf("%.2f KB", float64(info.Size())/1024)
			}

			currentURLPath := filepath.Join(r.URL.Path, entry.Name())
			currentURLPath = filepath.ToSlash(currentURLPath)

			if !strings.HasPrefix(currentURLPath, "/") {
				currentURLPath = "/" + currentURLPath
			}

			downloadURL := currentURLPath
			if entry.IsDir() {
				downloadURL = fmt.Sprintf("/zip?path=%s", currentURLPath)
			}

			items = append(items, FileItem{
				Name: entry.Name(),
				Path: currentURLPath,
				IsDir: entry.IsDir(),
				Size: size,
				DownloadURL: downloadURL,
			})
		}

		data := struct {
			BreadCrumbs []BreadCrumb
			Files []FileItem
			CurrentPath string
		}{
			BreadCrumbs: breadcrumbs,
			Files: items,
			CurrentPath: r.URL.Path,
		}

		t, err := template.New("webpage").Parse(templates.BrowseTpl)
		if err != nil {
			log.Printf("[ERROR] Template Parse error: %v", err)
			http.Error(w, "Template error", 500)
			return
		}
		 if err := t.Execute(w, data); err != nil {
			 log.Printf("[ERROR] Template execution error: %v", err)
		 }

	}
}
