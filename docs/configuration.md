# Configuration Reference

LLM Cost Guardian is configured via YAML file with environment variable overrides.

## Config File Location

Default search paths (in order):
1. Path specified via `--config` flag
2. `~/.lcg/config.yaml`
3. `./config.yaml`

## Full Reference

```yaml
# Database storage
storage:
  path: ~/.lcg/guardian.db        # SQLite database path

# Transparent proxy settings
proxy:
  listen: ":8080"                 # Listen address
  read_timeout: 30s               # HTTP read timeout
  write_timeout: 60s              # HTTP write timeout
  max_body_size: 10485760         # Max request body (10 MB), enforced before upstream calls
  deny_on_exceed: false           # Block requests when applicable budget is exceeded
  add_cost_headers: true          # Add X-LLM-Cost headers to responses

# Alert integrations
alerts:
  slack:
    enabled: false                # Enable Slack notifications
    webhook_url: ""               # Slack incoming webhook URL
    channel: "#llm-costs"         # Target channel
  webhook:
    enabled: false                # Enable generic webhook
    url: ""                       # Webhook endpoint URL
    secret: ""                    # HMAC-SHA256 signing secret

# Pricing data
pricing:
  dir: pricing/                   # Directory containing provider YAML pricing files

# Tenant auth
auth:
  multi_tenant_enabled: false     # Require API key auth and tenant isolation
  default_tenant: default         # Tenant used for legacy or bootstrap access
  bootstrap_admin_key: ""         # Optional admin key for bootstrap/API maintenance

# Logging
logging:
  level: info                     # Log level: debug, info, warn, error
  format: json                    # Log format: json, text

# Default values
defaults:
  project: default                # Default project name for untagged requests
```

## Environment Variable Overrides

All configuration keys can be overridden via environment variables with the `LCG_` prefix. Nested keys use `_` as separator.

| Config Key | Environment Variable |
|-----------|---------------------|
| `storage.path` | `LCG_STORAGE_PATH` |
| `proxy.listen` | `LCG_PROXY_LISTEN` |
| `proxy.max_body_size` | `LCG_PROXY_MAX_BODY_SIZE` |
| `proxy.deny_on_exceed` | `LCG_PROXY_DENY_ON_EXCEED` |
| `proxy.add_cost_headers` | `LCG_PROXY_ADD_COST_HEADERS` |
| `auth.multi_tenant_enabled` | `LCG_AUTH_MULTI_TENANT_ENABLED` |
| `auth.default_tenant` | `LCG_AUTH_DEFAULT_TENANT` |
| `auth.bootstrap_admin_key` | `LCG_AUTH_BOOTSTRAP_ADMIN_KEY` |
| `alerts.slack.enabled` | `LCG_ALERTS_SLACK_ENABLED` |
| `alerts.slack.webhook_url` | `LCG_ALERTS_SLACK_WEBHOOK_URL` |
| `logging.level` | `LCG_LOGGING_LEVEL` |
| `defaults.project` | `LCG_DEFAULTS_PROJECT` |

When `deny_on_exceed` is enabled, requests are checked against global budgets and any budget scoped to the request project. If `max_body_size` is exceeded, the proxy returns `413 Payload Too Large` before forwarding the request.

When `auth.multi_tenant_enabled` is enabled, requests must authenticate with either `X-LCG-API-Key` or `Authorization: Bearer <key>`. The authenticated key resolves a tenant, and all usage, budgets, reports, metrics, and analytics are scoped to that tenant.

## Bundled Pricing Files

The default `pricing/` directory now includes snapshots for:

- `openai.yaml`
- `anthropic.yaml`
- `azure-openai.yaml`
- `bedrock.yaml`
- `vertex-ai.yaml`

These files are editable. If your organization has custom pricing or committed-use discounts, update the YAML values and restart the proxy.

## Operational Endpoints

LLM Cost Guardian exposes these HTTP endpoints on the same listener as the proxy:

- `GET /healthz`
- `GET /metrics`
- `GET /api/v1/usage`
- `GET /api/v1/summary`
- `GET /api/v1/anomalies`
- `GET /api/v1/forecast`
- `GET /api/v1/recommendations`
- `GET /api/v1/prompt-optimizations`

`/metrics` exports tenant-aware series with `tenant`, `provider`, `model`, and `project` labels. The JSON endpoints accept `tenant`, `provider`, `model`, and `project` query filters; non-admin API keys are automatically constrained to their own tenant.
