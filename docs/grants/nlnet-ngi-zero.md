# NLnet NGI Zero Commons Fund Application

| Field | Value |
|-------|-------|
| **Fund** | NGI Zero Commons Fund |
| **URL** | https://nlnet.nl/propose/ |
| **Deadline** | 2026-04-01 |
| **Requested Amount** | EUR 35,000 |
| **Project** | LLM-Cost-Guardian |
| **License** | Apache 2.0 |
| **Repository** | https://github.com/ogulcanaydogan/LLM-Cost-Guardian |
| **Applicant** | Ogulcan Aydogan |
| **Language/Stack** | Go 1.25, SQLite, Prometheus |

---

## 1. Abstract

LLM Cost Guardian is an open-source transparent proxy that tracks, budgets, and optimizes LLM spending across five providers (OpenAI, Anthropic, Azure OpenAI, AWS Bedrock, Google Vertex AI). It sits between applications and provider APIs, counting tokens, calculating costs from YAML-based pricing tables, enforcing budget limits, and injecting cost headers into responses, all with under 10ms overhead. The project ships as a single Go binary with no CGO dependency, making it portable across Linux, macOS, and Windows on both amd64 and arm64. It includes TypeScript and Python SDKs, Prometheus metrics, Grafana dashboards, Slack alerts, anomaly detection, spend forecasting, and chargeback report generation. The codebase has 10,280 lines of Go, 25 test files, 80%+ CI-enforced coverage, and two releases (v1.0.0 and v1.1.0). Today, cost visibility for LLM workloads is either nonexistent or locked inside vendor-specific dashboards that can't show cross-provider totals. This forces teams into blind budgeting or manual spreadsheet tracking. The proxy approach solves this at the infrastructure level: change one URL and every API call gets tracked automatically. This grant would fund multi-currency support, FinOps integration (Kubecost, OpenCost), an interactive cost explorer dashboard, and hardened multi-tenant isolation for shared deployments.

---

## 2. Description of Work

### Background

LLM costs are the biggest operational surprise in production AI systems. A single misconfigured batch job can burn thousands of dollars in minutes. Teams running multiple providers, which is increasingly common as organizations avoid single-vendor dependency, face an even harder problem: there's no unified view of what they're spending.

The existing options aren't great. Provider dashboards (OpenAI Usage, Anthropic Console, AWS Cost Explorer) only show their own costs. Third-party tools like Helicone and LangSmith offer cost tracking but require sending request data through their servers, which many organizations can't accept for privacy or compliance reasons. Self-hosted alternatives are scarce, and none of them handle five providers with budget enforcement and alerting in a single binary.

LLM Cost Guardian takes a different approach. Instead of requiring SDK changes or API wrappers, it runs as a reverse proxy. Applications point their LLM API calls at the proxy URL instead of the provider URL. The proxy forwards requests unchanged, captures the response (including streaming SSE), counts tokens, calculates costs, records everything in SQLite, and returns the original response with cost headers attached. Budget checks happen before the request is forwarded, so overspending gets blocked, not just reported.

### Current State

The project is production-functional with v1.1.0 released. Here's what exists:

- **5 provider adapters**: OpenAI, Anthropic, Azure OpenAI, AWS Bedrock, Google Vertex AI
- **Token counting**: tiktoken for OpenAI models, estimation fallback for others
- **Budget enforcement**: daily/weekly/monthly periods, per-tenant and per-project scopes, configurable alert thresholds, optional request blocking
- **Multi-tenancy**: API-key based tenant isolation, bootstrap admin auth
- **Analytics**: anomaly detection (statistical heuristics), 7-day and 30-day forecasting, model recommendations, prompt optimization insights
- **Reporting**: CSV and PDF chargeback reports, aggregated summaries by provider/model/project/tenant
- **Monitoring**: Prometheus `/metrics` endpoint, Grafana dashboard template, Slack webhooks, generic HTTP webhooks with HMAC signing
- **SDKs**: TypeScript (`@ogulcanaydogan/llm-cost-guardian`) and Python (`llm-cost-guardian`)
- **CI**: linting (golangci-lint), tests with race detector, 80% coverage gate, benchmarks, multi-platform builds, release automation, OpenSSF Scorecard
- **Storage**: SQLite with WAL mode, CGO-free driver (modernc.org/sqlite)

