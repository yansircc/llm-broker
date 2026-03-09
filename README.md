[![CI](https://github.com/yansircc/cc-relayer/actions/workflows/ci.yml/badge.svg)](https://github.com/yansircc/cc-relayer/actions/workflows/ci.yml)

# cc-relayer

High-performance Claude Code / Codex CLI API relay written in Go. Multi-account scheduling, identity isolation, and anti-detection for 3-7 OAuth accounts.

## Installation

### Download binary

Download the latest binary from [GitHub Releases](https://github.com/yansircc/cc-relayer/releases):

```bash
chmod +x cc-relayer-*
mv cc-relayer-* /usr/local/bin/cc-relayer
```

Platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.

### Build from source

```bash
git clone https://github.com/yansircc/cc-relayer.git
cd cc-relayer
cd web && npm ci && npm run build && cd ..
go build -o cc-relayer ./cmd/relay
```

Requires Go 1.24+ and Node 22+.

## Quick Start

### 1. Start the server

```bash
export ENCRYPTION_KEY=$(openssl rand -hex 16)
export API_TOKEN=$(openssl rand -hex 16)
echo "ENCRYPTION_KEY=$ENCRYPTION_KEY"
echo "API_TOKEN=$API_TOKEN"

./cc-relayer
```

Listens on `0.0.0.0:3000` by default. Data stored in SQLite at `./cc-relayer.db`.

### 2. Add an account

```bash
# Generate OAuth URL
curl -X POST https://YOUR_SERVER/admin/accounts/generate-auth-url \
  -H "x-api-key: YOUR_API_TOKEN"

# Open auth_url in browser, complete login, copy callback URL

# Exchange code
curl -X POST https://YOUR_SERVER/admin/accounts/exchange-code \
  -H "Content-Type: application/json" -H "x-api-key: YOUR_API_TOKEN" \
  -d '{"session_id": "...", "callback_url": "https://..."}'
```

Or use the WebUI at `https://YOUR_SERVER/`.

### 3. Configure Claude Code

```bash
export ANTHROPIC_BASE_URL=http://YOUR_SERVER
export ANTHROPIC_API_KEY=YOUR_API_TOKEN
claude
```

### 4. Configure Codex CLI

```bash
export OPENAI_BASE_URL=http://YOUR_SERVER/openai
export OPENAI_API_KEY=YOUR_API_TOKEN
codex
```

## WebUI

Built-in admin dashboard at `/` and `/dashboard` — manage accounts, users, view usage and events. Authenticate with your `API_TOKEN`.

## Admin API

All endpoints require `API_TOKEN` via `x-api-key` or `Authorization: Bearer`.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/admin/dashboard` | Dashboard overview |
| `GET` | `/admin/health` | Health check |
| `POST` | `/admin/accounts/generate-auth-url` | Generate OAuth URL |
| `POST` | `/admin/accounts/exchange-code` | Exchange auth code |
| `GET` | `/admin/accounts` | List accounts |
| `GET` | `/admin/accounts/{id}` | Account detail |
| `DELETE` | `/admin/accounts/{id}` | Delete account |
| `POST` | `/admin/users` | Create user |
| `GET` | `/admin/users` | List users |
| `GET` | `/admin/users/{id}` | User detail |

## Scheduling & Fault Tolerance

### Account Selection

```
Session binding → Pool selection (priority DESC, lastUsedAt ASC)
```

Availability filter: status, cooldown_until, and provider-specific gates derived from provider_state_json.

### Error Handling

| Code | Strategy | Default Cooldown |
|------|----------|-----------------|
| **200** | Update rate limit headers | — |
| **401** | Short cooldown + background refresh | 30s |
| **403** (ban) | Mark blocked, auto-recover | 10 min |
| **429** | Parse Retry-After | 60s fallback |
| **529** | Overload cooldown | 5 min |

### Session Integrity

- Binds multi-turn conversations to the same account via session UUID (TTL 24h)
- Refuses to switch accounts mid-conversation (returns 400)

## Environment Variables

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `ENCRYPTION_KEY` | Yes | — | AES encryption key for stored tokens |
| `API_TOKEN` | Yes | — | Admin authentication token |
| `DB_PATH` | No | `./cc-relayer.db` | SQLite database path |
| `HOST` | No | `0.0.0.0` | Listen address |
| `PORT` | No | `3000` | Listen port |
| `LOG_LEVEL` | No | `info` | Log level (debug/info/warn/error) |
| `REQUEST_TIMEOUT` | No | `300000` | Request timeout in ms |
| `MAX_RETRY_ACCOUNTS` | No | `2` | Max account switches per request |

## Project Structure

```
cmd/relay/              Entry point
internal/
  auth/                 Token authentication middleware
  config/               Environment config
  crypto/               AES encryption, key derivation
  domain/               Pure types (Account, User, RequestLog)
  events/               Ring buffer event bus
  identity/             Request masking, stainless binding
  oauth/                PKCE flows (Claude + Codex), token management
  pool/                 Account state machine, scheduling
  relay/                Stateless request forwarding
  server/               HTTP handlers, admin API, WebUI
  store/                SQLite persistence
  transport/            Per-account HTTP clients, proxy support
  ui/                   Embedded SvelteKit frontend
web/                    SvelteKit source (adapter-static)
```

## Development

```bash
cd web && npm ci && npm run build  # build frontend
go build ./...                      # compile
go test ./...                       # run tests
```

## License

MIT
