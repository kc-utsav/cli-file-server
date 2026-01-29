package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

type FileItem struct {
	Name string
	Path string
	IsDir bool
	Size string
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
			padding: 15px;
			border-radius: 12px;
			text-align: center;
			box-shadow: 0 2px 5px rgba(0,0,0,0.1);
			transition: transform 0.2s;
			text-decoration: none;
			color: #333;
			display: block;
		}
		.card:hover { transform: translateY(-5px); box-shadow: 0 5px 15px rgba(0,0,0,0.2); }
		.icon { font-size: 50px; margin-bottom: 10px; }
		.name { font-weight: bold; word-break: break-word; }
		.size { font-size: 12px; color: #888; margin-top: 5px; }
		.upload-btn { display: block; max-width: 300px; margin: 20px auto; padding: 15px; background: #007bff; color: white; text-align: center; border-radius: 8px; text-decoration: none; font-weight: bold;}
	</style>
</head>
<body>
	<h1>📂 My Shared Files</h1>
	<a href="/upload" class="upload-btn">Upload New File</a>

	<div class="grid">
		{{range .}}
		<a href="{{.Path}}" class="card">
			<div class="icon">
				{{if .IsDir}} 📁 {{else}} 📄 {{end}}
			</div>
			<div class="name">{{.Name}}</div>
			{{if not .IsDir}} <div class="size">{{.Size}}</div> {{end}}
		</a>
		{{end}}
	</div>
</body>
</html>
`

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "localhost"
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

			var items []FileItem
			for _, entry := range entries {
				size := ""
				info, _ := entry.Info()

				if !entry.IsDir() {
					size = fmt.Sprintf("%.2f KB", float64(info.Size())/1024)
				}

				items = append(items, FileItem{
					Name: entry.Name(),
					Path: filepath.Join(r.URL.Path, entry.Name()),
					IsDir: entry.IsDir(),
					Size: size,
				})
			}
			t, err := template.New("webpage").Parse(tpl)
			if err != nil {
				http.Error(w, "Template error", 500)
				return
			}
			t.Execute(w, items)
			return
		}
		http.FileServer(http.Dir(dir)).ServeHTTP(w,r)
	}
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

	ip := getLocalIP()

	fmt.Printf("Sharing: %s\n", currentDir)
	fmt.Printf("On Wifi: http://%s:%s\n", ip, *portPtr)
	fmt.Printf("On PC: http://localhost:%s\n", *portPtr)

	log.Fatal(http.ListenAndServe(":"+*portPtr , nil))
}
