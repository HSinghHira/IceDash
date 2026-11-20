package host

import (
	"bufio"
	"net"
	"os"
	"strings"
)

// Iface is one of the server's network interfaces, for setup fields that
// ask for one (WatchYourLAN's IFACES, community feedback 2026-11-20).
type Iface struct {
	Name    string `json:"name"`
	Addr    string `json:"addr,omitempty"` // its first IPv4 address
	Default bool   `json:"default,omitempty"`
}

// virtual are the prefixes of interfaces Docker, VMs and overlays make;
// nobody means them when an app asks which network to watch.
var virtual = []string{"docker", "veth", "br-", "virbr", "vnet", "cni", "flannel", "kube", "lxc", "tap", "dummy"}

// Interfaces lists the server's real interfaces that are up and have an
// IPv4 address, the one with the default route marked.
func Interfaces() []Iface {
	return interfaces("/proc/net/route")
}

func interfaces(routes string) []Iface {
	list, err := net.Interfaces()
	if err != nil {
		return nil
	}
	def := defaultRoute(routes)
	out := []Iface{}
	for _, ni := range list {
		if ni.Flags&net.FlagUp == 0 || ni.Flags&net.FlagLoopback != 0 || isVirtual(ni.Name) {
			continue
		}
		addrs, _ := ni.Addrs()
		ip := ""
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				ip = n.IP.String()
				break
			}
		}
		if ip == "" {
			continue
		}
		out = append(out, Iface{Name: ni.Name, Addr: ip, Default: ni.Name == def})
	}
	return out
}

func isVirtual(name string) bool {
	for _, p := range virtual {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// defaultRoute reads the interface of the IPv4 default route from
// /proc/net/route (destination 00000000), "" when there is none.
func defaultRoute(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) > 2 && fields[1] == "00000000" && fields[0] != "Iface" {
			return fields[0]
		}
	}
	return ""
}
