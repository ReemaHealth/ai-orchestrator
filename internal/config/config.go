package config

// Package config loads runtime configuration from environment variables.
// For local development, godotenv loads a .env file from the working directory.
// Required: FIREBASE_PROJECT_ID, SLACK_SIGNING_SECRET.
// When AGENT_ENABLED=true, GCP_PROJECT, GCP_LOCATION, and REASONING_ENGINE_ID are also required.
import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                   string // HTTP listen port (env PORT, default "8080")
	FirebaseProjectID      string // Identity Platform project id (env FIREBASE_PROJECT_ID)
	FirebaseSignInProvider string // Required JWT sign_in_provider (env FIREBASE_SIGN_IN_PROVIDER)
	SlackSigningSecret     string // Slack app signing secret (env SLACK_SIGNING_SECRET)
	AgentEnabled           bool   // When true, call Vertex Reasoning Engine (env AGENT_ENABLED)
	GCPProject             string // GCP project hosting the reasoning engine (env GCP_PROJECT)
	GCPLocation            string // Vertex region, e.g. us-central1 (env GCP_LOCATION)
	ReasoningEngineID      string // Reasoning engine resource id (env REASONING_ENGINE_ID)
	AgentClassMethod       string // Reasoning engine class method (env AGENT_CLASS_METHOD)
}

// Load reads configuration from the environment. It attempts to load .env via godotenv
// (ignored if missing). Returns an error if required variables are unset.
func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Port:                   envOrDefault("PORT", "8080"),
		FirebaseProjectID:      os.Getenv("FIREBASE_PROJECT_ID"),
		FirebaseSignInProvider: envOrDefault("FIREBASE_SIGN_IN_PROVIDER", "google.com"),
		SlackSigningSecret:     os.Getenv("SLACK_SIGNING_SECRET"),
		AgentEnabled:           envBool("AGENT_ENABLED"),
		GCPProject:             os.Getenv("GCP_PROJECT"),
		GCPLocation:            os.Getenv("GCP_LOCATION"),
		ReasoningEngineID:      os.Getenv("REASONING_ENGINE_ID"),
		AgentClassMethod:       envOrDefault("AGENT_CLASS_METHOD", "stream_query"),
	}

	if cfg.FirebaseProjectID == "" {
		return Config{}, fmt.Errorf("FIREBASE_PROJECT_ID is required")
	}
	if cfg.SlackSigningSecret == "" {
		return Config{}, fmt.Errorf("SLACK_SIGNING_SECRET is required")
	}

	if cfg.AgentEnabled {
		if cfg.GCPProject == "" {
			return Config{}, fmt.Errorf("GCP_PROJECT is required when AGENT_ENABLED=true")
		}
		if cfg.GCPLocation == "" {
			return Config{}, fmt.Errorf("GCP_LOCATION is required when AGENT_ENABLED=true")
		}
		if cfg.ReasoningEngineID == "" {
			return Config{}, fmt.Errorf("REASONING_ENGINE_ID is required when AGENT_ENABLED=true")
		}
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
