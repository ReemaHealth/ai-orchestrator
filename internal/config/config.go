package config

// Package config loads runtime configuration from environment variables.
// For local development, godotenv loads a .env file from the working directory.
// Required: FIREBASE_PROJECT_ID, SLACK_SIGNING_SECRET.
import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                   string // HTTP listen port (env PORT, default "8080")
	FirebaseProjectID      string // Identity Platform project id (env FIREBASE_PROJECT_ID)
	FirebaseSignInProvider string // Required JWT sign_in_provider (env FIREBASE_SIGN_IN_PROVIDER)
	SlackSigningSecret     string // Slack app signing secret (env SLACK_SIGNING_SECRET)
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
	}

	if cfg.FirebaseProjectID == "" {
		return Config{}, fmt.Errorf("FIREBASE_PROJECT_ID is required")
	}
	if cfg.SlackSigningSecret == "" {
		return Config{}, fmt.Errorf("SLACK_SIGNING_SECRET is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
