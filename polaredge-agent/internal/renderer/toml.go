package renderer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// Ingress is the same structure as what client sends
type Ingress struct {
	Host        string `json:"host"`
	ServiceName string `json:"serviceName"`
	ServicePort int    `json:"servicePort"`
}

// cache tracks exposure decisions per unique host:port
var exposureCache = make(map[string]bool)

// RenderTOMLFromJSON renders full TOML config from raw JSON ingress list
func RenderTOMLFromJSON(raw []byte) (string, error) {
	var ingresses []Ingress
	if err := json.Unmarshal(raw, &ingresses); err != nil {
		return "", fmt.Errorf("unmarshal ingress list: %w", err)
	}
	return renderFromIngressList(ingresses)
}

// RenderTOMLFromJSONWithPrompt prompts for exposure on high ports
func RenderTOMLFromJSONWithPrompt(raw []byte) (string, error) {
	var ingresses []Ingress
	if err := json.Unmarshal(raw, &ingresses); err != nil {
		return "", fmt.Errorf("unmarshal ingress list: %w", err)
	}

	filtered := []Ingress{}
	seen := make(map[string]bool)

	for _, ing := range ingresses {
		key := fmt.Sprintf("%s:%d", ing.Host, ing.ServicePort)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Always allow low ports
		if ing.ServicePort <= 443 {
			filtered = append(filtered, ing)
			continue
		}

		// Use cache if available
		if allowed, ok := exposureCache[key]; ok {
			if allowed {
				filtered = append(filtered, ing)
			}
			continue
		}

		// Prompt user
		fmt.Printf("\n🚧 [POLAREDGE] New Ingress route detected: %s\n", ing.ServiceName)
		fmt.Printf("    Host: %s\n", ing.Host)
		fmt.Printf("    Service: %s:%d\n", ing.ServiceName, ing.ServicePort)
		fmt.Printf("\n⚠️  This route targets port %d, which is outside typical web ranges.\n", ing.ServicePort)
		fmt.Println("\nChoose exposure mode:")
		fmt.Println("    [Y] Public (expose via Traefik)")
		fmt.Println("    [P] Private (cluster-only)")
		fmt.Println("    [N] Off (ignore, no exposure) ← default in 60s")
		fmt.Print("\nYour choice [N/Y/P]: ")

		choice := getUserChoiceWithCountdownClean(60 * time.Second)
		choice = strings.TrimSpace(strings.ToLower(choice))

		switch choice {
		case "y":
			// Check if port is already in use
			if IsPortInUse(ing.ServicePort) {
				newPort, err := FindNextFreePort(7000, 7100)
				if err != nil {
					fmt.Println("❌ No free ports available. Skipping this route.")
					exposureCache[key] = false
					continue
				}

				if PromptUserPortSwitch(ing.ServicePort, newPort) {
					fmt.Printf("✅ Using port %d instead of %d.\n", newPort, ing.ServicePort)
					ing.ServicePort = newPort
				} else {
					fmt.Println("❌ Skipping due to user decline.")
					exposureCache[key] = false
					continue
				}
			}

			filtered = append(filtered, ing)
			exposureCache[key] = true

		case "p":
			filtered = append(filtered, ing)
			exposureCache[key] = true

		default:
			exposureCache[key] = false
		}
	}

	return renderFromIngressList(filtered)
}

// Internal helper for actual TOML rendering
func renderFromIngressList(ingresses []Ingress) (string, error) {
	var buf bytes.Buffer

	// 1. EntryPoints
	buf.WriteString("[entryPoints]\n")
	seenPorts := make(map[int]string)
	for _, ing := range ingresses {
		name := getEntryPointName(ing.ServicePort)
		if _, exists := seenPorts[ing.ServicePort]; !exists {
			seenPorts[ing.ServicePort] = name
			buf.WriteString(fmt.Sprintf("  [entryPoints.%s]\n", name))
			buf.WriteString(fmt.Sprintf("    address = \":%d\"\n", ing.ServicePort))
		}
	}

	// 2. Routers
	buf.WriteString("\n[http]\n  [http.routers]\n")
	routerSet := make(map[string]string)

	for _, ing := range ingresses {
		routerName := ing.ServiceName
		rule := fmt.Sprintf("Host(`%s`)", ing.Host)
		entryPoint := getEntryPointName(ing.ServicePort)

		if existingRule, ok := routerSet[routerName]; ok && existingRule == rule {
			continue
		}
		routerSet[routerName] = rule

		buf.WriteString(fmt.Sprintf("    [http.routers.%s]\n", routerName))
		buf.WriteString(fmt.Sprintf("      rule = \"%s\"\n", rule))
		buf.WriteString(fmt.Sprintf("      entryPoints = [\"%s\"]\n", entryPoint))
		buf.WriteString(fmt.Sprintf("      service = \"%s\"\n", ing.ServiceName))
	}

	// 3. Services
	buf.WriteString("  [http.services]\n")
	servers := make(map[string][]string)

	for _, ing := range ingresses {
		url := fmt.Sprintf("http://%s:%d", ing.ServiceName, ing.ServicePort)
		key := ing.ServiceName

		found := false
		for _, existing := range servers[key] {
			if existing == url {
				found = true
				break
			}
		}
		if !found {
			servers[key] = append(servers[key], url)
		}
	}

	for serviceName, urls := range servers {
		buf.WriteString(fmt.Sprintf("    [http.services.%s.loadBalancer]\n", serviceName))
		for _, url := range urls {
			buf.WriteString(fmt.Sprintf("      [[http.services.%s.loadBalancer.servers]]\n", serviceName))
			buf.WriteString(fmt.Sprintf("        url = \"%s\"\n", url))
		}
	}

	return buf.String(), nil
}

// Maps port to entryPoint name
func getEntryPointName(port int) string {
	switch port {
	case 80:
		return "web"
	case 443:
		return "websecure"
	case 22:
		return "ssh"
	case 2222:
		return "sshalt"
	default:
		return fmt.Sprintf("port%d", port)
	}
}

// Clean countdown + input fallback after timeout
func getUserChoiceWithCountdownClean(timeout time.Duration) string {
	inputCh := make(chan string, 1)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		inputCh <- text
	}()

	start := int(timeout.Seconds())

	for i := start; i > 0; i-- {
		fmt.Printf("\r⌛ %d seconds remaining... ", i)
		select {
		case input := <-inputCh:
			fmt.Print("\r\033[K") // clear line
			return input
		case <-time.After(1 * time.Second):
			continue
		}
	}

	fmt.Print("\r\033[K") // clear line
	fmt.Println("⏱️ No response — defaulting to [N]")
	return "n"
}

// Port check helpers

func IsPortInUse(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return true
	}
	_ = l.Close()
	return false
}

func FindNextFreePort(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		if !IsPortInUse(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port found in range %d–%d", start, end)
}

func PromptUserPortSwitch(oldPort, newPort int) bool {
	fmt.Printf("⚠️  Port %d is already in use.\n", oldPort)
	fmt.Printf("Use alternative port %d? [Y/N]: ", newPort)

	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	input := strings.TrimSpace(strings.ToLower(text))
	return input == "y" || input == "yes"
}
