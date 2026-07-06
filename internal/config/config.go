package config

// Package config loads runtime configuration from environment variables.
// For local development, godotenv loads a .env file from the working directory.
// Required: FIREBASE_PROJECT_ID, SLACK_SIGNING_SECRET.
// When AGENT_ENABLED=true, GCP_PROJECT, GCP_LOCATION, and REASONING_ENGINE_ID are also required.
// When Google OAuth is configured, user-scoped tokens are resolved server-side for /prompt.
import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const defaultGoogleOAuthScope = "https://www.googleapis.com/auth/cloud-platform"

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
	GEAuthStateKey         string // ADK session state key for user OAuth token (env AGENT_GE_AUTH_STATE_KEY)

	// Google OAuth for user-scoped workspace datastore access (optional).
	UserOAuthMode              string   // auto, required, client_only, disabled (env USER_OAUTH_MODE)
	GoogleOAuthClientID        string   // env GOOGLE_OAUTH_CLIENT_ID
	GoogleOAuthClientSecret    string   // env GOOGLE_OAUTH_CLIENT_SECRET
	GoogleOAuthRedirectURI     string   // env GOOGLE_OAUTH_REDIRECT_URI
	GoogleOAuthSuccessRedirect string   // post-consent browser redirect (env GOOGLE_OAUTH_SUCCESS_REDIRECT)
	GoogleOAuthStateSecret     string   // HMAC secret for OAuth state (env GOOGLE_OAUTH_STATE_SECRET)
	GoogleOAuthScopes          []string // comma-separated env GOOGLE_OAUTH_SCOPES
	GoogleOAuthAllowedDomains  []string // comma-separated env GOOGLE_OAUTH_ALLOWED_DOMAINS
}

// Load reads configuration from the environment. It attempts to load .env via godotenv
// (ignored if missing). Returns an error if required variables are unset.
func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Port:                       envOrDefault("PORT", "8080"),
		FirebaseProjectID:          os.Getenv("FIREBASE_PROJECT_ID"),
		FirebaseSignInProvider:     envOrDefault("FIREBASE_SIGN_IN_PROVIDER", "google.com"),
		SlackSigningSecret:         os.Getenv("SLACK_SIGNING_SECRET"),
		AgentEnabled:               envBool("AGENT_ENABLED"),
		GCPProject:                 os.Getenv("GCP_PROJECT"),
		GCPLocation:                os.Getenv("GCP_LOCATION"),
		ReasoningEngineID:          os.Getenv("REASONING_ENGINE_ID"),
		AgentClassMethod:           envOrDefault("AGENT_CLASS_METHOD", "async_stream_query"),
		GEAuthStateKey:             envOrDefault("AGENT_GE_AUTH_STATE_KEY", "fetch-agent-auth"),
		UserOAuthMode:              envOrDefault("USER_OAUTH_MODE", "auto"),
		GoogleOAuthClientID:        os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthClientSecret:    os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		GoogleOAuthRedirectURI:     os.Getenv("GOOGLE_OAUTH_REDIRECT_URI"),
		GoogleOAuthSuccessRedirect: os.Getenv("GOOGLE_OAUTH_SUCCESS_REDIRECT"),
		GoogleOAuthStateSecret:     os.Getenv("GOOGLE_OAUTH_STATE_SECRET"),
		GoogleOAuthScopes:          parseCSVEnv("GOOGLE_OAUTH_SCOPES", defaultGoogleOAuthScope),
		GoogleOAuthAllowedDomains:  parseCSVEnv("GOOGLE_OAUTH_ALLOWED_DOMAINS", ""),
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

	if cfg.GoogleOAuthStateSecret == "" {
		cfg.GoogleOAuthStateSecret = cfg.GoogleOAuthClientSecret
	}
	if cfg.GoogleOAuthStateSecret == "" {
		cfg.GoogleOAuthStateSecret = cfg.SlackSigningSecret
	}

	return cfg, nil
}

// GoogleOAuthConfigured reports whether server-side user OAuth token resolution is enabled.
func (c Config) GoogleOAuthConfigured() bool {
	return strings.TrimSpace(c.GoogleOAuthClientID) != "" &&
		strings.TrimSpace(c.GoogleOAuthClientSecret) != "" &&
		strings.TrimSpace(c.GoogleOAuthRedirectURI) != ""
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

func parseCSVEnv(key, fallback string) []string {
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		raw = fallback
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
