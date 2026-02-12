package main

import (
	"fileshare/internal/cleanup"
	"fileshare/internal/handlers"
	"fileshare/internal/network"
	"fileshare/internal/worker"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/mdp/qrterminal/v3"
)

func main() {
	portPtr := flag.String("port", "8080", "The port to run the server on")
	flag.Parse()

	numWorkers := runtime.NumCPU()
	uploadPool := worker.NewPool(numWorkers, 500)
	downloadPool := worker.NewPool(4, 20)
	uploadPool.Start()
	downloadPool.Start()
	defer uploadPool.Stop()
	defer downloadPool.Stop()

	currentDir, _ := os.Getwd()

	cleanup.StartCleanupRoutine(currentDir, 24*time.Hour, 1*time.Hour)

	http.HandleFunc("/", handlers.FileServerHandler(currentDir))
	http.HandleFunc("/upload", handlers.ChunkedUploadHandler())
	http.HandleFunc("/zip", handlers.ZipHandlerFactory(downloadPool))

	ip, iface := network.GetLocalIP()
	portInt, _ := strconv.Atoi(*portPtr)
	server, err := network.StartMDNS(portInt, ip, iface)
	if err == nil {
		defer server.Shutdown()
	}

	fullURL := fmt.Sprintf("http://%s:%s", ip, *portPtr)
	fmt.Printf("\n--- Server Running ---\n")
	fmt.Printf("Sharing: %s\n", currentDir)
	fmt.Printf("On domain: http://fileshare.local:%s\n", *portPtr)
	fmt.Printf("URL: %s\n", fullURL)

	qrterminal.GenerateHalfBlock(fullURL, qrterminal.L, os.Stdout)

	http := &http.Server{
		Addr: ":" + *portPtr,
		MaxHeaderBytes: 1 << 20,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	log.Fatal(http.ListenAndServe())
}
