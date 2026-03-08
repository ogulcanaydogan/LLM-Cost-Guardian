# LLM Cost Guardian TypeScript SDK

This SDK wraps the LLM Cost Guardian JSON API and proxy header contract.

## Install

```bash
npm install @ogulcanaydogan/llm-cost-guardian
```

## Usage

```ts
import { LCGClient } from "@ogulcanaydogan/llm-cost-guardian";

const client = new LCGClient({
  baseUrl: "http://127.0.0.1:8080",
  apiKey: "lcg_your_key",
  defaultProject: "payments",
  defaultTenant: "default"
});

const health = await client.health();
const summary = await client.summary({ period: "daily" });
const anomalies = await client.anomalies({ tenant: "default" });

const response = await client.proxyFetch({
  target: "https://api.openai.com/v1/chat/completions",
  path: "/v1/chat/completions",
  provider: "openai",
  requestInit: {
    method: "POST",
    headers: {
      Authorization: `Bearer ${process.env.OPENAI_API_KEY ?? ""}`,
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      model: "gpt-4o",
      messages: [{ role: "user", content: "Hello" }]
    })
  }
});

console.log(health.status, summary.total_cost_usd, anomalies.length, response.headers.get("X-LLM-Cost"));
```

The SDK uses `X-LCG-API-Key` for LCG authentication so upstream `Authorization` headers remain available for provider credentials.

Available JSON API helpers:

- `health()`
- `usage()`
- `summary()`
- `anomalies()`
- `forecast()`
- `recommendations()`
- `promptOptimizations()`
- `proxyHeaders()`
- `proxyFetch()`
