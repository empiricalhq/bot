package main

import (
	"context"
	"log"
	"os"

	"github.com/mdp/qrterminal/v3"

	"whatsbot/internal/service"
)

func main() {
	ctx := context.Background()

	controller, err := service.NewController()
	if err != nil {
		log.Fatalf("Failed to initialize controller: %v", err)
	}

	defer controller.Shutdown(ctx)

	callbacks := service.ControllerCallbacks{
		OnQRCode: func(qrCode string) {
			qrterminal.GenerateHalfBlock(qrCode, qrterminal.L, os.Stdout)
		},
	}

	err = controller.Start(ctx, callbacks)
	if err != nil {
		log.Fatalf("Bot failed to start: %v", err)
	}
}
