#!/usr/bin/env bash
#
# dns-watcher.sh — watch proxy_domain labels on egress cells,
# update proxy.host when the resolved IP changes.
#
# Usage:
#   BROKER_URL=http://localhost:3001 BROKER_COOKIE="session=xxx" ./dns-watcher.sh
#
# Env:
#   BROKER_URL     - broker admin base URL (required)
#   BROKER_COOKIE  - auth cookie for admin API (required)
#   DNS_SERVER     - DNS server for dig (default: 223.5.5.5, bypasses fake-ip)
#   DRY_RUN        - set to 1 to print changes without applying
#
set -euo pipefail

: "${BROKER_URL:?BROKER_URL is required}"
: "${BROKER_COOKIE:?BROKER_COOKIE is required}"
DNS_SERVER="${DNS_SERVER:-223.5.5.5}"
DRY_RUN="${DRY_RUN:-0}"

log() { echo "$(date '+%Y-%m-%d %H:%M:%S') $*"; }

# Fetch all cells
cells_json=$(curl -s -b "$BROKER_COOKIE" "${BROKER_URL}/admin/egress/cells")

# Extract cells that have a proxy_domain label
domains=$(echo "$cells_json" | python3 -c "
import json, sys
cells = json.load(sys.stdin)
for c in cells:
    domain = (c.get('labels') or {}).get('proxy_domain', '')
    if domain and c.get('proxy'):
        p = c['proxy']
        print(f\"{c['id']}\t{domain}\t{p.get('host','')}\t{p.get('port',0)}\t{p.get('type','socks5')}\t{p.get('username','')}\t{p.get('password','')}\t{c['name']}\t{c.get('status','active')}\")
" 2>/dev/null) || true

if [ -z "$domains" ]; then
    log "no cells with proxy_domain label"
    exit 0
fi

updated=0
while IFS=$'\t' read -r cell_id domain current_ip port proxy_type username password name status; do
    # Resolve domain via explicit DNS server (bypass mihomo fake-ip)
    resolved_ip=$(dig +short "$domain" "@${DNS_SERVER}" A 2>/dev/null | grep -E '^[0-9]+\.' | head -1)

    if [ -z "$resolved_ip" ]; then
        log "WARN $cell_id: failed to resolve $domain"
        continue
    fi

    if [ "$resolved_ip" = "$current_ip" ]; then
        continue
    fi

    log "CHANGE $cell_id: $domain resolved to $resolved_ip (was $current_ip)"

    if [ "$DRY_RUN" = "1" ]; then
        log "DRY_RUN: would update $cell_id proxy.host to $resolved_ip"
        continue
    fi

    # Build proxy JSON — omit empty username/password
    proxy_json="{\"type\":\"$proxy_type\",\"host\":\"$resolved_ip\",\"port\":$port"
    if [ -n "$username" ]; then
        proxy_json="$proxy_json,\"username\":\"$username\",\"password\":\"$password\""
    fi
    proxy_json="$proxy_json}"

    # Fetch current labels to preserve them
    labels_json=$(echo "$cells_json" | python3 -c "
import json, sys
cells = json.load(sys.stdin)
for c in cells:
    if c['id'] == '$cell_id':
        print(json.dumps(c.get('labels') or {}))
        break
" 2>/dev/null)

    # Update cell via upsert API
    resp=$(curl -s -w "\n%{http_code}" -b "$BROKER_COOKIE" \
        -X POST "${BROKER_URL}/admin/egress/cells" \
        -H "Content-Type: application/json" \
        -d "{\"id\":\"$cell_id\",\"name\":\"$name\",\"status\":\"$status\",\"proxy\":$proxy_json,\"labels\":$labels_json}")

    http_code=$(echo "$resp" | tail -1)
    if [ "$http_code" = "200" ]; then
        log "OK $cell_id: updated proxy.host to $resolved_ip"
        updated=$((updated + 1))
    else
        body=$(echo "$resp" | sed '$d')
        log "ERROR $cell_id: HTTP $http_code — $body"
    fi
done <<< "$domains"

if [ "$updated" -gt 0 ]; then
    log "updated $updated cell(s)"
fi
