#!/usr/bin/env bash
# Sign a Slack-style request and POST it to the local orchestrator.
# Usage: ./scripts/sign-slack-request.sh [body] [url]
#
# Reads SLACK_SIGNING_SECRET from .env in the project root.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BODY="${1:-'{\"type\":\"event_callback\"}'}"
URL="${2:-http://localhost:8080/api/v1/slack/events}"

if [[ "$BODY" == \'*\' ]]; then
  BODY="${BODY:1:-1}"
fi

if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

if [[ -z "${SLACK_SIGNING_SECRET:-}" ]]; then
  echo "SLACK_SIGNING_SECRET is not set. Add it to $ROOT/.env" >&2
  exit 1
fi

TS=$(date +%s)
SIG="v0=$(printf 'v0:%s:%s' "$TS" "$BODY" | openssl dgst -sha256 -hmac "$SLACK_SIGNING_SECRET" | awk '{print $2}')"

echo "Body:      $BODY"
echo "Timestamp: $TS"
echo "Signature: $SIG"
echo "URL:       $URL"
echo

curl -i -X POST "$URL" \
  -H "Content-Type: application/json" \
  -H "X-Slack-Request-Timestamp: $TS" \
  -H "X-Slack-Signature: $SIG" \
  --data-binary "$BODY"
