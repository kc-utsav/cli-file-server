package main

import (
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

	"github.com/mdp/qrterminal/v3"
)

func main() {
	portPtr := flag.String("port", "8080", "The port to run the server on")
	flag.Parse()

	numWorkers := runtime.NumCPU()
	wp := worker.NewPool(numWorkers, 10)
	wp.Start()
	defer wp.Stop()

	currentDir, _ := os.Getwd()

	http.HandleFunc("/", handlers.FileServerHandler(currentDir))
	http.HandleFunc("/upload", handlers.UploadHandlerFactory(wp))
	http.HandleFunc("/zip", handlers.ZipHandlerFactory(wp))

	ip, iface := network.GetLocalIP()
	portInt, _ := strconv.Atoi(*portPtr)
	server, err := network.StartMDNS(portInt, ip, iface)
	if err != nil {
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
	}

	log.Fatal(http.ListenAndServe())
}
