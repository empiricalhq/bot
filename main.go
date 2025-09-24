package main

import (
	"fmt"
	"log"
	"whatsbot/internal/config"

	"whatsbot/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	logger := logger.NewLogger("main", logger.INFO)
	logger.Info(fmt.Sprintf("Config loaded: %+v", cfg))
}
