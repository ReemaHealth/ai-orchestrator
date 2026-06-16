package server

import (
	"fmt"
	"net/http"

	"ai-orchestration/internal/auth"
	"ai-orchestration/internal/config"
	"ai-orchestration/internal/handlers"
	"ai-orchestration/internal/middleware"
)

type Server struct {
	cfg      config.Config
	firebase *auth.FirebaseVerifier
}

func New(cfg config.Config, firebase *auth.FirebaseVerifier) *Server {
	return &Server{cfg: cfg, firebase: firebase}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	firebaseAuth := middleware.FirebaseAuth(middleware.NewFirebaseVerifierAdapter(s.firebase))
	slackAuth := middleware.SlackAuth(s.cfg.SlackSigningSecret)

	mux.HandleFunc("GET /healthz", handlers.Healthz)
	mux.Handle("POST /api/v1/prompt", firebaseAuth(http.HandlerFunc(handlers.Prompt)))
	mux.Handle("POST /api/v1/slack/events", slackAuth(http.HandlerFunc(handlers.SlackEvents)))

	return mux
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%s", s.cfg.Port)
	return http.ListenAndServe(addr, s.Handler())
}
