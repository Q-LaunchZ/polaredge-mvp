package sender

import (
	"fmt"
	"net"
	"time"
)

// Send writes raw data to a TCP socket and returns an error if it fails.
func Send(addr string, payload []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	_, err = conn.Write(payload)
	if err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}
