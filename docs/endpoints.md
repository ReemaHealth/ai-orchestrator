# Endpoints

HTTP API reference for ai-orchestrator. See also [architecture.md](architecture.md) and [testing.md](testing.md).

## Authentication overview

| Path | Verification | Trusted identity |
|------|--------------|------------------|
| `POST /api/v1/prompt` | Firebase JWT (JWKS + iss/aud/exp/provider + `reemaUserId` + `email` claims) | `reemaUserId` and `email` from verified JWT |
| `POST /api/v1/slack/events` | Slack signing secret (HMAC + timestamp replay window) | Slack origin only (Reema user mapping is future work) |

HTTP status contract for protected routes:

- **401** — no auth attempt (missing credentials)
- **403** — credentials present but cryptographic verification failed
- **400** — auth passed but request body JSON is invalid (handlers only)
- **502** — agent call failed before or during stream (handlers only)
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

{"prompt":"user message"}
```

The `prompt` field is **required**.

JWT must be issued by the configured Identity Platform project (`FIREBASE_PROJECT_ID`) with:

- `iss` = `https://securetoken.google.com/<project-id>`
- `aud` = `<project-id>`
- `firebase.sign_in_provider` = `google.com` (or `FIREBASE_SIGN_IN_PROVIDER`)
- `reemaUserId` custom claim (UUID)
- `email` claim (required for `google.com` sign-in; passed to Agent Engine as `user_id`)

### Response

Auth passes → SSE stream from Vertex Agent Engine (or skeleton when `AGENT_ENABLED=false`):

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

event: meta
data: {"reemaUserId":"<verified-uuid>"}

data: <agent chunk 1>

data: <agent chunk 2>
...
```

On agent failure after streaming starts:

```
event: error
data: {"error":"agent error"}
```

| Status | When |
|--------|------|
| 400 | Missing or empty `prompt` |
| 502 | Agent unavailable before stream starts |

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
| `AGENT_ENABLED` | `true` to call Vertex Reasoning Engine; `false` for skeleton chunks (default `false`) |
| `GCP_PROJECT` | GCP project hosting the reasoning engine (required when `AGENT_ENABLED=true`) |
| `GCP_LOCATION` | Vertex region, e.g. `us-central1` |
| `REASONING_ENGINE_ID` | Reasoning engine resource id |
| `AGENT_CLASS_METHOD` | Reasoning engine class method (default `stream_query`) |

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
