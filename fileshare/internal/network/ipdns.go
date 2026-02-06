//Package network
package network

import (
	"log"
	"net"

	"github.com/grandcat/zeroconf"
)

func GetLocalIP() (string, *net.Interface) {
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

func StartMDNS(port int, ip string, iface *net.Interface) (*zeroconf.Server, error){

	if iface == nil {
		log.Println("No suitable network for mDNS")
		return nil, nil
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
		return nil, err
	}
	log.Printf("mDNS active: http://%s:%d (Bound to %s)", hostName, port, iface.Name)

	return server, nil
}
