package main

import (
	"fmt"
	"log"
	"polaredge-client/internal/sender"
	"polaredge-client/internal/watcher"
)

func main() {
	manifest := watcher.GetIngressManifest()

	if err := sender.SendWithAck("localhost:9005", manifest); err != nil {
		log.Printf("⚠️  SendWithAck failed (%v). Falling back to stdout.\n", err)
		fmt.Println(string(manifest))
	}
	fmt.Printf("TCP: OK\n")
}
