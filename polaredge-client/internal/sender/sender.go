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

// SendWithAck sends data to a TCP address and waits for an "ok" response.
func SendWithAck(addr string, payload []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	_, err = conn.Write(payload)
	if err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	// Wait for ACK
	buf := make([]byte, 8)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("read ack: %w", err)
	}
	if string(buf[:n]) != "ok" {
		return fmt.Errorf("unexpected ack: %q", buf[:n])
	}
	return nil
}
