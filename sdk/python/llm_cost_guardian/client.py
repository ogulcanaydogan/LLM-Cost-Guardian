from __future__ import annotations

from typing import Any

import httpx


class _BaseClient:
    def __init__(
        self,
        *,
        base_url: str,
        api_key: str | None = None,
        default_project: str | None = None,
        default_tenant: str | None = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.default_project = default_project
        self.default_tenant = default_tenant

    def _auth_headers(self) -> dict[str, str]:
        headers: dict[str, str] = {}
        if self.api_key:
            headers["X-LCG-API-Key"] = self.api_key
        return headers

    def proxy_headers(
        self,
        *,
        target: str,
        provider: str | None = None,
        project: str | None = None,
        tenant: str | None = None,
        extra_headers: dict[str, str] | None = None,
    ) -> dict[str, str]:
        headers = dict(extra_headers or {})
        headers["X-LCG-Target"] = target

        resolved_project = project or self.default_project
        if resolved_project:
            headers["X-LCG-Project"] = resolved_project

        resolved_tenant = tenant or self.default_tenant
        if resolved_tenant:
            headers["X-LCG-Tenant"] = resolved_tenant

        if provider:
            headers["X-LCG-Provider"] = provider

        headers.update(self._auth_headers())
        return headers

    def _query_params(self, **filters: Any) -> dict[str, Any]:
        params = {key: value for key, value in filters.items() if value not in (None, "")}
        if "tenant" not in params and self.default_tenant:
            params["tenant"] = self.default_tenant
        return params


class LCGClient(_BaseClient):
    def __init__(
        self,
        *,
        base_url: str,
        api_key: str | None = None,
        default_project: str | None = None,
        default_tenant: str | None = None,
        client: httpx.Client | None = None,
    ) -> None:
        super().__init__(
            base_url=base_url,
            api_key=api_key,
            default_project=default_project,
            default_tenant=default_tenant,
        )
        self.client = client or httpx.Client(base_url=self.base_url)

    def health(self) -> dict[str, Any]:
        return self._get_json("/healthz")

    def usage(self, **filters: Any) -> list[dict[str, Any]]:
        return self._get_json("/api/v1/usage", **filters)

    def summary(self, **filters: Any) -> dict[str, Any]:
        return self._get_json("/api/v1/summary", **filters)

    def anomalies(self, **filters: Any) -> list[dict[str, Any]]:
        return self._get_json("/api/v1/anomalies", **filters)

    def forecast(self, **filters: Any) -> list[dict[str, Any]]:
        return self._get_json("/api/v1/forecast", **filters)

    def recommendations(self, **filters: Any) -> list[dict[str, Any]]:
        return self._get_json("/api/v1/recommendations", **filters)

    def prompt_optimizations(self, **filters: Any) -> list[dict[str, Any]]:
        return self._get_json("/api/v1/prompt-optimizations", **filters)

    def proxy_request(
        self,
        *,
        target: str,
        path: str = "/",
        provider: str | None = None,
        project: str | None = None,
        tenant: str | None = None,
        method: str = "GET",
        headers: dict[str, str] | None = None,
        **kwargs: Any,
    ) -> httpx.Response:
        request_headers = self.proxy_headers(
            target=target,
            provider=provider,
            project=project,
            tenant=tenant,
            extra_headers=headers,
        )
        response = self.client.request(method, path, headers=request_headers, **kwargs)
        response.raise_for_status()
        return response

    def close(self) -> None:
        self.client.close()

    def _get_json(self, path: str, **filters: Any) -> Any:
        response = self.client.get(path, params=self._query_params(**filters), headers=self._auth_headers())
        response.raise_for_status()
        return response.json()


class AsyncLCGClient(_BaseClient):
    def __init__(
        self,
        *,
        base_url: str,
        api_key: str | None = None,
        default_project: str | None = None,
        default_tenant: str | None = None,
        client: httpx.AsyncClient | None = None,
    ) -> None:
        super().__init__(
            base_url=base_url,
            api_key=api_key,
            default_project=default_project,
            default_tenant=default_tenant,
        )
        self.client = client or httpx.AsyncClient(base_url=self.base_url)

    async def health(self) -> dict[str, Any]:
        return await self._get_json("/healthz")

    async def usage(self, **filters: Any) -> list[dict[str, Any]]:
        return await self._get_json("/api/v1/usage", **filters)

    async def summary(self, **filters: Any) -> dict[str, Any]:
        return await self._get_json("/api/v1/summary", **filters)

    async def anomalies(self, **filters: Any) -> list[dict[str, Any]]:
        return await self._get_json("/api/v1/anomalies", **filters)

    async def forecast(self, **filters: Any) -> list[dict[str, Any]]:
        return await self._get_json("/api/v1/forecast", **filters)

    async def recommendations(self, **filters: Any) -> list[dict[str, Any]]:
        return await self._get_json("/api/v1/recommendations", **filters)

    async def prompt_optimizations(self, **filters: Any) -> list[dict[str, Any]]:
        return await self._get_json("/api/v1/prompt-optimizations", **filters)

    async def proxy_request(
        self,
        *,
        target: str,
        path: str = "/",
        provider: str | None = None,
        project: str | None = None,
        tenant: str | None = None,
        method: str = "GET",
        headers: dict[str, str] | None = None,
        **kwargs: Any,
    ) -> httpx.Response:
        request_headers = self.proxy_headers(
            target=target,
            provider=provider,
            project=project,
            tenant=tenant,
            extra_headers=headers,
        )
        response = await self.client.request(method, path, headers=request_headers, **kwargs)
        response.raise_for_status()
        return response

    async def aclose(self) -> None:
        await self.client.aclose()

    async def _get_json(self, path: str, **filters: Any) -> Any:
        response = await self.client.get(path, params=self._query_params(**filters), headers=self._auth_headers())
        response.raise_for_status()
        return response.json()
