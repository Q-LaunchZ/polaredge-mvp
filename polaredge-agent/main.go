package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"polaredge-agent/internal/renderer"
	"polaredge-agent/internal/traefik"
	"time"
)

const (
	configPath = "/tmp/polaredge.toml"
	portMin    = 7000
	portMax    = 7100
	socketPort = ":9005"
)

func getFreePortInRange(min, max int) (int, error) {
	for port := min; port <= max; port++ {
		addr := fmt.Sprintf(":%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			_ = ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port found in range %d–%d", min, max)
}

func handle(conn net.Conn) {
	defer conn.Close()

	// Fix: read once with timeout instead of blocking forever
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("❌ Read error: %v", err)
		return
	}
	data := buf[:n]
	log.Printf("📥 Received %d bytes", len(data))

	// Render TOML
	toml, err := renderer.RenderTOMLFromJSON(data)
	if err != nil {
		log.Printf("❌ Failed to render TOML: %v", err)
		_, _ = conn.Write([]byte("error"))
		return
	}

	// Save to config file
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Printf("mkdir error: %v", err)
		return
	}
	if err := os.WriteFile(configPath, []byte(toml), 0644); err != nil {
		log.Printf("file write error: %v", err)
		return
	}
	log.Printf("✅ TOML written to %s", configPath)

	// Respond to client
	_, _ = conn.Write([]byte("ok"))

	// Run Traefik
	log.Println("🔁 Starting Traefik with new config...")
	if err := traefik.RunWithConfig(configPath); err != nil {
		log.Printf("❌ Traefik reload failed: %v", err)
	} else {
		log.Println("🚀 Traefik exited cleanly.")
	}
}

func main() {
	log.Println("🚀 POLAREDGE Agent starting...")

	if !traefik.IsInstalled() {
		fmt.Println("⚠️  Traefik not found.")
		if err := traefik.Install(); err != nil {
			fmt.Println("❌ Failed to install Traefik:", err)
			return
		}
		fmt.Println("✅ Traefik installed.")
	}

	if err := traefik.Verify(); err != nil {
		fmt.Println("❌ Traefik install appears broken:", err)
		return
	}

	port, err := getFreePortInRange(portMin, portMax)
	if err != nil {
		log.Fatalf("❌ No free port found: %v", err)
	}
	fmt.Printf("✅ Free port selected: %d\n", port)

	listener, err := net.Listen("tcp", socketPort)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	log.Printf("📡 Agent listening on %s", socketPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handle(conn)
	}
}
