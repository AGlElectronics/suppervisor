# Talos Kubernetes deployment

The chart targets a conformant Kubernetes cluster on Talos Linux. Talos supplies the immutable host; Suppervisor does not require privileged pods, host mounts, or a service-account token.

## Prerequisites

- An ingress controller and TLS issuer.
- External PostgreSQL with TLS, backups, and a dedicated least-privilege role.
- Keycloak realm/client configured for the public origin.
- A registry containing immutable backend and web image tags.
- Optional: a default StorageClass and GPU operator/device plugin for in-cluster Ollama.

Create secret references without committing values:

```sh
kubectl create namespace suppervisor
kubectl -n suppervisor create secret generic suppervisor-database \
  --from-literal=database-url='postgres://RUNTIME_USER:PASSWORD@HOST:5432/suppervisor?sslmode=require'
kubectl -n suppervisor create secret generic suppervisor-migration-database \
  --from-literal=database-url='postgres://MIGRATION_USER:PASSWORD@HOST:5432/suppervisor?sslmode=require'
kubectl -n suppervisor create secret generic suppervisor-secrets \
  --from-literal=telegram-bot-token='' \
  --from-literal=telegram-allowed-user-ids='' \
  --from-literal=telegram-webhook-secret=''
```

Grant the runtime role connect and DML rights on Suppervisor tables and sequences,
but not schema creation or alteration. Grant the migration role the DDL rights
needed by committed migrations. The migration Job is the only workload that reads
the migration-role Secret.

Render and inspect before applying:

```sh
helm lint deploy/helm/suppervisor --strict
helm template suppervisor deploy/helm/suppervisor \
  --namespace suppervisor \
  --set image.backend.repository="$CI_REGISTRY_IMAGE" \
  --set image.web.repository="$CI_REGISTRY_IMAGE/web" \
  --set image.backend.tag=IMMUTABLE_TAG \
  --set image.web.tag=IMMUTABLE_TAG \
  --set ingress.host=suppervisor.example.com \
  --set config.webOrigin=https://suppervisor.example.com \
  --set auth.issuer=https://keycloak.example.com/realms/home
```

Use the commit-SHA tags emitted by the release pipeline. Its backend repository is
`$CI_REGISTRY_IMAGE`; its web repository is `$CI_REGISTRY_IMAGE/web`. The chart
rejects empty and `latest` application tags when `config.environment=production`.

Install:

```sh
helm upgrade --install suppervisor deploy/helm/suppervisor \
  --namespace suppervisor --create-namespace \
  --values production-values.yaml
```

The pre-install/pre-upgrade migration Job must succeed before workloads roll out. PostgreSQL advisory/transaction semantics make each migration atomic, and migration files contain explicit down sections for operator-led rollback.

Migrations and application rollouts use an expand/contract policy: a release must
remain compatible with the preceding schema while old pods drain. Helm rollback
does not automatically execute destructive down migrations. Take a database backup
before upgrading; for a schema rollback, stop application writers, validate the
committed down SQL against a restored backup, and execute it explicitly before
restoring the older release.

## Keycloak

Create a public OIDC client named `suppervisor`, allow only the production redirect/origin values, use Authorization Code + PKCE in the browser gateway, and map the expected audience. Keep `auth.devBypass=false`; the backend also refuses a production startup if bypass is enabled.

## Optional Ollama

Set `ollama.enabled=true`, choose a PVC size, and preload the configured model after first start. For GPU nodes:

```yaml
ollama:
  enabled: true
  persistence:
    size: 50Gi
    storageClass: local-path
  gpu:
    enabled: true
    resourceName: nvidia.com/gpu
    count: 1
```

Use Talos machine configuration extensions and the matching Kubernetes device plugin for the GPU vendor. Taint GPU nodes and add chart tolerations/node selectors in the production values file.

## Operations

- Readiness includes PostgreSQL connectivity; liveness only confirms the process.
- When Ollama is configured, readiness also requires its API and configured model.
- The Go server handles SIGTERM and drains for 15 seconds within a 25-second pod grace period.
- Logs are JSON on stdout.
- Application root filesystems are read-only; writable paths are bounded `emptyDir` volumes.
- Scale backend polling mode to one replica, or use Telegram webhook mode for normal multi-replica production.
- Back up external PostgreSQL and Ollama model storage independently; model data is reproducible, preference events are not.
- Config, auth, and secret-reference changes restart backend pods. When an external
  Secret's values change without its name or keys changing, also change
  `secrets.rolloutToken` in the release values.
