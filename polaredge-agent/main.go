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

var queue = make(chan []byte, 100)
var processing sync.Mutex

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

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("❌ Read error: %v", err)
		return
	}
	data := buf[:n]
	log.Printf("📥 Received %d bytes", len(data))

	// ✅ Confirm to client: it's accepted and safe
	_, _ = conn.Write([]byte("ok"))
	log.Println("📬 Acknowledged to client: ok")

	// 🔁 Queue it for async processing
	queue <- data
}

func processManifest(data []byte) {
	defer processing.Unlock()

	toml, err := renderer.RenderTOMLFromJSONWithPrompt(data)
	if err != nil {
		log.Printf("❌ Failed to render TOML: %v", err)
		return
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Printf("mkdir error: %v", err)
		return
	}
	if err := os.WriteFile(configPath, []byte(toml), 0644); err != nil {
		log.Printf("file write error: %v", err)
		return
	}
	log.Printf("✅ TOML written to %s", configPath)

	log.Println("🔁 Starting Traefik with new config...")
	if err := traefik.RunWithConfig(configPath); err != nil {
		log.Printf("❌ Traefik reload failed: %v", err)
	} else {
		log.Println("🚀 Traefik exited cleanly.")
	}
}

func queueWorker() {
	for data := range queue {
		processing.Lock()
		processManifest(data)
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

	go queueWorker()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handle(conn)
	}
}
