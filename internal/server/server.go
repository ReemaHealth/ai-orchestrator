package server

// Package server registers HTTP routes and composes auth middleware with handlers.
import (
	"fmt"
	"net/http"

	"ai-orchestration/internal/agent"
	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/config"
	"ai-orchestration/internal/handlers"
	"ai-orchestration/internal/middleware"
)

type Server struct {
	cfg      config.Config
	firebase *auth.FirebaseVerifier
	agent    agent.Client
}

// New wires configuration, Firebase verification, and the agent client into a Server.
func New(cfg config.Config, firebase *auth.FirebaseVerifier, agentClient agent.Client) *Server {
	return &Server{cfg: cfg, firebase: firebase, agent: agentClient}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	firebaseAuth := middleware.FirebaseAuth(middleware.NewFirebaseVerifierAdapter(s.firebase))
	slackAuth := middleware.SlackAuth(s.cfg.SlackSigningSecret)
	promptHandler := handlers.NewPromptHandler(s.agent)

	mux.HandleFunc("GET /healthz", handlers.Healthz)
	mux.Handle("POST /api/v1/prompt", firebaseAuth(promptHandler))
	mux.Handle("POST /api/v1/slack/events", slackAuth(http.HandlerFunc(handlers.SlackEvents)))

	return mux
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%s", s.cfg.Port)
	return http.ListenAndServe(addr, s.Handler())
}
