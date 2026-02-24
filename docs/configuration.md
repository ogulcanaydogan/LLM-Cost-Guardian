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
  max_body_size: 10485760         # Max request body (10 MB)
  deny_on_exceed: false           # Block requests when budget exceeded
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
  dir: pricing/                   # Directory containing YAML pricing files

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
| `proxy.deny_on_exceed` | `LCG_PROXY_DENY_ON_EXCEED` |
| `alerts.slack.enabled` | `LCG_ALERTS_SLACK_ENABLED` |
| `alerts.slack.webhook_url` | `LCG_ALERTS_SLACK_WEBHOOK_URL` |
| `logging.level` | `LCG_LOGGING_LEVEL` |
| `defaults.project` | `LCG_DEFAULTS_PROJECT` |
