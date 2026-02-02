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

	"golang.org/x/net/http2"
	"github.com/grandcat/zeroconf"
	"github.com/mdp/qrterminal/v3"
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

const (
	certFile = "cert.pem"
	keyFile = "key.pem"
)

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

const uploadTpl = `
<!DOCTYPE html>
<html>
<head>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Upload Files</title>
	<style>
		body { font-family: -apple-system, system-ui, sans-serif; background: #f0f2f5; padding: 20px; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; }
		.container { background: white; padding: 30px; border-radius: 12px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); width: 100%; max-width: 400px; text-align: center; }
		h1 { margin-top: 0; color: #333; }

		/* The Drag & Drop Zone */
		.upload-zone {
			border: 2px dashed #007bff;
			border-radius: 8px;
			padding: 40px 20px;
			margin: 20px 0;
			cursor: pointer;
			background: #f8faff;
			transition: background 0.2s;
			position: relative;
		}
		.upload-zone:hover { background: #eef; }
		.upload-zone p { margin: 0; color: #555; font-weight: 500; }
		.upload-zone input {
			position: absolute; width: 100%; height: 100%; top: 0; left: 0; opacity: 0; cursor: pointer;
		}

		/* The Submit Button */
		.btn {
			background: #007bff; color: white; border: none; padding: 12px 24px;
			border-radius: 6px; font-size: 16px; font-weight: bold; cursor: pointer; width: 100%;
			transition: background 0.2s;
		}
		.btn:hover { background: #0056b3; }
		.btn:disabled { background: #ccc; cursor: not-allowed; }
		.back-link { display: block; margin-top: 15px; color: #666; text-decoration: none; font-size: 14px; }

		/* The Queue List */
		#queue { list-style: none; padding: 0; margin: 20px 0; text-align: left; }
		.queue-item { background: #fff; border: 1px solid #eee; margin-bottom: 10px; padding: 10px; border-radius: 6px; position: relative; overflow: hidden; }

		/* The Progress Bar inside the list item */
		.progress-bg { position: absolute; top: 0; left: 0; height: 100%; background: #e6fffa; width: 0%; z-index: 0; transition: width 0.2s; }
		.item-text { position: relative; z-index: 1; display: flex; justify-content: space-between; font-size: 14px; }
		.status-icon { font-weight: bold; }
		/* progress bar */
		/*
		#progress-container { display: none; margin-top: 20px; background: #eee; border-radius: 6px; overflow: hidden; }
		#progress-bar { width: 0%; height: 20px; background: #28a745; transition: width 0.2s; }
		#status { margin-top: 10px; font-size: 14px; color: #555; }
		*/
	</style>
</head>
<body>
	<div class="container">
		<h1>⬆️ Upload Files</h1>
		<form id="uploadForm">
			<div class="upload-zone">
				<p>📂 Drag files here<br>or tap to browse</p>
				<input type="file" name="myFiles" id="fileInput" multiple onchange="handleSelect()">
			</div>

			<ul id="queue"></ul>

			<button id="uploadBtn" type="button" class="btn" onclick="startQueue()">Start Upload</button>
		</form>
		<a href="/" class="back-link">Cancel</a>
	</div>

	<script>
		let uploadQueue = [];
		let isUploading = false;

		function handleSelect() {
			const input = document.getElementById('fileInput');
			const list = document.getElementById('queue');

			for (let i = 0; i < input.files.length; i++) {
				uploadQueue.push({
					file: input.files[i],
					id: 'file-' + Date.now() + '-' + i,
					status: 'pending'
				});
			}
			renderQueue();
			document.getElementById('uploadBtn').style.display = "block";
			input.value = '';
		}


		function uploadFiles() {
			const input = document.getElementById('fileInput');
			if (input.files.length === 0) {
				alert("Please select a file.");
				return;
			}

			const files = input.files;
			const formData = new FormData();

			for (let i = 0; i < files.length; i++) {
				formData.append("myFiles", files[i])
			}

			document.getElementById('progress-container').style.display = 'block';

			const xhr = new XMLHttpRequest();
			xhr.open("POST", "/upload", true);

			xhr.upload.onprogress = function(e) {
				if (e.lengthComputable) {
					const percentComplete = (e.loaded / e.total) * 100;
					document.getElementById('progress-bar').style.width = percentComplete + "%";
					document.getElementById('status').innerText = Math.round(percentComplete) + '% uploaded';
				}
			};

			xhr.onload = function(e) {
				if (xhr.status == 200) {
					document.getElementById('status').innerText = "Done! Redirecting...";
					setTimeout(() => window.location.href = '/', 1000);
				} else {
					document.getElementById('status').innerText = "Error: " + xhr.statusText;
				}
			};
			xhr.send(formData);
		}
	</script>
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
		t, _ := template.New("upload").Parse(uploadTpl)
		t.Execute(w, nil)
		return
	}

	rawFileName := r.Header.Get("X-File-Name")

	if rawFileName != "" {
		fileName, _ := url.PathUnescape(rawFileName)
		fileName = filepath.Base(fileName)

		dst, err := os.Create(fileName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		_, err = io.Copy(dst, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Upload Success")
	}

	//MultipartReader (fallback)
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if part.FileName() == "" {
			continue
		}
		dst, err := os.Create(filepath.Base(part.FileName()))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			continue
		}
		_, err = io.Copy(dst, part)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			dst.Close()
			return
		}
		dst.Close()
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Upload Complete")
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

				currentURLPath := filepath.Join(r.URL.Path, entry.Name())
				downloadURL := ""

				if entry.IsDir() {
					downloadURL = fmt.Sprintf("/zip?path=%s", currentURLPath)
				} else {
					downloadURL = currentURLPath
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
		http.ServeFile(w, r, filePath)
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

	fullURL := fmt.Sprintf("http://%s:%s", ip, *portPtr)
	fmt.Println("\nScan to connect")

	qrterminal.GenerateHalfBlock(fullURL, qrterminal.L, os.Stdout)
	fmt.Println("")

	server := &http.Server{Addr: ":port"}
	http2.ConfigureServer(server, &http2.Server{})
	server.ListenAndServeTLS(certFile, keyFile)

}
