# ai-orchestrator

HTTP API for project Lasso. Cryptographically verifies callers (Firebase JWT for the web app, Slack HMAC for Events API), then orchestrates prompt streaming and Slack event handling. Agent (ADK/VDS) and Reema IAM integration are planned follow-ups.

## Documentation

| Doc | Contents |
|-----|----------|
| [docs/architecture.md](docs/architecture.md) | Package layout, request flows, auth design, file reference |
| [docs/endpoints.md](docs/endpoints.md) | HTTP routes, request/response shapes, status codes |
| [docs/testing.md](docs/testing.md) | Unit tests, local manual testing, Postman/curl |

## Quick start

```bash
cp .env.example .env   # edit with your Firebase project + Slack signing secret
go run .
```

Restart the server after any `.env` change — config loads once at startup.

```bash
go test ./...
```

### Docker

```bash
docker compose up --build
```

Plain `docker run` does not load `.env` automatically:

```bash
docker run -p 8080:8080 --env-file .env orchestrator-local
```

## Routes

| Method | Path | Auth |
|--------|------|------|
| GET | `/healthz` | None |
| POST | `/api/v1/prompt` | Firebase JWT |
| POST | `/api/v1/slack/events` | Slack signing secret |

## Core rule

Identity is only trusted when cryptographically verified:

- **Web:** `reemaUserId` from a verified Firebase JWT claim — never from headers or JSON body
- **Slack:** request authenticity via HMAC — Slack user IDs are not Reema user IDs until an explicit mapping exists

## Project layout

```
main.go                 Entrypoint: config, Firebase verifier, HTTP server
internal/
  auth/                 Crypto verification utilities + request context
  config/               Environment / .env loading
  handlers/             Route handlers (health, prompt SSE, Slack events)
  middleware/           Auth middleware (maps verify errors → 401/403)
  server/                 ServeMux route registration
docs/                   Architecture, endpoints, testing guides
scripts/                Local dev helpers (Slack request signing)
```

## Dependencies

- [github.com/MicahParks/keyfunc/v3](https://github.com/MicahParks/keyfunc) — Firebase JWKS
- [github.com/golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) — JWT parsing
- [github.com/google/uuid](https://github.com/google/uuid) — `reemaUserId` validation
- [github.com/joho/godotenv](https://github.com/joho/godotenv) — local `.env` loading
