package main

import (
	"fmt"
	"log"
	"whatsbot/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	fmt.Printf("Config loaded: %+v\n", cfg)
}
