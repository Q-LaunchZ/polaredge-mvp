package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"polaredge-agent/internal/renderer"
	"polaredge-agent/internal/traefik"
	"sync"
	"time"
)

const (
	configPath = "/tmp/polaredge.toml"
	portMin    = 7000
	portMax    = 7100
	socketPort = ":9005"
)

var (
	queue      = make(chan []byte, 100)
	processing sync.Mutex
)

// getFreePortInRange returns a free port within the defined port range.
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

// handle manages a single TCP connection from the client.
func handle(conn net.Conn) {
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("❌ Read error: %v", err)
		return
	}
	data := buf[:n]

	// Confirm receipt to client
	_, _ = conn.Write([]byte("ok"))

	// Queue data for async processing
	queue <- data
}

// processManifest renders and applies the new TOML manifest.
func processManifest(data []byte) {
	defer processing.Unlock()

	toml, err := renderer.RenderTOMLFromJSONWithPrompt(data)
	if err != nil {
		log.Printf("❌ Failed to render TOML: %v", err)
		return
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Printf("❌ mkdir error: %v", err)
		return
	}
	if err := os.WriteFile(configPath, []byte(toml), 0644); err != nil {
		log.Printf("❌ file write error: %v", err)
		return
	}
	log.Printf("✅ TOML written to %s", configPath)

	log.Println("🔁 Reloading Traefik with new config...")
	if err := traefik.RunWithConfig(configPath); err != nil {
		log.Printf("❌ Traefik reload failed: %v", err)
	} else {
		log.Println("🚀 Traefik exited cleanly.")
	}
}

// queueWorker processes queued manifest updates sequentially.
func queueWorker() {
	for data := range queue {
		processing.Lock()
		processManifest(data)
	}
}

// main initializes and runs the POLAREDGE agent.
func main() {
	log.Println("🚀 POLAREDGE Agent starting...")

	// Ensure Traefik is installed
	if !traefik.IsInstalled() {
		log.Println("⚠️  Traefik not found.")
		if err := traefik.Install(); err != nil {
			log.Fatalf("❌ Failed to install Traefik: %v", err)
		}
		log.Println("✅ Traefik installed.")
	}

	if err := traefik.Verify(); err != nil {
		log.Fatalf("❌ Traefik verification failed: %v", err)
	}

	// Pick a free TCP port for later use (for app ingress)
	port, err := getFreePortInRange(portMin, portMax)
	if err != nil {
		log.Fatalf("❌ No free port found: %v", err)
	}
	log.Printf("✅ Free port selected: %d", port)

	// Start socket listener (API or ingress intent receiver)
	listener, err := net.Listen("tcp", socketPort)
	if err != nil {
		log.Fatalf("❌ Listen error on %s: %v", socketPort, err)
	}
	log.Printf("📡 Agent listening on %s", socketPort)

	// Start async TOML processor
	go queueWorker()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("❌ Accept error: %v", err)
			continue
		}
		go handle(conn)
	}
}