### Trade-offs Worth Knowing

The proxy model has real constraints:

- **Single point of observation.** If traffic doesn't flow through the proxy, it doesn't get tracked. Applications that bypass the proxy (direct SDK calls to providers) create blind spots. This is by design: we track at the infrastructure level, not the application level.
- **SQLite scaling limits.** WAL mode handles concurrent reads well, but write throughput tops out around 1,000-2,000 inserts per second on typical hardware. For organizations processing millions of LLM requests daily, this needs a PostgreSQL backend option.
- **Pricing data maintenance.** Provider pricing changes regularly. The YAML files need manual updates when providers adjust token prices or add new models. We don't scrape pricing pages automatically because providers change their pricing page structure frequently.

### Proposed Work

This grant funds four milestones that address the gaps between "works for a single team" and "works for an organization running shared LLM infrastructure."

---

## 3. Budget

| Milestone | Description | Amount (EUR) |
|-----------|-------------|--------------|
| M1 | Multi-currency and FinOps integration | 10,000 |
| M2 | PostgreSQL backend and scaling | 8,000 |
| M3 | Interactive cost explorer dashboard | 9,000 |
| M4 | Security hardening and documentation | 8,000 |
| **Total** | | **35,000** |

All amounts cover developer time. No hardware costs; CI runs on GitHub Actions free tier.

---

## 4. Milestones and Timeline

### M1: Multi-Currency and FinOps Integration (Months 1-2, EUR 10,000)

**Goal:** Make cost data usable in existing financial workflows rather than forcing teams to build export pipelines.

Deliverables:
- Multi-currency cost tracking (EUR, GBP, JPY alongside USD) with configurable exchange rates and daily rate refresh from a free API (ECB or similar)
- Kubecost integration: export per-namespace LLM costs as custom metrics that Kubecost can aggregate with compute/storage costs
- OpenCost adapter: feed LLM spend into the CNCF cost allocation standard
- FinOps Foundation FOCUS format export for organizations using cloud cost management platforms
- Updated chargeback reports with currency conversion and departmental allocation

**Exit criteria:** Cost data flows into Kubecost and OpenCost in a test cluster. Multi-currency reports generate correctly for 5 currencies. FOCUS export validates against the specification.

### M2: PostgreSQL Backend and Scaling (Months 2-3, EUR 8,000)

**Goal:** Remove the SQLite write throughput ceiling for high-volume deployments.

Deliverables:
- PostgreSQL storage backend as an alternative to SQLite, selectable via configuration
- Connection pooling with pgxpool
- Database migration tooling (SQLite to PostgreSQL one-way migration)
- Write throughput benchmarks comparing SQLite WAL vs PostgreSQL at 100, 1,000, and 10,000 requests/second
- Horizontal proxy scaling documentation (multiple proxy instances sharing one PostgreSQL database)

**Exit criteria:** PostgreSQL backend passes all existing tests. Benchmarks show linear write scaling to 10,000 req/s. Migration tool handles a 1M-row SQLite database without data loss.

### M3: Interactive Cost Explorer Dashboard (Months 3-4, EUR 9,000)

**Goal:** Replace the current JSON API with a visual dashboard that non-technical stakeholders (finance, management) can use directly.

Deliverables:
- Web dashboard built with Go templates and HTMX (no separate frontend build step)
- Views: real-time spend by provider/model/project, budget utilization gauges, anomaly timeline, forecast projections, model cost comparison
- Drill-down from organization to tenant to project to individual request
- Export buttons for CSV, PDF, and FOCUS format
- Authentication via the existing API key system
- Docker Compose setup with dashboard, proxy, and Grafana in one stack

**Exit criteria:** Dashboard renders all analytics views. A non-technical user can find their team's monthly spend in under 30 seconds (verified by user testing).

### M4: Security Hardening and Documentation (Months 4-5, EUR 8,000)

**Goal:** Make the proxy safe for shared multi-tenant deployments and lower the barrier to adoption.

Deliverables:
- Tenant isolation audit: verify that no API key can access another tenant's data through parameter manipulation, timing attacks, or error messages
- Rate limiting per tenant (configurable, not just per-IP)
- Request logging with sensitive field redaction (API keys, prompt content optionally excluded)
- Getting-started guide tested by someone unfamiliar with the project
- Architecture decision records for key design choices
- Helm chart for Kubernetes deployment with sensible defaults
- Contributor guide with local development setup

