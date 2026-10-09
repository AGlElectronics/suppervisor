# Architecture

Suppervisor is a modular monolith. Iteration one deliberately has no worker process: the API and Telegram polling/webhook adapter share one binary and one graceful-shutdown boundary.

## Request path

```text
PWA / Telegram
      │
      ▼
delivery adapters ── OIDC / Telegram allowlist
      │
      ▼
application service
      ├── conservative natural-language structuring
      ├── provider-neutral candidate generation ── Ollama
      ├── hard-rule validation
      ├── deterministic ranking
      └── workflow commands
      │
      ▼
PostgreSQL
```

The generator returns recipe facts; it does not make policy decisions. `internal/domain.Planner` owns all constraint checks and score arithmetic. Identical inputs and history produce identical ordering.

## Persistence and learning

Profiles contain explicit likes, hard dislikes, baseline preference weights, and a materialized current preference map. Ratings create append-only `preference_events`; each event records profile, key, delta, reason, source rating, timestamp, and optional reversal timestamp. Rebuilding the current map starts with baseline preferences and sums non-reverted events. Reversing an event never deletes evidence.

Recipe bodies, requests, scores, and violations are stored as JSONB so iteration-one schemas can evolve while decision metadata remains queryable. Every accepted or cooked decision participates in recent-meal novelty scoring.

## Generation failure policy

Ollama receives a JSON-only schema request. Missing or malformed candidate fields cause one corrective retry containing the validation error. Transport errors fail the request visibly. When Ollama is deliberately unconfigured in development, a deterministic candidate provider keeps the complete workflow runnable; production should configure Ollama.

## Authentication boundaries

- Browser REST uses Keycloak OIDC ID tokens.
- `AUTH_DEV_BYPASS` is accepted only outside `APP_ENV=production`.
- Telegram requires a numeric user allowlist whenever a bot token is configured.
- Webhooks require Telegram’s `X-Telegram-Bot-Api-Secret-Token`.
- Health endpoints are unauthenticated and expose no user data.

## Deployment

Compose starts PostgreSQL, an explicit migration job, backend, web, and optional profile-gated Ollama. The Helm chart uses external PostgreSQL, existing Secrets, migration hooks, ingress path routing, non-root/read-only application containers, probes, limits, and optional Ollama storage/GPU resources.
