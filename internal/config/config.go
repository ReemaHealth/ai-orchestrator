package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                   string
	FirebaseProjectID      string
	FirebaseSignInProvider string
	SlackSigningSecret     string
}

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
