package server

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
)

type metricKey struct {
	Tenant   string
	Provider string
	Model    string
	Project  string
}

type metricTotals struct {
	Requests     int64
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

func renderPrometheusMetrics(summary *tracker.UsageSummary, records []tracker.UsageRecord) string {
	var builder strings.Builder

	builder.WriteString("# HELP lcg_usage_records_total Total recorded LLM requests.\n")
	builder.WriteString("# TYPE lcg_usage_records_total counter\n")
	fmt.Fprintf(&builder, "lcg_usage_records_total %d\n", summary.RecordCount)

	builder.WriteString("# HELP lcg_cost_usd_total Total recorded LLM cost in USD.\n")
	builder.WriteString("# TYPE lcg_cost_usd_total counter\n")
	fmt.Fprintf(&builder, "lcg_cost_usd_total %.6f\n", summary.TotalCostUSD)

	builder.WriteString("# HELP lcg_input_tokens_total Total recorded input tokens.\n")
	builder.WriteString("# TYPE lcg_input_tokens_total counter\n")
	fmt.Fprintf(&builder, "lcg_input_tokens_total %d\n", summary.TotalInputTokens)

	builder.WriteString("# HELP lcg_output_tokens_total Total recorded output tokens.\n")
	builder.WriteString("# TYPE lcg_output_tokens_total counter\n")
	fmt.Fprintf(&builder, "lcg_output_tokens_total %d\n", summary.TotalOutputTokens)

	grouped := make(map[metricKey]metricTotals)
	for _, record := range records {
		key := metricKey{
			Tenant:   record.Tenant,
			Provider: record.Provider,
			Model:    record.Model,
			Project:  record.Project,
		}
		total := grouped[key]
		total.Requests++
		total.InputTokens += record.InputTokens
		total.OutputTokens += record.OutputTokens
		total.CostUSD += record.CostUSD
		grouped[key] = total
	}

	keys := make([]metricKey, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Tenant != keys[j].Tenant {
			return keys[i].Tenant < keys[j].Tenant
		}
		if keys[i].Provider != keys[j].Provider {
			return keys[i].Provider < keys[j].Provider
		}
		if keys[i].Model != keys[j].Model {
			return keys[i].Model < keys[j].Model
		}
		return keys[i].Project < keys[j].Project
	})

	builder.WriteString("# HELP lcg_requests_total Total LLM requests by tenant, provider, model, and project.\n")
	builder.WriteString("# TYPE lcg_requests_total counter\n")
	for _, key := range keys {
		total := grouped[key]
		fmt.Fprintf(&builder,
			"lcg_requests_total{tenant=%q,provider=%q,model=%q,project=%q} %d\n",
			key.Tenant,
			key.Provider,
			key.Model,
			key.Project,
			total.Requests,
		)
	}

	builder.WriteString("# HELP lcg_input_tokens_by_dimension_total Input tokens by tenant, provider, model, and project.\n")
	builder.WriteString("# TYPE lcg_input_tokens_by_dimension_total counter\n")
	for _, key := range keys {
		total := grouped[key]
		fmt.Fprintf(&builder,
			"lcg_input_tokens_by_dimension_total{tenant=%q,provider=%q,model=%q,project=%q} %d\n",
			key.Tenant,
			key.Provider,
			key.Model,
			key.Project,
			total.InputTokens,
		)
	}

	builder.WriteString("# HELP lcg_output_tokens_by_dimension_total Output tokens by tenant, provider, model, and project.\n")
	builder.WriteString("# TYPE lcg_output_tokens_by_dimension_total counter\n")
	for _, key := range keys {
		total := grouped[key]
		fmt.Fprintf(&builder,
			"lcg_output_tokens_by_dimension_total{tenant=%q,provider=%q,model=%q,project=%q} %d\n",
			key.Tenant,
			key.Provider,
			key.Model,
			key.Project,
			total.OutputTokens,
		)
	}

	builder.WriteString("# HELP lcg_cost_usd_by_dimension_total Cost in USD by tenant, provider, model, and project.\n")
	builder.WriteString("# TYPE lcg_cost_usd_by_dimension_total counter\n")
	for _, key := range keys {
		total := grouped[key]
		fmt.Fprintf(&builder,
			"lcg_cost_usd_by_dimension_total{tenant=%q,provider=%q,model=%q,project=%q} %.6f\n",
			key.Tenant,
			key.Provider,
			key.Model,
			key.Project,
			total.CostUSD,
		)
	}

	return builder.String()
}
