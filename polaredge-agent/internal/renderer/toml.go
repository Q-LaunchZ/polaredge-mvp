package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Ingress is the same structure as what client sends
type Ingress struct {
	Host        string `json:"host"`
	ServiceName string `json:"serviceName"`
	ServicePort int    `json:"servicePort"`
}

// RenderTOMLFromJSON renders full TOML config from raw JSON ingress list
func RenderTOMLFromJSON(raw []byte) (string, error) {
	var ingresses []Ingress
	if err := json.Unmarshal(raw, &ingresses); err != nil {
		return "", fmt.Errorf("unmarshal ingress list: %w", err)
	}

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
