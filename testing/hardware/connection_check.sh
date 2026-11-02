#!/usr/bin/env bash
#
# connection_check.sh — Verify connectivity to a UniFi controller using
# the same env vars the Terrifi provider expects.
#
# Required env vars:
#   UNIFI_API        Base URL (e.g. https://192.168.1.1:8443)
#
# Auth (one of):
#   UNIFI_API_KEY                          API-key auth
#   UNIFI_USERNAME + UNIFI_PASSWORD        Session auth
#
# Optional:
#   UNIFI_INSECURE   "true" to skip TLS verification (default: false)
#   UNIFI_SITE       Site name (default: "default")

set -euo pipefail

SITE="${UNIFI_SITE:-default}"

CURL_OPTS=(-s -S --max-time 10)
if [[ "${UNIFI_INSECURE:-false}" == "true" ]]; then
  CURL_OPTS+=(-k)
fi

# ── Preflight ────────────────────────────────────────────────────────
if [[ -z "${UNIFI_API:-}" ]]; then
  echo "ERROR: UNIFI_API is not set." >&2
  exit 1
fi

if [[ -z "${UNIFI_API_KEY:-}" && ( -z "${UNIFI_USERNAME:-}" || -z "${UNIFI_PASSWORD:-}" ) ]]; then
  echo "ERROR: Set UNIFI_API_KEY, or both UNIFI_USERNAME and UNIFI_PASSWORD." >&2
  exit 1
fi

BASE_URL="${UNIFI_API%/}"

echo "Checking connection to ${BASE_URL} ..."

# ── Detect controller type (classic vs UniFi OS) ────────────────────
# UniFi OS controllers expose /api/users/self.  Classic controllers do not,
# but both respond on the base URL.  We try the UniFi OS auth path first;
# if it fails we fall back to the classic path.

COOKIE_JAR=$(mktemp)
trap 'rm -f "$COOKIE_JAR"' EXIT

api_prefix=""  # will be set after detection

authenticate() {
  if [[ -n "${UNIFI_API_KEY:-}" ]]; then
    # API-key auth — no login needed, just set the header for later calls.
    AUTH_HEADER="X-API-KEY: ${UNIFI_API_KEY}"
    echo "Using API-key authentication."
  else
    echo "Logging in as ${UNIFI_USERNAME} ..."

    # Try UniFi OS endpoint first.
    HTTP_CODE=$(curl "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
      -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
      -H "Content-Type: application/json" \
      -d "{\"username\":\"${UNIFI_USERNAME}\",\"password\":\"${UNIFI_PASSWORD}\"}" \
      "${BASE_URL}/api/auth/login" 2>/dev/null || echo "000")

    if [[ "$HTTP_CODE" == "200" ]]; then
      api_prefix="/proxy/network"
      echo "Authenticated (UniFi OS controller)."
      return 0
    fi

    # Fall back to classic controller endpoint.
    HTTP_CODE=$(curl "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
      -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
      -H "Content-Type: application/json" \
      -d "{\"username\":\"${UNIFI_USERNAME}\",\"password\":\"${UNIFI_PASSWORD}\"}" \
      "${BASE_URL}/api/login" 2>/dev/null || echo "000")

    if [[ "$HTTP_CODE" == "200" ]]; then
      api_prefix=""
      echo "Authenticated (classic controller)."
      return 0
    fi

    echo "ERROR: Login failed (HTTP ${HTTP_CODE})." >&2
    return 1
  fi
}

# ── Authenticate ─────────────────────────────────────────────────────
authenticate

# ── Build curl args for authenticated requests ───────────────────────
auth_curl() {
  if [[ -n "${AUTH_HEADER:-}" ]]; then
    curl "${CURL_OPTS[@]}" -H "$AUTH_HEADER" "$@"
  else
    curl "${CURL_OPTS[@]}" -b "$COOKIE_JAR" "$@"
  fi
}

# When using an API key we still need to detect classic vs UniFi OS.
if [[ -n "${UNIFI_API_KEY:-}" ]]; then
  HTTP_CODE=$(auth_curl -o /dev/null -w "%{http_code}" \
    "${BASE_URL}/proxy/network/api/s/${SITE}/self" 2>/dev/null || echo "000")
  if [[ "$HTTP_CODE" == "200" ]]; then
    api_prefix="/proxy/network"
  else
    api_prefix=""
  fi
fi

# ── Test API call: list self ─────────────────────────────────────────
echo "Fetching site '${SITE}' self info ..."

RESPONSE=$(auth_curl "${BASE_URL}${api_prefix}/api/s/${SITE}/self" 2>&1) || {
  echo "ERROR: API request failed." >&2
  echo "$RESPONSE" >&2
  exit 1
}

# Validate we got JSON back with data.
if echo "$RESPONSE" | jq empty 2>/dev/null; then
  NAME=$(echo "$RESPONSE" | jq -r '.data[0].name // "unknown"')
  IS_SUPER=$(echo "$RESPONSE" | jq -r '.data[0].is_super // false')
  echo "OK — logged in as '${NAME}' (super=${IS_SUPER}), site '${SITE}'."
else
  echo "ERROR: Unexpected non-JSON response:" >&2
  echo "$RESPONSE" | head -5 >&2
  exit 1
fi
