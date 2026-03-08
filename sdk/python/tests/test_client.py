from __future__ import annotations

import asyncio
import unittest

import httpx

from llm_cost_guardian import AsyncLCGClient, LCGClient


def mock_transport() -> httpx.MockTransport:
    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/healthz":
            return httpx.Response(200, json={"status": "ok"})
        if request.url.path == "/api/v1/summary":
            return httpx.Response(200, json={"total_cost_usd": 1.25, "record_count": 1})
        if request.url.path == "/api/v1/anomalies":
            return httpx.Response(200, json=[{"tenant": "default", "severity": "warning"}])
        return httpx.Response(200, json={"ok": True}, headers={"X-LLM-Cost": "0.42"})

    return httpx.MockTransport(handler)


class ClientTests(unittest.TestCase):
    def test_sync_client(self) -> None:
        client = LCGClient(
            base_url="http://localhost:8080",
            api_key="lcg_test",
            default_project="proj-a",
            default_tenant="default",
            client=httpx.Client(base_url="http://localhost:8080", transport=mock_transport()),
        )

        self.assertEqual(client.health()["status"], "ok")
        self.assertEqual(client.summary()["total_cost_usd"], 1.25)
        self.assertEqual(client.anomalies()[0]["severity"], "warning")

        headers = client.proxy_headers(target="https://api.openai.com/v1/chat/completions", provider="openai")
        self.assertEqual(headers["X-LCG-API-Key"], "lcg_test")
        self.assertEqual(headers["X-LCG-Project"], "proj-a")

        response = client.proxy_request(
            target="https://api.openai.com/v1/chat/completions",
            path="/v1/chat/completions",
            provider="openai",
            method="POST",
            json={"model": "gpt-4o"},
        )
        self.assertEqual(response.headers["X-LLM-Cost"], "0.42")
        client.close()

    def test_async_client(self) -> None:
        async def run() -> None:
            client = AsyncLCGClient(
                base_url="http://localhost:8080",
                api_key="lcg_async",
                default_tenant="default",
                client=httpx.AsyncClient(base_url="http://localhost:8080", transport=mock_transport()),
            )
            self.assertEqual((await client.health())["status"], "ok")
            response = await client.proxy_request(
                target="https://api.openai.com/v1/chat/completions",
                path="/v1/chat/completions",
                provider="openai",
                method="POST",
                json={"model": "gpt-4o"},
            )
            self.assertEqual(response.headers["X-LLM-Cost"], "0.42")
            await client.aclose()

        asyncio.run(run())


if __name__ == "__main__":
    unittest.main()
