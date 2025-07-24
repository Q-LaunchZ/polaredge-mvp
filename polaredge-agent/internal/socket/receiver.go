package socket

import (
	"bufio"
	"log"
	"net"
	"os"
)

func StartTCPReceiver() {
	listen, err := net.Listen("tcp", "localhost:9005")
	if err != nil {
		log.Fatalf("❌ Failed to bind TCP socket: %v", err)
	}
	log.Println("📡 Agent listening on TCP :9005")

	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Printf("⚠️  Accept failed: %v", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	data, err := bufio.NewReader(conn).ReadBytes(0)
	if err != nil {
		log.Printf("⚠️  Read failed: %v", err)
		return
	}

	// Remove null terminator if sent
	data = trimNullBytes(data)

	// Save for inspection
	_ = os.WriteFile("received_manifest.json", data, 0644)
	log.Printf("✅ Received ingress data (%d bytes), saved to received_manifest.json\n", len(data))

	// Respond with ACK
	_, _ = conn.Write([]byte("ok"))
}

func trimNullBytes(data []byte) []byte {
	for i, b := range data {
		if b == 0 {
			return data[:i]
		}
	}
	return data
}
