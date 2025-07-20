package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	"polaredge-agent/internal/traefik"
)

const (
	configPath  = "/tmp/polaredge.toml"
	portMin     = 7000
	portMax     = 7100
	socketPort  = ":9005"
	serviceHost = "example.com"
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
	return 0, fmt.Errorf("no free port found in range %d-%d", min, max)
}

func writeInitialTOML(port int, path string) error {
	toml := fmt.Sprintf(`
[entryPoints.edge]
  address = ":%d"

[http.routers]
  [http.routers.myapp]
    rule = "Host('%s')"
    entryPoints = ["edge"]
    service = "myapp"

[http.services]
  [http.services.myapp.loadBalancer]
    [[http.services.myapp.loadBalancer.servers]]
      url = "http://localhost:8080"
`, port, serviceHost)

	return os.WriteFile(path, []byte(toml), 0644)
}

func handle(conn net.Conn) {
	defer conn.Close()

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Printf("mkdir error: %v", err)
		return
	}

	file, err := os.Create(configPath)
	if err != nil {
		log.Printf("file create error: %v", err)
		return
	}
	defer file.Close()

	n, err := io.Copy(file, conn)
	if err != nil {
		log.Printf("write error: %v", err)
		return
	}

	log.Printf("✅ TOML written to %s (%d bytes)", configPath, n)

	log.Println("🔁 Starting Traefik with new config...")
	if err := traefik.RunWithConfig(configPath); err != nil {
		log.Printf("❌ Traefik reload failed: %v", err)
	} else {
		log.Println("🚀 Traefik exited cleanly.")
	}
}

func main() {
	// 1. Install Traefik if missing
	if !traefik.IsInstalled() {
		fmt.Println("⚠️  Traefik not found.")
		if err := traefik.Install(); err != nil {
			fmt.Println("❌ Failed to install Traefik:", err)
			return
		}
		fmt.Println("✅ Traefik installed.")
	}

	// 2. Verify Traefik binary works
	if err := traefik.Verify(); err != nil {
		fmt.Println("❌ Traefik install appears broken:", err)
		return
	}

	// 3. Pick a free port in range
	port, err := getFreePortInRange(portMin, portMax)
	if err != nil {
		log.Fatalf("❌ No free port found: %v", err)
	}
	fmt.Printf("✅ Free port selected: %d\n", port)

	// 4. Write initial TOML (optional)
	if err := writeInitialTOML(port, configPath); err != nil {
		log.Fatalf("❌ Failed to write initial TOML: %v", err)
	}
	fmt.Printf("📄 Initial TOML written to %s\n", configPath)

	// 5. Start socket listener
	listener, err := net.Listen("tcp", socketPort)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	log.Printf("📡 Agent stub listening on %s", socketPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handle(conn)
	}
}
