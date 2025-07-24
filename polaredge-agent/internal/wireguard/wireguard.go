package wireguard

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/songgao/water"
)

func StartTUN(ip string) error {
	// Create TUN device
	cfg := water.Config{
		DeviceType: water.TUN,
	}
	iface, err := water.New(cfg)
	if err != nil {
		return fmt.Errorf("create TUN: %w", err)
	}

	log.Println("TUN device created:", iface.Name())

	// Assign IP using system call (temporary)
	cmd := exec.Command("ifconfig", iface.Name(), ip, ip, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("assign IP: %w", err)
	}

	log.Println("Assigned", ip, "to", iface.Name())
	return nil
}
