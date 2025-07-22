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

func main() {
	reader := bufio.NewReader(os.Stdin)
	log.Println("📡 POLAREDGE Client started.")
	log.Println("Press 'r' to refresh and re-send ingress manifest.")

	for {
		log.Print("⏳ Waiting for input... ")
		input, _ := reader.ReadString('\n')

		if input == "r\n" || input == "R\n" {
			log.Println("🔁 Refresh triggered.")
			manifest := watcher.GetIngressManifest()
			ok := sendWithRetries(manifest, 3)
			if !ok {
				fmt.Println(string(manifest)) // fallback print
			}
		}
	}
}
