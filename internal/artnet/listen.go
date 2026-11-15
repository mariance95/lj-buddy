package artnet

import (
	"context"
	"fmt"
	"net"
)

// Listen abre el socket UDP de Art-Net en la IP indicada ("" o "0.0.0.0" para
// todas). Se activa SO_REUSEADDR para convivir con otros programas Art-Net en
// el mismo equipo.
func Listen(ip string) (*net.UDPConn, error) {
	if ip == "" {
		ip = "0.0.0.0"
	}
	lc := net.ListenConfig{Control: reuseControl}
	pc, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf("%s:%d", ip, Port))
	if err != nil {
		return nil, err
	}
	return pc.(*net.UDPConn), nil
}

// LocalIPFor devuelve la IP y MAC locales de la interfaz que comparte subred
// con remote (la que hay que anunciar en ArtPollReply).
func LocalIPFor(remote net.IP) (net.IP, net.HardwareAddr) {
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.Contains(remote) {
				return n.IP.To4(), ifc.HardwareAddr
			}
		}
	}
	if c, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: remote, Port: Port}); err == nil {
		defer c.Close()
		return c.LocalAddr().(*net.UDPAddr).IP.To4(), nil
	}
	return net.IPv4zero.To4(), nil
}

// LocalIPs lista las IPv4 locales (para elegir en qué interfaz escuchar).
func LocalIPs() []string {
	var out []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
			out = append(out, n.IP.String())
		}
	}
	return out
}
