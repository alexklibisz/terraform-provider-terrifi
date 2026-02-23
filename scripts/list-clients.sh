#!/usr/bin/env bash
# List UniFi client devices (MAC address, device ID, name) for use with
# terraform import.
#
# Uses the same environment variables as the Terrifi provider:
#   UNIFI_API       - Controller URL (e.g. https://192.168.1.1:8443)
#   UNIFI_API_KEY   - API key (preferred) OR
#   UNIFI_USERNAME / UNIFI_PASSWORD - Credentials for session auth
#   UNIFI_SITE      - Site name (default: "default")
#   UNIFI_INSECURE  - Set to "true" to skip TLS verification
#
# Example:
#   ./scripts/list-clients.sh
#   ./scripts/list-clients.sh | grep -i printer

set -euo pipefail

: "${UNIFI_API:?Set UNIFI_API to the controller URL}"
: "${UNIFI_SITE:=default}"

CURL_OPTS=(-s)
if [[ "${UNIFI_INSECURE:-}" == "true" ]]; then
  CURL_OPTS+=(-k)
fi

COOKIE_JAR=$(mktemp)
trap 'rm -f "$COOKIE_JAR"' EXIT

# Detect UniFi OS vs legacy controller (UniFi OS returns 200 on GET /).
status=$(curl "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
  --max-redirs 0 "$UNIFI_API" 2>/dev/null || true)

if [[ "$status" == "200" ]]; then
  API_PATH="/proxy/network"
else
  API_PATH=""
fi

# Authenticate.
if [[ -n "${UNIFI_API_KEY:-}" ]]; then
  CURL_OPTS+=(-H "X-API-KEY: $UNIFI_API_KEY")
else
  : "${UNIFI_USERNAME:?Set UNIFI_API_KEY or both UNIFI_USERNAME and UNIFI_PASSWORD}"
  : "${UNIFI_PASSWORD:?Set UNIFI_API_KEY or both UNIFI_USERNAME and UNIFI_PASSWORD}"
  curl "${CURL_OPTS[@]}" -c "$COOKIE_JAR" \
    -X POST "${UNIFI_API}${API_PATH}/api/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$UNIFI_USERNAME\",\"password\":\"$UNIFI_PASSWORD\"}" \
    -o /dev/null
  CURL_OPTS+=(-b "$COOKIE_JAR")
fi

# Fetch all known clients from rest/user (these are the "configured" clients
# that have device IDs and can be managed by terrifi_client_device).
response=$(curl "${CURL_OPTS[@]}" \
  "${UNIFI_API}${API_PATH}/api/s/${UNIFI_SITE}/rest/user")

echo "$response" | jq -r '
  .data | sort_by(.mac) | .[] |
    (if .name then (.name | gsub("[- ]"; "_")) else "SET_NAME" end) as $tf_name |
    (.name // "unnamed") as $display_name |
    .mac as $mac | ._id as $id |

    # Build optional attributes, only including fields that are actively set.
    ([ (if .name              then "  name                = \"\(.name)\"" else empty end),
       (if .note and .note != "" then "  note                = \"\(.note)\"" else empty end),
       (if .use_fixedip       then "  fixed_ip            = \"\(.fixed_ip)\""  else empty end),
       (if .use_fixedip       then "  network_id          = \"\(.network_id)\"" else empty end),
       (if .local_dns_record_enabled then "  local_dns_record    = \"\(.local_dns_record)\"" else empty end),
       (if .virtual_network_override_enabled then "  network_override_id = \"\(.virtual_network_override_id)\"" else empty end),
       (if .blocked           then "  blocked             = true" else empty end)
    ] | join("\n")) as $attrs |

    "# \($display_name), \($mac), \($id)",
    "import {",
    "  to = terrifi_client_device.\($tf_name)",
    "  id = \"\($id)\"",
    "}",
    "resource \"terrifi_client_device\" \"\($tf_name)\" {",
    "  mac               = \"\($mac)\"",
    (if $attrs != "" then $attrs else empty end),
    "}",
    ""'
