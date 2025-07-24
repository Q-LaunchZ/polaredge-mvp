package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"polaredge-client/internal/sender"
	"polaredge-client/internal/watcher"
	"time"
)

func sendWithRetries(manifest []byte, retries int) bool {
	for i := 0; i < retries; i++ {
		if err := sender.SendWithAck("localhost:9005", manifest); err != nil {
			log.Printf("⚠️  Send attempt %d failed: %v", i+1, err)
			time.Sleep(1 * time.Second)
			continue
		}
		log.Println("✅ TCP send confirmed.")
		return true
	}
	log.Println("❌ All send attempts failed. Will wait.")
	return false
}

func refreshAndSend() {
	log.Println("🔁 Refresh triggered.")
	ings := watcher.GetIngresses()
	data := watcher.EncodeIngresses(ings)
	if ok := sendWithRetries(data, 3); !ok {
		fmt.Println(string(data)) // fallback output
	}
}

func main() {
	log.Println("📡 POLAREDGE Client (Hybrid Mode)")
	log.Println("Press 'r' to manually trigger a refresh")

	// 1. Start keyboard listener in background
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			input, _ := reader.ReadString('\n')
			if input == "r\n" || input == "R\n" {
				refreshAndSend()
			}
		}
	}()

	// 2. Start Kubernetes ingress watcher
	watcher.StartWatcher(func(_ []watcher.Ingress) {
		log.Println("📶 Ingress event detected")
		refreshAndSend()
	})
}
