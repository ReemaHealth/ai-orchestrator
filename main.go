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

	"ai-orchestration/internal/agent"
	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/config"
	"ai-orchestration/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	firebaseVerifier, err := auth.NewFirebaseVerifier(ctx, cfg)
	if err != nil {
		log.Fatalf("firebase verifier: %v", err)
	}

	agentClient, err := buildAgentClient(ctx, cfg)
	if err != nil {
		log.Fatalf("agent client: %v", err)
	}

	srv := server.New(cfg, firebaseVerifier, agentClient)

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

func buildAgentClient(ctx context.Context, cfg config.Config) (agent.Client, error) {
	if !cfg.AgentEnabled {
		return agent.NewSkeletonClient(), nil
	}
	return agent.NewVertexClient(ctx, cfg)
}
