package main

// ai-orchestrator: HTTP API for Lasso with Firebase and Slack cryptographic auth.
// See README.md and docs/ for architecture, endpoints, and testing guides.
import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/config"
	"ai-orchestration/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	firebaseVerifier, err := auth.NewFirebaseVerifier(context.Background(), cfg)
	if err != nil {
		log.Fatalf("firebase verifier: %v", err)
	}

	srv := server.New(cfg, firebaseVerifier)

	go func() {
		fmt.Printf("Starting server on port %s...\n", cfg.Port)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}
