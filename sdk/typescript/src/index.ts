export type ReportPeriod = "daily" | "weekly" | "monthly";

export interface UsageRecord {
  id: string;
  tenant?: string;
  tenant_id?: string;
  provider: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  cost_usd: number;
  project: string;
  metadata?: string;
  timestamp: string;
}

export interface UsageSummary {
  total_cost_usd: number;
  total_input_tokens: number;
  total_output_tokens: number;
  record_count: number;
  by_tenant?: Record<string, number>;
  by_provider?: Record<string, number>;
  by_model?: Record<string, number>;
  by_project?: Record<string, number>;
}

export interface UsageAnomaly {
  tenant: string;
  provider?: string;
  model?: string;
  project?: string;
  granularity: string;
  bucket_start: string;
  observed_cost_usd: number;
  baseline_cost_usd: number;
  z_score: number;
  severity: string;
  message: string;
}

export interface SpendForecast {
  tenant: string;
  project?: string;
  horizon_days: number;
  forecast_cost_usd: number;
  average_daily_cost_usd: number;
  trend_daily_delta_usd: number;
  confidence: string;
}

export interface ModelRecommendation {
  tenant: string;
  project?: string;
  current_provider: string;
  current_model: string;
  suggested_provider: string;
  suggested_model: string;
  estimated_savings_usd: number;
  estimated_savings_pct: number;
  reason: string;
}

export interface PromptOptimization {
  tenant: string;
  project?: string;
  provider: string;
  model: string;
  severity: string;
  suggestion: string;
  evidence: string;
  estimated_impact: string;
  average_input_output_ratio?: number;
}

export interface HealthResponse {
  status: string;
}

export interface UsageFilters {
  tenant?: string;
  provider?: string;
  model?: string;
  project?: string;
}

export interface SummaryFilters extends UsageFilters {
  period?: ReportPeriod;
}

export interface ProxyHeadersOptions {
  apiKey?: string;
  provider?: string;
  tenant?: string;
  project?: string;
  extraHeaders?: HeadersInit;
}

export interface ProxyFetchOptions extends ProxyHeadersOptions {
  path?: string;
  requestInit?: RequestInit;
  target: string;
}

export interface LCGClientOptions {
  apiKey?: string;
  baseUrl: string;
  defaultProject?: string;
  defaultTenant?: string;
  fetch?: typeof fetch;
}

export class LCGClient {
  private readonly apiKey?: string;
  private readonly baseUrl: URL;
  private readonly defaultProject?: string;
  private readonly defaultTenant?: string;
  private readonly fetchImpl: typeof fetch;

  constructor(options: LCGClientOptions) {
    this.apiKey = options.apiKey;
    this.baseUrl = new URL(normalizeBaseUrl(options.baseUrl));
    this.defaultProject = options.defaultProject;
    this.defaultTenant = options.defaultTenant;
    this.fetchImpl = options.fetch ?? fetch;
  }

  async health(): Promise<HealthResponse> {
    return this.jsonRequest<HealthResponse>("/healthz");
  }

  async usage(filters: UsageFilters = {}): Promise<UsageRecord[]> {
    return this.jsonRequest<UsageRecord[]>("/api/v1/usage", filters);
  }

  async summary(filters: SummaryFilters = {}): Promise<UsageSummary> {
    return this.jsonRequest<UsageSummary>("/api/v1/summary", filters);
  }

  async anomalies(filters: UsageFilters = {}): Promise<UsageAnomaly[]> {
    return this.jsonRequest<UsageAnomaly[]>("/api/v1/anomalies", filters);
  }

  async forecast(filters: UsageFilters = {}): Promise<SpendForecast[]> {
    return this.jsonRequest<SpendForecast[]>("/api/v1/forecast", filters);
  }

  async recommendations(filters: UsageFilters = {}): Promise<ModelRecommendation[]> {
    return this.jsonRequest<ModelRecommendation[]>("/api/v1/recommendations", filters);
  }

  async promptOptimizations(filters: UsageFilters = {}): Promise<PromptOptimization[]> {
    return this.jsonRequest<PromptOptimization[]>("/api/v1/prompt-optimizations", filters);
  }

  proxyHeaders(target: string, options: ProxyHeadersOptions = {}): Headers {
    const headers = new Headers(options.extraHeaders);
    headers.set("X-LCG-Target", target);

    const tenant = options.tenant ?? this.defaultTenant;
    if (tenant) {
      headers.set("X-LCG-Tenant", tenant);
    }

    const project = options.project ?? this.defaultProject;
    if (project) {
      headers.set("X-LCG-Project", project);
    }

    const apiKey = options.apiKey ?? this.apiKey;
    if (apiKey) {
      headers.set("X-LCG-API-Key", apiKey);
    }

    if (options.provider) {
      headers.set("X-LCG-Provider", options.provider);
    }

    return headers;
  }

  async proxyFetch(options: ProxyFetchOptions): Promise<Response> {
    const url = new URL(options.path ?? "/", this.baseUrl);
    const requestInit: RequestInit = {
      ...options.requestInit,
      headers: this.proxyHeaders(options.target, {
        apiKey: options.apiKey,
        provider: options.provider,
        tenant: options.tenant,
        project: options.project,
        extraHeaders: options.requestInit?.headers
      })
    };

    return this.fetchImpl(url, requestInit);
  }

  private async jsonRequest<T>(pathname: string, query?: UsageFilters | SummaryFilters): Promise<T> {
    const url = new URL(pathname, this.baseUrl);
    if (query) {
      for (const [key, value] of Object.entries(query)) {
        if (value) {
          url.searchParams.set(key, value);
        }
      }
    }

    const headers = new Headers();
    if (this.apiKey) {
      headers.set("X-LCG-API-Key", this.apiKey);
    }

    const response = await this.fetchImpl(url, { headers });
    if (!response.ok) {
      throw new Error(`LLM Cost Guardian request failed with status ${response.status}`);
    }

    return (await response.json()) as T;
  }
}

function normalizeBaseUrl(baseUrl: string): string {
  if (baseUrl.endsWith("/")) {
    return baseUrl;
  }
  return `${baseUrl}/`;
}
