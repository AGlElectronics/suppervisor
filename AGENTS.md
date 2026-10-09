# Working on Suppervisor

## Product invariants

- Suppervisor is standalone. Do not couple it to a personal-operations bot.
- Hard rules are deterministic Go code. An LLM may propose and structure; it never waives a diner dislike, time limit, budget, equipment requirement, portion count, or reheating requirement.
- Keep Alan and Partner data separate through requests, ratings, and preference events.
- Preference learning is event-based and reversible. Never mutate learned taste without recording why.
- Local operation is the default. Do not introduce a cloud LLM dependency.
- Keep product copy concise, warm, and useful. Branding lives in `web/src/brand.ts`.

## Architecture

- `cmd/suppervisor`: single iteration-one process (API plus Telegram polling/webhook).
- `internal/domain`: dependency-free rules, scoring, request parsing, and types.
- `internal/llm`: provider-neutral interface and Ollama adapter.
- `internal/store`: PostgreSQL repository and embedded reversible migrations.
- `internal/app`: use-case orchestration.
- `internal/httpapi` and `internal/telegram`: delivery adapters.
- `web`: React/TypeScript Vite PWA with Tailwind, Radix, and Lucide.
- `deploy/helm/suppervisor`: production Kubernetes chart.

Dependencies point inward: delivery → app → domain. Domain must not import storage, transport, or LLM packages.

## Before committing

Run:

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
cd web && npm run lint && npm run build
```

When deployment files change, also run strict Helm lint and render. Use conventional commits. Do not commit `.env`, credentials, tokens, generated `dist`, or database data.

## Database changes

Add a new, ordered SQL file under `internal/store/migrations`. Include both markers:

```sql
-- +suppervisor Up
-- forward change
-- +suppervisor Down
-- exact reversal
```

Never rewrite a migration that may have shipped.
