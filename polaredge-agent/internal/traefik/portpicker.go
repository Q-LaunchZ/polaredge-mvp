package traefik

import (
	"fmt"
	"net"
)

// GetFreePortInRange tries to find a free TCP port within a given range (inclusive).
func GetFreePortInRange(min, max int) (int, error) {
	for port := min; port <= max; port++ {
		addr := fmt.Sprintf(":%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			_ = ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port found in range %d-%d", min, max)
}
