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
- **400** — auth passed but request body JSON is invalid (handlers only)
- **200** — success (SSE stream or JSON ack/challenge)

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

### Request

```
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{"prompt":"optional user message"}
```

The `prompt` field is optional for now; the skeleton stream does not yet call the ADK agent.

JWT must be issued by the configured Identity Platform project (`FIREBASE_PROJECT_ID`) with:

- `iss` = `https://securetoken.google.com/<project-id>`
- `aud` = `<project-id>`
- `firebase.sign_in_provider` = `google.com` (or `FIREBASE_SIGN_IN_PROVIDER`)
- `reemaUserId` custom claim (UUID)

### Response

Auth passes → SSE stream:

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

event: meta
data: {"reemaUserId":"<verified-uuid>"}

data: Skeleton token chunk 1

data: Skeleton token chunk 2
...
```

- First event carries verified `reemaUserId` from the JWT (for client attribution / debugging)
- Five placeholder chunks follow, flushed every 500ms
- **Future:** replace skeleton chunks with real ADK/VDS agent token stream keyed off `reemaUserId` and `prompt`

---

## `POST /api/v1/slack/events`

Slack Events API webhook. Slack HMAC required.

### Request

```
X-Slack-Signature: v0=<hmac>
X-Slack-Request-Timestamp: <unix>
Content-Type: application/json
```

Body is the raw Slack JSON payload. Signature is verified before the handler runs; the verified body is passed to the handler via request context.

### Response

| Payload `type` | Response | Notes |
|----------------|----------|-------|
| `url_verification` | `200` `{"challenge":"<challenge>"}` | Slack app install handshake |
| `event_callback` | `200` `{"status":"accepted"}` | Ack within ~3s; event processed in background |
| other | `200` `{"status":"accepted"}` | Safe default ack |

Background processing (`processSlackEvent`) currently logs the event. **Future:**

- Map `event.user` + `team_id` → `reemaUserId`
- Invoke ADK agent asynchronously

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
