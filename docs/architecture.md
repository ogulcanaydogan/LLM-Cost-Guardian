# Architecture

## Overview

LLM Cost Guardian operates as a transparent HTTP proxy between client applications and LLM API providers. It intercepts every request, extracts token usage from responses, calculates costs using provider-specific pricing, and stores the data for reporting, chargeback exports, metrics, budget enforcement, multi-tenant isolation, and lightweight optimization analytics.

## Component Architecture

### Core Components

| Component | Package | Responsibility |
|-----------|---------|---------------|
| Provider Registry | `pkg/providers` | Manages provider pricing data and cost-per-token lookups for OpenAI, Anthropic, Azure OpenAI, Bedrock, and Vertex AI |
| Token Counter | `pkg/tokenizer` | Counts tokens using tiktoken (OpenAI) or estimation (others) |
| Cost Calculator | `pkg/tracker` | Computes USD costs from token counts and provider pricing |
| Usage Tracker | `pkg/tracker` | Orchestrates recording, reporting, anomaly detection, forecasting, and recommendations |
| Budget Manager | `pkg/tracker` | Enforces tenant-global and tenant-project spending limits and dispatches threshold alerts |
| Storage | `pkg/storage` | Persists tenants, API keys, usage records, budgets, and rollups in SQLite (WAL mode) |
| Alert System | `pkg/alerts` | Delivers notifications via Slack webhooks or generic HTTP |
| Proxy Handler | `internal/proxy` | Transparent reverse proxy with cost tracking middleware |
| Auth Middleware | `internal/httpauth` | Resolves tenant identity from API keys or bootstrap admin access |
| API Server | `internal/server` | Health check, Prometheus metrics, and JSON usage/report/analytics API |
| Reporting | `internal/reporting` | Generates chargeback exports in CSV and PDF formats |
| CLI | `internal/cli` | Command-line interface for tracking, tenant admin, budgets, reports, and analytics |

### Request Flow

1. Client sends API request to LCG proxy instead of directly to the LLM provider
2. Proxy reads request body and extracts model name
3. Auth middleware resolves the tenant from `X-LCG-API-Key` or `Authorization: Bearer`
4. If `deny_on_exceed` is enabled, proxy checks tenant-global budgets plus any budget scoped to the request project before forwarding
5. Request is forwarded to the actual LLM API via `httputil.ReverseProxy`
6. Non-streaming responses are buffered; streaming responses are passed through live while usage is captured at EOF
7. Token usage is extracted from provider-specific response metadata (`usage`, `usageMetadata`, SSE events, or compatible fields)
8. Cost is calculated using provider pricing data
9. Usage record and rollups are persisted to SQLite under the resolved tenant
10. Budget spend is updated for applicable tenant-global and tenant-project budgets and thresholds checked
11. Cost headers are injected for non-streaming responses; streaming responses expose `X-LCG-Streaming: true`
12. Prometheus and JSON APIs expose the recorded data for dashboards, automation, analytics, and exports
13. Response is returned to client

### Data Model

**tenants** / **api_keys**: Store tenant identity, status, and API-key based access.

**usage_records**: Store individual API call records with tenant, provider, model, token counts, cost, project, derived prompt metadata, and timestamp.

**usage_rollups**: Store hourly and daily aggregates per tenant/project/provider/model for anomaly detection and forecasting.

**budgets**: Store spending limits with optional project scope inside a tenant, period (daily/weekly/monthly), alert thresholds, and current spend accumulator.

## Provider Surface

The proxy currently includes request/response extraction paths for:

- OpenAI and Azure OpenAI chat-completions style payloads
- Anthropic Messages API payloads
- Bedrock Converse-compatible payloads
- Vertex AI Gemini `generateContent`-style payloads

Provider selection is either explicit via `X-LCG-Provider` or inferred from the upstream host/path.

For streaming endpoints, the proxy supports live passthrough plus end-of-stream usage capture for OpenAI, Azure OpenAI, Anthropic, Bedrock, and Vertex AI. If a provider stream does not expose terminal usage, LLM Cost Guardian falls back to prompt/output estimation and still records spend.

## Export and Metrics Surface

- `lcg report --format csv` writes project chargeback summaries or detailed records.
- `lcg report --format pdf` writes a printable chargeback report with summary tables.
- `GET /metrics` exposes Prometheus-style counters for requests, tokens, and spend, labeled by tenant, provider, model, and project.
- `GET /api/v1/anomalies`, `GET /api/v1/forecast`, `GET /api/v1/recommendations`, and `GET /api/v1/prompt-optimizations` expose the production-lite analytics surface.

### SQLite Design Decisions

- **WAL mode**: Enables concurrent reads while writing, critical for proxy performance
- **CGO-free driver** (`modernc.org/sqlite`): Enables cross-compilation for all target platforms
- **Indexed columns**: provider, project, model, and timestamp for fast filtered queries
- **Async writes**: Usage recording is non-blocking to minimize proxy latency

## Configuration

Configuration is loaded via Viper with this precedence (highest first):
1. Environment variables (prefix: `LCG_`)
2. Config file (`~/.lcg/config.yaml` or specified via `--config`)
3. Built-in defaults
