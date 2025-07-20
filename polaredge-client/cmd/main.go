package main

import (
	"fmt"
	"log"
	"polaredge-client/internal/renderer"
	"polaredge-client/internal/sender"
	"polaredge-client/internal/watcher"
)

func main() {
	ingresses := watcher.GetIngresses()
	tomlOutput := renderer.RenderTOML(ingresses)

	if err := sender.Send("localhost:9005", []byte(tomlOutput)); err != nil {
		log.Printf("⚠️  socket send failed (%v). Falling back to stdout.\n", err)
		fmt.Println(tomlOutput)
	}
}
