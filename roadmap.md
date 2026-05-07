# Roadmap

## v1.2.0 — Azure & Bedrock Pricing + Rate-Limit Budgets (target: 2026-06-30)

- Azure OpenAI pricing data (per-deployment, per-region) in YAML pricing registry
- AWS Bedrock pricing for Claude, Titan, and Llama model families
- Rate-limit aware budget enforcement: pre-emptively reject requests approaching limit
- `guardian budget forecast` command — project monthly spend from rolling 7-day average

## v1.3.0 — Grafana & Alertmanager Integration (target: 2026-08-31)

- Grafana dashboard bundle (JSON provisioning) for cost, token usage, and budget burn
- Alertmanager webhook sink alongside existing Slack/PagerDuty alerts
- Multi-window budget periods: hourly, daily, weekly, monthly all enforced independently
- Helm chart update with Grafana sidecar and pre-configured alert rules

## v2.0.0 — Multi-Tenant Team Budgets (target: 2026-Q4)

- Team-level budget namespaces with per-team token quotas and rollover policies
- SSO-integrated audit trail: attribute spend to individual users via JWT claims
- Cost anomaly detection: statistical spike detection with configurable sensitivity
- Public pricing registry contribution model — community PRs for new providers
