# Suppervisor

**Dinner, handled.**

Suppervisor is a standalone, local-first dinner assistant for Alan and Partner. It accepts a natural-language brief from Telegram or the web, turns it into explicit constraints, asks a provider-neutral local Ollama adapter for candidates, rejects candidates that break hard rules, and ranks the rest deterministically. The same app carries a meal through shopping, prep, cooking, individual ratings, and reversible preference learning.

## What iteration one does

- Keeps separate Alan and Partner profiles, dislikes, winners, and learned preferences.
- Structures natural requests such as “something cosy under 35 minutes, two lunches, no mushrooms”.
- Validates generated recipe shape with one repair retry.
- Enforces portions, lunch leftovers, reheating, time, cost, equipment, ingredients, and diner dislikes as hard constraints.
- Scores preference fit, speed, price, leftovers, available ingredients, and recent-meal novelty deterministically.
- Records accept, reject, adjust, cook, corrections, ratings, and reversible preference events.
- Produces shopping lists and a focused step-by-step cooking view.
- Supports Telegram polling or signed webhooks with a mandatory user allowlist.
- Provides management views for profiles, preferences, meals, ratings, recipes, decisions, runtime status, and corrections.

When Ollama is not configured, the development app uses three deterministic sample candidates. The web PWA also falls back to visibly labelled demo data if the API is unavailable.

## Run with Docker Compose

Requirements: Docker with Compose v2.

```sh
cp .env.example .env
docker compose up --build
```

Open <http://localhost:4179>. The API is at <http://localhost:8091>; PostgreSQL is exposed on port `55432`.

To include Ollama:

```sh
OLLAMA_URL=http://ollama:11434 docker compose --profile ollama up --build
docker compose exec ollama ollama pull qwen3:8b
```

Set Telegram values in `.env` only. Suppervisor refuses to start with a bot token but no `TELEGRAM_ALLOWED_USER_IDS`.

## Run services directly

Start PostgreSQL, export `DATABASE_URL`, then:

```sh
go run ./cmd/suppervisor migrate
go run ./cmd/suppervisor
```

In a second terminal:

```sh
cd web
npm ci
npm run dev
```

The Vite server runs on <http://localhost:4179> and proxies `/api` to port `8091`.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | local PostgreSQL | pgx connection string |
| `OLLAMA_URL` | empty | Ollama base URL; empty enables deterministic development candidates |
| `OLLAMA_MODEL` | `qwen3:8b` | local generation model |
| `AUTH_DEV_BYPASS` | `true` | skips OIDC only outside production |
| `DEV_DEMO_DATA` | `true` | seeds an accepted sample meal outside production |
| `OIDC_ISSUER` | empty | Keycloak realm issuer; required in production |
| `OIDC_CLIENT_ID` | `suppervisor` | expected ID-token audience |
| `TELEGRAM_MODE` | `polling` | `polling` or `webhook` |
| `TELEGRAM_BOT_TOKEN` | empty | enables Telegram |
| `TELEGRAM_ALLOWED_USER_IDS` | empty | comma-separated numeric allowlist; required with a token |
| `TELEGRAM_WEBHOOK_URL` | empty | public webhook endpoint |
| `TELEGRAM_WEBHOOK_SECRET` | empty | verifies Telegram’s secret-token header |

`APP_ENV=production` rejects `AUTH_DEV_BYPASS=true` and requires `OIDC_ISSUER`.

## Quality checks

```sh
go test ./...
go vet ./...
cd web && npm ci && npm run lint && npm run build
helm lint deploy/helm/suppervisor --strict
helm template suppervisor deploy/helm/suppervisor
```

Migrations live in `internal/store/migrations`. Every migration must include non-empty `-- +suppervisor Up` and `-- +suppervisor Down` sections; the test suite validates this.

## Production

The Helm chart under `deploy/helm/suppervisor` targets Kubernetes/Talos and expects external PostgreSQL plus existing Kubernetes Secrets. Application containers run non-root with read-only root filesystems, health probes, resource limits, dropped capabilities, short-lived migration Jobs, and graceful termination. Optional in-cluster Ollama has PVC and GPU controls.

See [Talos deployment notes](docs/talos-deployment.md) and [architecture](docs/architecture.md).
