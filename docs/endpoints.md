# Endpoints

This document describes HTTP endpoints for ai-orchestrator: current auth-gated behavior and intended future behavior.

## Authentication overview

| Path | Verification | Trusted identity |
|------|--------------|------------------|
| `POST /api/v1/prompt` | Firebase JWT (JWKS + iss/aud/exp/provider + `reemaUserId` claim) | `reemaUserId` from verified JWT |
| `POST /api/v1/slack/events` | Slack signing secret (HMAC + timestamp replay window) | Slack origin only (Reema user mapping is future work) |

HTTP status contract for protected routes:

- **401** — no auth attempt (missing credentials)
- **403** — credentials present but cryptographic verification failed
- **200** — verification passed

Never trust `reemaUserId`, email, or user ids from request body or custom headers.

---

## `GET /healthz`

GCP liveness probe. No authentication.

| | |
|---|---|
| **Method** | GET |
| **Auth** | None |
| **Success** | `200` `{"status":"healthy"}` |
| **Errors** | `405` for non-GET methods |

---

## `POST /api/v1/prompt`

Web app prompt endpoint. Firebase JWT required.

### Request (now)

```
Authorization: Bearer <firebase-id-token>
```

JWT must be issued by the configured Identity Platform project (`FIREBASE_PROJECT_ID`) with:

- `iss` = `https://securetoken.google.com/<project-id>`
- `aud` = `<project-id>`
- `firebase.sign_in_provider` = `google.com` (or `FIREBASE_SIGN_IN_PROVIDER`)
- `reemaUserId` custom claim (UUID)

### Response (now)

Auth passes → `200` `{"status":"ok"}`

### Response (future)

Auth passes → SSE stream:

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

data: <agent token chunk>

...
```

Skeleton implementation preserved in `handlers.PromptSSEFuture`:

- Streams 5 placeholder chunks (`Skeleton token chunk N`)
- Flushes after each chunk with 500ms delay
- Uses verified `reemaUserId` from auth context for agent attribution (when wired)

Reference: original `sseHandler` in the project skeleton.

---

## `POST /api/v1/slack/events`

Slack Events API webhook. Slack HMAC required.

### Request (now)

```
X-Slack-Signature: v0=<hmac>
X-Slack-Request-Timestamp: <unix>
```

Body is the raw Slack JSON payload. Signature is verified before the handler runs.

### Response (now)

Auth passes → `200` `{"status":"ok"}`

### Response (future)

1. **URL verification** — when `type` is `url_verification`, return `{"challenge":"<challenge>"}` after signature verify
2. **Events** — immediately ack within Slack's ~3s window: `200` `{"status":"accepted"}`
3. **Background processing** — spawn worker to invoke ADK agent with parsed event
4. **User mapping** — map `event.user` + `team_id` → `reemaUserId` before agent calls (separate from crypto verify)

Skeleton implementation preserved in `handlers.SlackEventsFuture`.

Reference: original `slackHandler` in the project skeleton.

---

## Environment variables

See [`.env.example`](../.env.example). Copy to `.env` for local development (`.env` is gitignored).

| Variable | Purpose |
|----------|---------|
| `PORT` | HTTP listen port (default `8080`) |
| `FIREBASE_PROJECT_ID` | Firebase / Identity Platform project id |
| `FIREBASE_SIGN_IN_PROVIDER` | Required sign-in provider (default `google.com`) |
| `SLACK_SIGNING_SECRET` | Slack app signing secret |

### Loading `.env`

| Runtime | How config is loaded |
|---------|----------------------|
| `go run .` | `godotenv` reads `.env` from the project root |
| Docker | `.env` is **not** in the image — pass it at runtime |

```bash
# Recommended for local Docker
docker compose up --build

# Or with an existing image
docker run -p 8080:8080 --env-file .env orchestrator-local
```

On Cloud Run, set the same variables in the service config or Secret Manager.
