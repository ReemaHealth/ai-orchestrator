# ai-orchestrator

HTTP API for project Lasso. Cryptographically verifies callers (Firebase JWT for the web app, Slack HMAC for Events API), streams prompt responses from Vertex AI Agent Engine (or a local skeleton), and handles Slack events.

## Documentation

| Doc | Contents |
|-----|----------|
| [docs/architecture.md](docs/architecture.md) | Package layout, request flows, auth design, file reference |
| [docs/endpoints.md](docs/endpoints.md) | HTTP routes, request/response shapes, status codes |
| [docs/testing.md](docs/testing.md) | Unit tests, local manual testing, Postman/curl |

## Quick start

```bash
cp .env.example .env   # edit Firebase, Slack, and optional Vertex settings
go run .
```

Default: `AGENT_ENABLED=false` uses skeleton agent chunks locally. Set `AGENT_ENABLED=true` and Vertex env vars to call Reasoning Engine (requires ADC — see [docs/testing.md](docs/testing.md)).

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

- **Web:** `reemaUserId` and `email` from verified Firebase JWT claims — never from headers or JSON body
- **Slack:** request authenticity via HMAC — Slack user IDs are not Reema user IDs until an explicit mapping exists
- **Vertex:** orchestrator calls Agent Engine with Cloud Run SA (ADC); verified `email` is passed as `user_id` for enterprise ACL context — the raw Firebase JWT is not forwarded

## Project layout

```
main.go                 Entrypoint: config, Firebase verifier, HTTP server
internal/
  auth/                 Crypto verification utilities + request context
  agent/                Vertex Reasoning Engine + skeleton agent clients
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
- [golang.org/x/oauth2](https://golang.org/x/oauth2) — Application Default Credentials for Vertex API calls
