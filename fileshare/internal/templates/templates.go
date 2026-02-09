// Package templates
package templates

const BrowseTpl = `
<!DOCTYPE html>
<html>
<head>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>File Server</title>
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
    <h1>My Shared Files</h1>
    <div style="background:white; padding: 10px; margin-bottom: 20px; border-radius: 8px;">
        {{range .BreadCrumbs}}
            <a href="{{.Link}}" style="text-decoration: none; color: #007bff; font-weight: bold;">{{.Name}}</a>
            <span style="color: #999;"> / </span>
        {{end}}
    </div>
    <a href="/upload?dir={{.CurrentPath}}" class="upload-btn">Upload New File</a>
    <div class="grid">
        {{range .Files}}
        <div class="card">
            <a href="{{.Path}}" class="card-content">
                <div class="icon">
                    {{if .IsDir}} 📁 {{else}} 📄 {{end}}
                </div>
                <div class="name">{{.Name}}</div>
                {{if not .IsDir}} <div class="size">{{.Size}}</div> {{end}}
            </a>
            {{if .DownloadURL}}
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

const UploadTpl = `
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

        .cancel-btn { display: block; margin-top: 15px; color: #666; background: none; border: none; font-size: 14px; cursor: pointer; width: 100%; text-decoration: underline;}
        .cancel-btn:hover { color: #333; }

        /* File List */
        #file-list { list-style: none; padding: 0; margin: 15px 0; text-align: left; }
        #file-list li { padding: 8px; border-bottom: 1px solid #eee; font-size: 14px; display: flex; justify-content: space-between; }
        #file-list li:last-child { border-bottom: none; }
        .count { background: #eee; padding: 2px 6px; border-radius: 4px; font-size: 12px; }

        /* Progress Bar */
        #progress-container { display: none; margin-top: 20px; background: #eee; border-radius: 6px; overflow: hidden; }
        #progress-bar { width: 0%; height: 20px; background: #28a745; transition: width 0.2s; }
        #status { margin-top: 10px; font-size: 14px; color: #555; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Upload Files</h1>
        <form id="uploadForm">
            <div class="upload-zone">
                <p>📂 Drag files here<br>or tap to browse</p>
                <input type="file" name="myFiles" id="fileInput" accept="*/*" multiple onchange="updateList()">
            </div>

            <p id="fileCount">No files selected</p>

            <ul id="file-list"></ul>

            <button id="uploadBtn" type="button" class="btn" onclick="uploadFiles()">Start Upload</button>

            <div id="progress-container">
                <div id="progress-bar"></div>
            </div>
        </form>

        <div id="status"></div>
        <button type="button" class="cancel-btn" onclick="cancelUpload()">Cancel / Go Back</button>
    </div>

    <script>
				const CHUNK_SIZE = 4 << 20

        const urlParams = new URLSearchParams(window.location.search);
        const targetDir = urlParams.get('dir') || "/";

				let uploadController = null
				let isUploading = false

        function formatSize(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }

        function updateList() {
            const input = document.getElementById('fileInput');
            const list = document.getElementById('file-list');
            const countLabel = document.getElementById('fileCount');

            if (!input || !list || !countLabel) return;

            countLabel.innerText = input.files.length + " file(s) selected.";
            list.innerHTML = '';

            if (input.files.length > 0) {
                for (let i = 0; i < input.files.length; i++) {
                    const li = document.createElement('li');
                    li.innerHTML = '<span>' + input.files[i].name + '</span>' + '<span class="count">' + formatSize(input.files[i].size) + '</span>';
                    list.appendChild(li);
                }
            }
						countLabel.innerText = input.files.length + " file(s) ready.";
        }


        async function uploadFiles() {
            const input = document.getElementById('fileInput');
            const files = input.files;
            const statusDisplay = document.getElementById('status');

            if (files.length === 0) {
                alert("Please select a file.");
                return;
            }

						if (isUploading) return
						isUploading = true;

						uploadController = new AbortController();

						document.getElementById('uploadBtn').disabled = true;
						document.getElementById('progress-container').style.display = 'block';

						let totalSize = 0;
						let totalUploaded = 0;

						for (let i = 0; i < files.length; i++) {
							totalSize += files[i].size;
						}

						for (let i = 0; i < files.length; i++) {
							const file = files[i]
							statusDisplay.innerText = "Uploading " + file.name + "...";


							try {
								let startTime = Date.now();
								let previousLoaded = 0;
								let speedStr = "0 B/s";

								await uploadFileInChunks(file, (fileProgress) => {
									const currentLoaded = fileProgress * file.size;
									const now = Date.now()
									const timeDiff = (now - startTime) / 1000;
									if (timeDiff > 0.2) {
										const bytesDiff = currentLoaded - previousLoaded;
										const speed = bytesDiff / timeDiff;
										speedStr = formatSize(speed) + "/s";

										startTime = now;
										previousLoaded = currentLoaded;
									}
									const overallProgress = ((totalUploaded + (fileProgress * file.size)) / totalSize) * 100;

									document.getElementById('progress-bar').style.width = overallProgress + '%';
									statusDisplay.innerText = "Uploading " + file.name + ": " + Math.round(fileProgress * 100) + "% (" + speedStr + ")";
								});
								totalUploaded += file.size;
							} catch (err) {
								if (err.name === 'AbortError') {
									statusDisplay.innerText = "Upload Cancelled."
								} else {
									statusDisplay.innerText = "Error: " + err.message;
								}
								isUploading = false
								document.getElementById('uploadBtn').disabled = false;
							}
						}
						statusDisplay.innerText = "Done! Redirecting...";
						setTimeout(() => {
							window.location.href = targetDir;
						}, 1000);
					}

					async function uploadFileInChunks(file, onProgress) {
						const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
						let uploadedBytes = 0;

						for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
							const start = chunkIndex * CHUNK_SIZE;
							const end = Math.min(start + CHUNK_SIZE, file.size);

							const chunk = file.slice(start, end);
							const isFinal = (chunkIndex === totalChunks - 1);

							const response = await fetch("/upload?dir="+encodeURIComponent(targetDir), {
								method: 'POST',
								headers: {
									'X-File-Name': file.name,
									'X-Chunk-Offset': start,
									'X-Final-Chunk': isFinal,
									'Content-Type': 'application/octet-stream'
								},
								body: chunk,
								signal: uploadController.signal
							});

							if (!response.ok) {
								throw new Error(await response.text())
							}

							uploadedBytes += (end - start);
							onProgress(uploadedBytes / file.size);
						}
					}
					function cancelUpload() {
						if (uploadController) {
							uploadController.abort();
						}
						window.location.href = targetDir
					}
    </script>
</body>
</html>
`
