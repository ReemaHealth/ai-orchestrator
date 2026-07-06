# Endpoints

HTTP API reference for ai-orchestrator. See also [architecture.md](architecture.md) and [testing.md](testing.md).

## Authentication overview

| Path | Verification | Trusted identity |
|------|--------------|------------------|
| `POST /api/v1/prompt` | Firebase JWT (JWKS + iss/aud/exp/provider + `reemaUserId` + `email` claims) | `reemaUserId` and `email` from verified JWT |
| `GET /api/v1/oauth/google/start` | Firebase JWT | `reemaUserId` and `email` from verified JWT |
| `DELETE /api/v1/oauth/google/revoke` | Firebase JWT | `reemaUserId` from verified JWT |
| `GET /api/v1/oauth/google/callback` | HMAC-signed OAuth `state` (no JWT) | `reemaUserId` and `email` embedded in signed state |
| `POST /api/v1/slack/events` | Slack signing secret (HMAC + timestamp replay window) | Slack origin only (Reema user mapping is future work) |

HTTP status contract for protected routes:

- **401** — no auth attempt (missing credentials)
- **403** — credentials present but cryptographic verification failed
- **400** — auth passed but request body JSON is invalid (handlers only)
- **428** — Google OAuth consent required before workspace datastore queries (handlers only)
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

Optional override (local dev / Postman):

```
X-GCP-Access-Token: <ge-oauth-token-with-drive-scopes>
```

Or pass the GE OAuth token in the JSON body as `gcpAccessToken` (header takes precedence).

When `AGENT_ENABLED=true` and Google OAuth is configured (`GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`, `GOOGLE_OAUTH_REDIRECT_URI`), the orchestrator resolves the user-scoped GCP access token server-side from a stored refresh grant. Complete one-time consent via `GET /api/v1/oauth/google/start` first.

When OAuth is not configured, pass `X-GCP-Access-Token` or `gcpAccessToken` (legacy / curl testing).

The user OAuth token is stored in ADK session state under `fetch-agent-auth` (configurable via `AGENT_GE_AUTH_STATE_KEY`) so workspace datastores can search as the end user.

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
data: {"reemaUserId":"<verified-uuid>","email":"<verified-email>"}

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
| 428 | Google OAuth consent required (`{"error":"google_oauth_required","authorizePath":"/api/v1/oauth/google/start"}`) |
| 502 | Agent unavailable before stream starts |

---

## Google OAuth — workspace datastore access

One-time consent flow when `GOOGLE_OAUTH_*` env vars are set. Stores a refresh token keyed by verified `reemaUserId`.

### `GET /api/v1/oauth/google/start`

| | |
|---|---|
| **Auth** | Firebase JWT (`Authorization: Bearer …`) |
| **Success** | `302` redirect to Google OAuth consent |
| **Errors** | `401` no JWT; `405` non-GET |

Open in a browser (or follow redirects) while authenticated. Requires `prompt=consent` on first grant to obtain a refresh token.

### `GET /api/v1/oauth/google/callback`

| | |
|---|---|
| **Auth** | HMAC-signed `state` query param (binds to `reemaUserId` + `email`) |
| **Success** | `302` redirect to `GOOGLE_OAUTH_SUCCESS_REDIRECT` (default `/`) |
| **Errors** | `400` missing/invalid state or no refresh token; `502` token exchange failed |

Registered as the OAuth redirect URI in Google Cloud Console (e.g. `http://localhost:8080/api/v1/oauth/google/callback`).

### `DELETE /api/v1/oauth/google/revoke`

| | |
|---|---|
| **Auth** | Firebase JWT |
| **Success** | `204 No Content` |
| **Errors** | `401` no JWT; `405` non-DELETE |

Deletes the stored refresh token for the authenticated user (disconnect workspace access).

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
| `AGENT_CLASS_METHOD` | Reasoning engine class method (default `async_stream_query` for ADK agents) |
| `AGENT_GE_AUTH_STATE_KEY` | ADK session state key for user OAuth token (default `fetch-agent-auth`) |
| `USER_OAUTH_MODE` | `auto` (default), `required`, `client_only`, or `disabled` |
| `GOOGLE_OAUTH_CLIENT_ID` | OAuth 2.0 web client id for user consent |
| `GOOGLE_OAUTH_CLIENT_SECRET` | OAuth client secret (Secret Manager in prod) |
| `GOOGLE_OAUTH_REDIRECT_URI` | Callback URL (must match Google Cloud Console) |
| `GOOGLE_OAUTH_SUCCESS_REDIRECT` | Browser redirect after successful consent |
| `GOOGLE_OAUTH_STATE_SECRET` | HMAC secret for OAuth state (defaults to client secret or Slack secret) |
| `GOOGLE_OAUTH_SCOPES` | Comma-separated scopes (default `https://www.googleapis.com/auth/cloud-platform`) |
| `GOOGLE_OAUTH_ALLOWED_DOMAINS` | Comma-separated email domains allowed for user OAuth (optional) |

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
