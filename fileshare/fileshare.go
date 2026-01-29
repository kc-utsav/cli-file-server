package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/grandcat/zeroconf"
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

const tpl = `
<!DOCTYPE html>
<html>
<head>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>My File Server</title>
	<style>
		body { font-family: -apple-system, system-ui, sans-serif; background: #f4f4f4; padding: 20px; }
		h1 { text-align: center; color: #333; }
		.grid {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
			gap: 15px;
		}
		.card {
			background: white;
			border-radius: 12px;
			overflow: hidden;
			text-align: center;
			box-shadow: 0 2px 5px rgba(0,0,0,0.1);
			transition: transform 0.2s;
			display: flex;
			flex-direction: column;
			text-decoration: none;
			color: #333;
		}
		.card-content {
			padding: 15px;
			text-align: center;
			flex-grow: 1;
			text-decoration: none;
			color: #333;
		}
		.card:hover { transform: translateY(-5px); box-shadow: 0 5px 15px rgba(0,0,0,0.2); }
		.icon { font-size: 50px; margin-bottom: 10px; }
		.name { font-weight: bold; word-break: break-word; }
		.size { font-size: 12px; color: #888; margin-top: 5px; }
		.actions {
			border-top: 1px solid #eee;
			background: #fafafa;
			text-align: center;
		}
		.download-btn {
			display: block;
			padding: 10px;
			color: #007bff;
			font-weight: bold;
			text-decoration: none;
			font-size: 14px;
		}
		.download-btn:hover { background: #eef}
		.upload-btn { display: block; max-width: 300px; margin: 20px auto; padding: 15px; background: #007bff; color: white; text-align: center; border-radius: 8px; text-decoration: none; font-weight: bold;}
	</style>
</head>
<body>
	<h1>📂 My Shared Files</h1>
	<div style="background:white; padding: 10px; margin-bottom: 20px; border-radius: 8px;">
		{{range .BreadCrumbs}}
			<a href="{{.Link}}" style="text-decoration: none; color: #007bff; font-weight: bold;">{{.Name}}</a>
			<span style="color: #999;"> / </span>
		{{end}}
	</div>
	<a href="/upload" class="upload-btn">Upload New File</a>

	<div class="grid">
		{{range .Files}}
		<div class="card">
			<a href="{{.Path}}" class="card">
				<div class="icon">
					{{if .IsDir}} 📁 {{else}} 📄 {{end}}
				</div>
				<div class="name">{{.Name}}</div>
				{{if not .IsDir}} <div class="size">{{.Size}}</div> {{end}}
			</a>
			{{if.DownloadURL}}
			<div class="actions">
				<a href="{{.DownloadURL}}" class="download-btn" download>Save</a>
				</div>
				{{end}}
		</div>
		{{end}}
	</div>
</body>
</html>
`

func getLocalIP() (string, *net.Interface) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", nil
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && ip.To4() != nil {
				return ip.String(), &iface
			}
		}
	}
	return "localhost", nil
}

func loggingMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)

		next.ServeHTTP(w, r)
	})
}

func uploadHandler( w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		html := `
		<!DOCTYPE html>
		<html>
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<head><title>Upload File</title></head>
		<body>
			<h1>Upload a file</h1>
			<form action="/upload" method="post" enctype="multipart/form-data">
				<input type="file" name="myFile">
				<input type="submit" value="Upload">
			</form>
			<br>
			<a href="/">Back to files</a>
		</body>
		</html>
		`
		fmt.Fprint(w, html)
		return
	}

	file, header, err := r.FormFile("myFile")
	if err != nil {
		http.Error(w, "Error parsing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	dst, err := os.Create(filepath.Base(header.Filename))
	if err != nil {
		http.Error(w, "Error creating file on server", http.StatusInternalServerError)
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Upload successful! File: %s", header.Filename)
}

func customFileHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, r.URL.Path)

		info, err := os.Stat(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				http.Error(w, "Could not read directory", 500)
				return
			}

			var breadcrumbs []BreadCrumb
			breadcrumbs = append(breadcrumbs, BreadCrumb{
				Name: "Home",
				Link: "/",
			})

			cleanPath := strings.Trim(r.URL.Path, "/")
			if cleanPath != "" {
				parts := strings.Split(cleanPath, "/")
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
				size := ""
				info, _ := entry.Info()


				if !entry.IsDir() {
					size = fmt.Sprintf("%.2f KB", float64(info.Size())/1024)
				}

				currentUrlPath := filepath.Join(r.URL.Path, entry.Name())
				downloadUrl := ""

				if entry.IsDir() {
					downloadUrl = fmt.Sprintf("/zip?path=%s", currentUrlPath)
				} else {
					downloadUrl = currentUrlPath
				}

				items = append(items, FileItem{
					Name: entry.Name(),
					Path: currentUrlPath,
					IsDir: entry.IsDir(),
					Size: size,
					DownloadURL: downloadUrl,
				})
			}

			data := struct {
				BreadCrumbs []BreadCrumb
				Files []FileItem
			}{
				BreadCrumbs: breadcrumbs,
				Files: items,
			}

			t, err := template.New("webpage").Parse(tpl)
			if err != nil {
				http.Error(w, "Template error", 500)
				return
			}
			t.Execute(w, data)
			return
		}
		http.FileServer(http.Dir(dir)).ServeHTTP(w,r)
	}
}

func zipHandler(w http.ResponseWriter, r *http.Request) {
	relativePath := r.URL.Query().Get("path")

	baseDir, _ := os.Getwd()
	fullSourcePath := filepath.Join(baseDir, relativePath)

	fileName := filepath.Base(fullSourcePath) + ".zip"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	err := filepath.Walk(fullSourcePath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filePath == fullSourcePath {
			return nil
		}

		relPath, err := filepath.Rel(filepath.Dir(fullSourcePath), filePath)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		zipFileEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		fsFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		_, err = io.Copy(zipFileEntry, fsFile)
		return err

	})
	if err != nil {
		log.Printf("Failed to zip: %v", err)
	}
}

func startMDNS(port int, ip string, iface *net.Interface) {

	if iface == nil {
		log.Println("No suitable network for mDNS")
		return
	}

	hostName := "fileshare"
	interfaces := []net.Interface{*iface}
	server, err := zeroconf.RegisterProxy(
		"FileShare",
		"_http._tcp",
		"local.",
		port,
		hostName,
		[]string{ip},
		[]string{"txtv=0", "lo=1", "la=2"},
		interfaces,
	)
	if err != nil {
		log.Printf("Could not start mDNS: %v", err)
		return
	}
	log.Printf("mDNS active: http://%s:%d (Bound to %s)", hostName, port, iface.Name)

	go func() {
		<-make(chan struct{})
	_ = server
	}()
}

func main(){

	portPtr := flag.String("port", "8080", "The port to run the server on")
	flag.Parse()

	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = "."
	}

	uiHandler := customFileHandler(currentDir)
	wrappedHandler := loggingMiddleware(uiHandler)

	http.Handle("/", wrappedHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/zip", zipHandler)

	ip, iface := getLocalIP()

	portInt, _ := strconv.Atoi(*portPtr)
	startMDNS(portInt, ip, iface)

	fmt.Printf("Sharing: %s\n", currentDir)
	fmt.Printf("On Wifi: http://%s:%s\n", ip, *portPtr)
	fmt.Printf("On domain: http://fileshare.local:%s\n", *portPtr)

	log.Fatal(http.ListenAndServe("0.0.0.0:"+*portPtr , nil))
}
