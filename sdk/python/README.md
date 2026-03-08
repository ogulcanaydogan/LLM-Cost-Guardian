# LLM Cost Guardian Python SDK

```python
from llm_cost_guardian import LCGClient

client = LCGClient(
    base_url="http://127.0.0.1:8080",
    api_key="lcg_your_key",
    default_project="payments",
    default_tenant="default",
)

health = client.health()
summary = client.summary(period="daily")
anomalies = client.anomalies(tenant="default")

response = client.proxy_request(
    target="https://api.openai.com/v1/chat/completions",
    path="/v1/chat/completions",
    provider="openai",
    method="POST",
    headers={"Content-Type": "application/json"},
    json={
        "model": "gpt-4o",
        "messages": [{"role": "user", "content": "Hello"}],
    },
)

print(health["status"], summary["total_cost_usd"], len(anomalies), response.headers.get("X-LLM-Cost"))
```

The client sends LCG authentication in `X-LCG-API-Key`, which keeps upstream provider `Authorization` headers untouched for proxy requests.

Sync and async clients expose:

- `health()`
- `usage()`
- `summary()`
- `anomalies()`
- `forecast()`
- `recommendations()`
- `prompt_optimizations()`
- `proxy_headers()`
- `proxy_request()`
