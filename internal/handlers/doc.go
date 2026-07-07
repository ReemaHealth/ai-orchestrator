// Package handlers contains HTTP handlers for ai-orchestrator.
//
// POST /api/v1/prompt uses PromptHandler to stream SSE from agent.Client (Vertex or skeleton).
// GET/DELETE /api/v1/oauth/google/* handles Google OAuth consent for workspace datastore access.
// POST /api/v1/slack/events handles Slack URL verification and event callbacks.
// See docs/endpoints.md and docs/architecture.md.
package handlers
