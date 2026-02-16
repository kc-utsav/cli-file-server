const CHUNK_SIZE = 4 << 20
const PARALLEL_CHUNKS = 4

const urlParams = new URLSearchParams(window.location.search);
const targetDir = urlParams.get('dir') || "/";

let uploadController = null;
let isUploading = false;

function formatSize(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function updateList() {
  const list = document.getElementById('file-list');
  const countLabel = document.getElementById('fileCount');

  if (!list || !countLabel) return;

  const allFiles = getAllFiles();

  countLabel.innerText = allFiles.length + " file(s) selected.";
  list.innerHTML = '';

  for (let i = 0; i < allFiles.length; i++) {
    const li = document.createElement('li');
    const displayName = allFiles[i].webkitRelativePath || allFiles[i].name;
    li.innerHTML = '<span>' + displayName + '</span>' +
      '<span class="count">' + formatSize(allFiles[i].size) + '</span>';
    list.appendChild(li);
  }
}

function getAllFiles() {
  const fileInput = document.getElementById('fileInput');
  const folderInput = document.getElementById('folderInput');
  const files = [];

  if (fileInput && fileInput.files) {
    for (let i = 0; i < fileInput.files.length; i++) {
      files.push(fileInput.files[i]);
    }
  }

  if (folderInput && folderInput.files) {
    for (let i = 0; i < folderInput.files.length; i++) {
      files.push(folderInput.files[i]);
    }
  }
  return files;
}

async function uploadFiles() {
  const files = getAllFiles();
  const statusDisplay = document.getElementById('status');

  if (files.length === 0) {
    alert("Please select a file or a folder.");
    return;
  }

  if (isUploading) return;
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
    const file = files[i];
    const relativePath = file.webkitRelativePath || file.name;
    statusDisplay.innerText = "Uploading " + relativePath + "...";

    try {
      // Speed tracking
      let speedStartTime = Date.now();
      let speedPreviousBytes = 0;
      let speedStr = "calculating...";

      await uploadFileInChunks(file, (uploadedBytes) => {
        // Speed calculation
        const now = Date.now();
        const timeDiff = (now - speedStartTime) / 1000;
        if (timeDiff > 0.5) {
          const bytesDiff = uploadedBytes - speedPreviousBytes;
          const speed = bytesDiff / timeDiff;
          speedStr = formatSize(speed) + "/s";
          speedStartTime = now;
          speedPreviousBytes = uploadedBytes;
        }

        // Progress bar
        const overallBytes = totalUploaded + uploadedBytes;
        const overallPercent = (overallBytes / totalSize) * 100;
        const filePercent = Math.round((uploadedBytes / file.size) * 100);

        document.getElementById('progress-bar').style.width = overallPercent + '%';
        statusDisplay.innerText = "Uploading " + relativePath +
          ": " + filePercent + "% (" + speedStr + ")";
      });

      totalUploaded += file.size;
    } catch (err) {
      if (err.name === 'AbortError') {
        statusDisplay.innerText = "Upload cancelled.";
      } else {
        statusDisplay.innerText = "Error: " + err.message;
      }
      isUploading = false;
      document.getElementById('uploadBtn').disabled = false;
      return;
    }
  }

  statusDisplay.innerText = "Done! Redirecting...";
  setTimeout(() => {
    window.location.href = targetDir;
  }, 1000);
}

// Upload a single file in parallel chunks with per-chunk progress
async function uploadFileInChunks(file, onProgress) {
  const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
  let uploadedBytes = 0;
  const relativePath = file.webkitRelativePath || file.name;

  let nextChunk = 0;
  const activeUploads = new Set();

  function startChunkUpload(chunkIndex) {
    const start = chunkIndex * CHUNK_SIZE;
    const end = Math.min(start + CHUNK_SIZE, file.size);
    const chunk = file.slice(start, end);

    const promise = fetch("/upload?dir=" + encodeURIComponent(targetDir), {
      method: 'POST',
      headers: {
        'X-File-Name': relativePath,
        'X-Chunk-Offset': String(start),
        'X-Final-Chunk': 'false',
        'Content-Type': 'application/octet-stream'
      },
      body: chunk,
      signal: uploadController.signal
    }).then(async (response) => {
      if (!response.ok) throw new Error(await response.text());

      uploadedBytes += (end - start);
      onProgress(uploadedBytes);

      // Remove from active set so new chunks can start
      activeUploads.delete(promise);
    });

    // Track this pending promise
    activeUploads.add(promise);
    return promise;
  }

  // Start initial batch of parallel uploads
  while (nextChunk < totalChunks && activeUploads.size < PARALLEL_CHUNKS) {
    startChunkUpload(nextChunk);
    nextChunk++;
  }

  // As each chunk finishes, start another
  while (activeUploads.size > 0) {
    await Promise.race([...activeUploads]);

    while (nextChunk < totalChunks && activeUploads.size < PARALLEL_CHUNKS) {
      startChunkUpload(nextChunk);
      nextChunk++;
    }
  }

  const finalResponse = await fetch("/upload?dir=" + encodeURIComponent(targetDir), {
    method: 'POST',
    headers: {
      'X-File-Name': relativePath,
      'X-Chunk-Offset': '0',
      'X-Final-Chunk': 'true',
      'Content-Type': 'application/octet-stream'
    },
    body: null,
    signal: uploadController.signal
  });

  if (!finalResponse.ok) {
    throw new Error(await finalResponse.text());
  }
}
function cancelUpload() {
  if (uploadController) {
    uploadController.abort();
  }
  window.location.href = targetDir;
}
