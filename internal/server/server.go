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
	"ai-orchestration/internal/useroauth"
)

type Server struct {
	cfg           config.Config
	firebase      *auth.FirebaseVerifier
	agentClient   agent.Client
	tokenProvider useroauth.TokenProvider
	oauthHandler  *handlers.GoogleOAuthHandler
}

// New builds the HTTP server with routes, auth middleware, and handlers.
func New(
	cfg config.Config,
	firebase *auth.FirebaseVerifier,
	agentClient agent.Client,
	tokenProvider useroauth.TokenProvider,
	oauthHandler *handlers.GoogleOAuthHandler,
) *Server {
	return &Server{
		cfg:           cfg,
		firebase:      firebase,
		agentClient:   agentClient,
		tokenProvider: tokenProvider,
		oauthHandler:  oauthHandler,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	firebaseAuth := middleware.FirebaseAuth(middleware.NewFirebaseVerifierAdapter(s.firebase))
	slackAuth := middleware.SlackAuth(s.cfg.SlackSigningSecret)
	promptHandler := handlers.NewPromptHandler(s.agentClient, s.tokenProvider)
	oauthStatusHandler := handlers.NewOAuthStatusHandler(s.tokenProvider)

	mux.HandleFunc("GET /healthz", handlers.Healthz)
	mux.Handle("POST /api/v1/prompt", firebaseAuth(promptHandler))
	mux.Handle("GET /api/v1/oauth/google/status", firebaseAuth(oauthStatusHandler))
	mux.Handle("POST /api/v1/slack/events", slackAuth(http.HandlerFunc(handlers.SlackEvents)))

	if s.oauthHandler != nil {
		mux.Handle("GET /api/v1/oauth/google/start", firebaseAuth(http.HandlerFunc(s.oauthHandler.Start)))
		mux.HandleFunc("GET /api/v1/oauth/google/callback", s.oauthHandler.Callback)
		mux.Handle("DELETE /api/v1/oauth/google/revoke", firebaseAuth(http.HandlerFunc(s.oauthHandler.Revoke)))
	}

	return mux
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%s", s.cfg.Port)
	return http.ListenAndServe(addr, s.Handler())
}
