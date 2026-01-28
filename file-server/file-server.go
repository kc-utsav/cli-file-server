package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

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

func main(){

	portPtr := flag.String("port", "8080", "The port to run the server on")
	flag.Parse()

	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = "."
	}

	fileHandler := http.FileServer(http.Dir(currentDir))
	wrappedHandler := loggingMiddleware(fileHandler)
	http.Handle("/", wrappedHandler)

	ip := getLocalIP()

	fmt.Printf("Sharing: %s\n", currentDir)
	fmt.Printf("On Wifi: http://%s:%s\n", ip, *portPtr)
	fmt.Printf("On PC: http://localhost:%s\n", *portPtr)

	log.Fatal(http.ListenAndServe(":"+*portPtr , nil))
}
