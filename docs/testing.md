# Testing

## Running tests

```bash
go test ./...
```

All tests are unit/integration style with `httptest` — no live Firebase or Slack calls in CI.

| Package | Test file | Focus |
|---------|-----------|-------|
| `internal/auth` | `firebase_test.go` | JWT verify with mock JWKS server |
| `internal/auth` | `slack_test.go` | HMAC verify, replay window, missing headers |
| `internal/middleware` | `auth_test.go` | Middleware → HTTP status mapping |
| `internal/handlers` | `handlers_test.go` | SSE output, Slack handler branches |

---

## `internal/auth` tests

### `firebase_test.go`

#### `TestFirebaseVerifierVerifyToken`

- Spins up a local `httptest` server returning a synthetic JWKS (RSA key).
- Signs a JWT with matching `iss`, `aud`, `exp`, `firebase.sign_in_provider`, and `reemaUserId`.
- Asserts `VerifyToken` returns the expected `Principal.ReemaUserID`.

Uses `NewFirebaseVerifierWithJWKS` to avoid calling Google in tests.

#### `TestExtractBearerToken`

| Case | Expected |
|------|----------|
| `Bearer abc.def.ghi` | token `abc.def.ghi`, no error |
| empty header | `ErrNoCredentials` |
| `Token abc` (non-Bearer) | `ErrNoCredentials` |

#### `signTestFirebaseToken` (helper)

Builds RS256 JWTs for tests with configurable project ID and `reemaUserId`.

### `slack_test.go`

#### `TestVerifySlackRequest`

Valid signature for current timestamp → no error (`VerifySlackRequestWithNow` with fixed clock).

#### `TestVerifySlackRequestMissingHeaders`

Empty timestamp → `ErrNoCredentials`.

#### `TestVerifySlackRequestBadSignature`

Wrong hex signature → `ErrVerificationFailed`.

#### `TestVerifySlackRequestStaleTimestamp`

Timestamp 10 minutes in the past → `ErrVerificationFailed` (replay protection).

Uses `auth.SignSlackRequest` helper to generate valid signatures in tests.

---

## `internal/middleware` tests

### `auth_test.go`

#### `TestFirebaseAuthStatuses`

Uses `stubFirebaseVerifier` (does not call real Firebase). Stub handler returns 200 on success.

| Case | Expected status |
|------|-----------------|
| missing `Authorization` | 401 |
| `Bearer bad.token` (verifier returns `ErrVerificationFailed`) | 403 |
| valid Bearer (verifier returns principal) | 200 |

#### `TestSlackAuthStatuses`

Uses a noop inner handler (auth-only tests, not full `SlackEvents`).

| Case | Expected status |
|------|-----------------|
| missing Slack headers | 401 |
| wrong `X-Slack-Signature` | 403 |
| valid HMAC for body + secret | 200 |

#### `TestWriteAuthError`

| Error | Status |
|-------|--------|
| `ErrNoCredentials` | 401 |
| `ErrVerificationFailed` | 403 |
| unknown error | 403 |

---

## `internal/handlers` tests

### `handlers_test.go`

#### `TestPromptStreamsSSE`

Calls `StreamPromptForTest` with **zero chunk delay** (avoids 2.5s sleep in CI).

Asserts:
- `Content-Type: text/event-stream`
- body contains `event: meta` and verified `reemaUserId`
- body contains final skeleton chunk `5`

#### `TestPromptRequiresPrincipal`

Calls `Prompt` without `auth.WithPrincipal` on context → **401**.

#### `TestStreamPromptNoDelay`

Smoke test for first SSE data line.

#### `TestSlackEventsURLVerification`

Context body `{"type":"url_verification","challenge":"abc123"}` → **200** with `"challenge":"abc123"` in response.

#### `TestSlackEventsAckAndProcessesAsync`

Context body `event_callback` payload → **200** `{"status":"accepted"}`. Sleeps 50ms to allow background goroutine to run (no assertion on logs).

---

## Manual / local testing

### Prerequisites

1. Copy `.env.example` → `.env` and fill in values.
2. **Restart server** after `.env` changes.
3. Use HTTPS in production; local HTTP is fine for dev.

### Health check

```bash
curl -i http://localhost:8080/healthz
```

### Slack — signed request script

```bash
chmod +x scripts/sign-slack-request.sh

# event_callback
./scripts/sign-slack-request.sh '{"type":"event_callback","team_id":"T123","event":{"type":"message","user":"U123"}}'

# url_verification
./scripts/sign-slack-request.sh '{"type":"url_verification","challenge":"test-challenge-123"}'
```

Script reads `SLACK_SIGNING_SECRET` from `.env` and computes the HMAC.

### Slack — Postman

1. Create environment variable `signing_secret` = same value as `SLACK_SIGNING_SECRET` in `.env`.
2. Body → raw → JSON (exact bytes matter for signing).
3. Pre-request script using `crypto.subtle` (see team notes) sets `X-Slack-Request-Timestamp` and `X-Slack-Signature`.

**Common 403 causes:**
- Server not restarted after `.env` change (stale secret in memory)
- Body pretty-printed differently than signed string
- Postman `signing_secret` ≠ server `SLACK_SIGNING_SECRET`

### Firebase — `POST /api/v1/prompt`

Requires a real Firebase ID token from your Identity Platform project:

```bash
curl -N -X POST http://localhost:8080/api/v1/prompt \
  -H "Authorization: Bearer <firebase-id-token>" \
  -H "Content-Type: application/json" \
  -d '{"prompt":"hello"}'
```

`-N` disables curl buffering so SSE chunks appear as they arrive.

Token must include `reemaUserId` custom claim from your `beforeSignIn` hook.

| Result | Meaning |
|--------|---------|
| 401 | missing / malformed Bearer header |
| 403 | invalid, expired, or wrong-project JWT |
| 400 | invalid JSON body (after auth passes) |
| 200 + `text/event-stream` | success |

---

## Test helpers (production code)

| Symbol | Package | Purpose |
|--------|---------|---------|
| `SignSlackRequest` | `auth` | Build valid Slack HMAC for tests/scripts |
| `VerifySlackRequestWithNow` | `auth` | Slack verify with injectable clock |
| `NewFirebaseVerifierWithJWKS` | `auth` | Firebase verify with mock JWKS |
| `StreamPromptForTest` | `handlers` | SSE stream with configurable delay |

These exist to keep crypto and handler logic testable without network calls.
