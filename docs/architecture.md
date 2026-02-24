# Architecture

## Overview

LLM Cost Guardian operates as a transparent HTTP proxy between client applications and LLM API providers. It intercepts every request, extracts token usage from responses, calculates costs using provider-specific pricing, and stores the data for reporting and budget enforcement.

## Component Architecture

### Core Components

| Component | Package | Responsibility |
|-----------|---------|---------------|
| Provider Registry | `pkg/providers` | Manages provider pricing data and cost-per-token lookups |
| Token Counter | `pkg/tokenizer` | Counts tokens using tiktoken (OpenAI) or estimation (others) |
| Cost Calculator | `pkg/tracker` | Computes USD costs from token counts and provider pricing |
| Usage Tracker | `pkg/tracker` | Orchestrates recording, reporting, and budget checking |
| Budget Manager | `pkg/tracker` | Enforces spending limits and dispatches threshold alerts |
| Storage | `pkg/storage` | Persists usage records and budgets in SQLite (WAL mode) |
| Alert System | `pkg/alerts` | Delivers notifications via Slack webhooks or generic HTTP |
| Proxy Handler | `internal/proxy` | Transparent reverse proxy with cost tracking middleware |
| API Server | `internal/server` | Health check and metrics REST API |
| CLI | `internal/cli` | Command-line interface for manual tracking and management |

### Request Flow

1. Client sends API request to LCG proxy instead of directly to the LLM provider
2. Proxy reads request body and extracts model name
3. If `deny_on_exceed` is enabled, proxy checks budget before forwarding
4. Request is forwarded to the actual LLM API via `httputil.ReverseProxy`
5. Response is captured via `ModifyResponse` callback
6. Token usage is extracted from the API response `usage` field
7. Cost is calculated using provider pricing data
8. Usage record is persisted to SQLite asynchronously
9. Budget spend is updated and thresholds checked
10. Cost headers (`X-LLM-Cost`, `X-LLM-Input-Tokens`, `X-LLM-Output-Tokens`) are injected
11. Response is returned to client

### Data Model

**usage_records**: Stores individual API call records with provider, model, token counts, cost, project, and timestamp.

**budgets**: Stores spending limits with period (daily/weekly/monthly), alert thresholds, and current spend accumulator.

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