**Exit criteria:** Security audit findings documented and addressed. Helm chart installs clean on EKS, GKE, and AKS. Getting-started guide gets a new user from zero to tracking costs in under 10 minutes.

---

## 5. NGI Relevance

### How does this project contribute to the Next Generation Internet?

LLM APIs are becoming foundational internet infrastructure, used by search engines, customer support, content moderation, document processing, accessibility tools, and translation services. The cost of using this infrastructure is opaque by design: providers benefit from usage-based pricing that's hard to predict or control.

Three aspects connect to NGI Zero Commons Fund priorities:

**Cost transparency as digital sovereignty.** Organizations that can't see what they're spending can't make informed decisions about which providers to use or whether to self-host. Cost visibility is a prerequisite for digital autonomy. This proxy gives any team the same cost intelligence that only large tech companies currently build internally.

**Open infrastructure for shared benefit.** The proxy is Apache 2.0 licensed with no proprietary components. The FinOps integrations (M1) connect it to CNCF open standards rather than vendor-specific platforms. The PostgreSQL backend (M2) uses standard infrastructure that any organization already operates.

**Privacy-preserving by architecture.** Unlike hosted cost tracking services (Helicone, LangSmith), this proxy runs entirely within the user's infrastructure. No request data, prompt content, or usage patterns leave the organization's network. For public sector organizations, healthcare providers, and legal firms using LLMs, this self-hosted model isn't optional, it's a requirement.

### How does this project relate to the open internet?

Non-profit organizations, academic institutions, and small companies are increasingly dependent on LLM APIs for their operations. Without cost visibility, these organizations face surprise bills, budget overruns, and the inability to compare providers, all of which increase dependency on a single vendor. Open-source cost tracking infrastructure levels the playing field: the same budgeting and optimization tools that large enterprises build internally become available to everyone.

---

## 6. Comparable Projects

| Project | What it does | How LLM Cost Guardian differs |
|---------|-------------|-------------------------------|
| **Helicone** | LLM observability and cost tracking (hosted SaaS) | Requires sending all request data through their servers. Not self-hostable. Doesn't support budget enforcement. |
| **LangSmith** | LLM tracing and evaluation by LangChain | Tied to LangChain framework. Cost tracking is secondary to tracing. Hosted service with privacy implications. |
| **OpenLIT** | Open-source LLM observability | Focuses on OpenTelemetry integration. No budget enforcement, no multi-tenant isolation, no chargeback reports. |
| **LiteLLM Proxy** | Multi-provider LLM proxy | Primarily a routing layer, not a cost governance tool. Python-based with higher overhead. Limited budget enforcement. |
| **Provider dashboards** | OpenAI Usage, Anthropic Console, AWS Cost Explorer | Single-provider only. No cross-provider totals. No budget enforcement. No self-hosted option. |

LLM Cost Guardian is unique in combining transparent proxying, multi-provider cost tracking, budget enforcement, and analytics in a single self-hosted Go binary with under 10ms overhead.

---

## 7. Supporting Materials Checklist

Before submitting, confirm these are ready:

- [ ] Public repository: https://github.com/ogulcanaydogan/LLM-Cost-Guardian
- [ ] Apache 2.0 LICENSE in repository root
- [ ] README with build instructions, CLI usage, and proxy configuration
- [ ] CHANGELOG.md with release history (v1.0.0, v1.1.0)
- [ ] CI workflows passing (lint, test, benchmark, release, scorecard)
- [ ] 25 Go test files with `go test ./...` passing at 80%+ coverage
- [ ] Grafana dashboard template in repository
- [ ] TypeScript and Python SDKs with documentation
- [ ] Docker deployment configuration

---

## 8. Submission Steps

1. Go to https://nlnet.nl/propose/
2. Select **NGI Zero Commons Fund**
3. Fill in form fields using the header table and abstract above
4. For "Describe the work" use Section 2 (background + milestones)
5. For "Budget" enter EUR 35,000 and reference the budget table
6. For "Relevance" use Section 5
7. For "Comparable efforts" use Section 6
8. Submit before **2026-04-01**

---

*Last updated: 2026-03-14*
