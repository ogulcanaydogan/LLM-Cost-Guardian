package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/alerts"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
)

type rollupKey struct {
	Tenant   string
	Project  string
	Provider string
	Model    string
}

type usageAggregate struct {
	Tenant       string
	Project      string
	Provider     string
	Model        string
	RequestCount int64
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

type promptAggregate struct {
	count              int
	totalPromptChars   int
	totalPromptTokens  int64
	totalRatio         float64
	totalRepeatedRatio float64
	largeContextCount  int
	cachedContextCount int
	tenant             string
	project            string
	provider           string
	model              string
}

// DetectAnomalies returns tenant-aware daily spend anomalies.
func (t *UsageTracker) DetectAnomalies(ctx context.Context, filter ReportFilter) ([]UsageAnomaly, error) {
	start, end := analyticsWindow(filter, 30)
	rollups, err := t.storage.QueryUsageRollups(ctx, filter, "daily", start, end)
	if err != nil {
		return nil, err
	}

	grouped := make(map[rollupKey][]UsageRollup)
	for _, rollup := range rollups {
		key := rollupKey{
			Tenant:   rollup.Tenant,
			Project:  rollup.Project,
			Provider: rollup.Provider,
			Model:    rollup.Model,
		}
		grouped[key] = append(grouped[key], rollup)
	}

	var anomalies []UsageAnomaly
	for key, series := range grouped {
		if len(series) < 7 {
			continue
		}
		sort.Slice(series, func(i, j int) bool {
			return series[i].BucketStart.Before(series[j].BucketStart)
		})

		latest := series[len(series)-1]
		baselineValues := make([]float64, 0, len(series)-1)
		for _, item := range series[:len(series)-1] {
			baselineValues = append(baselineValues, item.CostUSD)
		}
		mean, stddev := stats(baselineValues)
		if mean == 0 && latest.CostUSD == 0 {
			continue
		}

		zScore := 0.0
		if stddev > 0 {
			zScore = (latest.CostUSD - mean) / stddev
		}

		severity := ""
		switch {
		case latest.CostUSD > mean*3 && latest.CostUSD > 0:
			severity = "critical"
		case latest.CostUSD > mean*2 && latest.CostUSD > 0:
			severity = "warning"
		case zScore >= 3:
			severity = "critical"
		case zScore >= 2:
			severity = "warning"
		}
		if severity == "" {
			continue
		}

		anomalies = append(anomalies, UsageAnomaly{
			Tenant:          key.Tenant,
			Provider:        key.Provider,
			Model:           key.Model,
			Project:         key.Project,
			Granularity:     latest.Granularity,
			BucketStart:     latest.BucketStart,
			ObservedCostUSD: latest.CostUSD,
			BaselineCostUSD: mean,
			ZScore:          zScore,
			Severity:        severity,
			Message: fmt.Sprintf("%s spend spike for %s/%s observed $%.4f vs baseline $%.4f",
				titleCase(severity), key.Provider, key.Model, latest.CostUSD, mean),
		})
	}

	sort.Slice(anomalies, func(i, j int) bool {
		if anomalies[i].Severity != anomalies[j].Severity {
			return anomalies[i].Severity > anomalies[j].Severity
		}
		return anomalies[i].ObservedCostUSD > anomalies[j].ObservedCostUSD
	})
	return anomalies, nil
}

// Forecast estimates 7-day and 30-day spend from daily rollups.
func (t *UsageTracker) Forecast(ctx context.Context, filter ReportFilter) ([]SpendForecast, error) {
	start, end := analyticsWindow(filter, 60)
	rollups, err := t.storage.QueryUsageRollups(ctx, filter, "daily", start, end)
	if err != nil {
		return nil, err
	}

	type projectKey struct {
		Tenant  string
		Project string
	}
	grouped := make(map[projectKey][]UsageRollup)
	for _, rollup := range rollups {
		key := projectKey{Tenant: rollup.Tenant, Project: rollup.Project}
		grouped[key] = append(grouped[key], rollup)
	}

	var forecasts []SpendForecast
	for key, series := range grouped {
		dailyTotals := collapseDailySeries(series)
		if len(dailyTotals) == 0 {
			continue
		}

		values := make([]float64, 0, len(dailyTotals))
		for _, item := range dailyTotals {
			values = append(values, item.CostUSD)
		}
		mean, _ := stats(values)
		ewma := computeEWMA(values, 0.4)
		slope := computeDailySlope(values)

		for _, horizon := range []int{7, 30} {
			total := 0.0
			for day := 1; day <= horizon; day++ {
				next := ewma + (slope * float64(day))
				if next < 0 {
					next = 0
				}
				total += next
			}

			forecasts = append(forecasts, SpendForecast{
				Tenant:              key.Tenant,
				Project:             key.Project,
				HorizonDays:         horizon,
				ForecastCostUSD:     total,
				AverageDailyCostUSD: mean,
				TrendDailyDeltaUSD:  slope,
				Confidence:          confidenceLabel(len(values)),
			})
		}
	}

	sort.Slice(forecasts, func(i, j int) bool {
		if forecasts[i].Tenant != forecasts[j].Tenant {
			return forecasts[i].Tenant < forecasts[j].Tenant
		}
		if forecasts[i].Project != forecasts[j].Project {
			return forecasts[i].Project < forecasts[j].Project
		}
		return forecasts[i].HorizonDays < forecasts[j].HorizonDays
	})
	return forecasts, nil
}

// RecommendModels suggests lower-cost alternatives within the same normalized model family.
func (t *UsageTracker) RecommendModels(ctx context.Context, filter ReportFilter) ([]ModelRecommendation, error) {
	start, end := analyticsWindow(filter, 30)
	filter.StartTime = start
	filter.EndTime = end

	records, err := t.storage.QueryUsage(ctx, filter)
	if err != nil {
		return nil, err
	}

	grouped := make(map[rollupKey]*usageAggregate)
	for _, record := range records {
		key := rollupKey{
			Tenant:   record.Tenant,
			Project:  record.Project,
			Provider: record.Provider,
			Model:    record.Model,
		}
		aggregate, ok := grouped[key]
		if !ok {
			aggregate = &usageAggregate{
				Tenant:   record.Tenant,
				Project:  record.Project,
				Provider: record.Provider,
				Model:    record.Model,
			}
			grouped[key] = aggregate
		}
		aggregate.RequestCount++
		aggregate.InputTokens += record.InputTokens
		aggregate.OutputTokens += record.OutputTokens
		aggregate.CostUSD += record.CostUSD
	}

	var recommendations []ModelRecommendation
	for _, aggregate := range grouped {
		if aggregate.RequestCount == 0 {
			continue
		}
		family := normalizeModelFamily(aggregate.Model)
		if family == "" {
			continue
		}

		avgInput := aggregate.InputTokens / aggregate.RequestCount
		avgOutput := aggregate.OutputTokens / aggregate.RequestCount
		currentCost, err := t.calculator.Calculate(aggregate.Provider, aggregate.Model, avgInput, avgOutput)
		if err != nil {
			continue
		}

		bestProvider := aggregate.Provider
		bestModel := aggregate.Model
		bestCost := currentCost

		for _, provider := range t.registry.All() {
			for _, candidate := range provider.Models() {
				if normalizeModelFamily(candidate.Model) != family {
					continue
				}
				cost, err := t.calculator.Calculate(provider.Name(), candidate.Model, avgInput, avgOutput)
				if err != nil {
					continue
				}
				if cost < bestCost {
					bestCost = cost
					bestProvider = provider.Name()
					bestModel = candidate.Model
				}
			}
		}

		if bestProvider == aggregate.Provider && bestModel == aggregate.Model {
			continue
		}

		savingsPerRequest := currentCost - bestCost
		totalSavings := savingsPerRequest * float64(aggregate.RequestCount)
		savingsPct := 0.0
		if currentCost > 0 {
			savingsPct = (savingsPerRequest / currentCost) * 100
		}

		recommendations = append(recommendations, ModelRecommendation{
			Tenant:              aggregate.Tenant,
			Project:             aggregate.Project,
			CurrentProvider:     aggregate.Provider,
			CurrentModel:        aggregate.Model,
			SuggestedProvider:   bestProvider,
			SuggestedModel:      bestModel,
			EstimatedSavingsUSD: totalSavings,
			EstimatedSavingsPct: savingsPct,
			Reason:              fmt.Sprintf("Normalized family %q has a lower-cost option for the observed token mix.", family),
		})
	}

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].EstimatedSavingsUSD > recommendations[j].EstimatedSavingsUSD
	})
	return recommendations, nil
}

// PromptOptimizations derives prompt-efficiency suggestions from stored metadata.
func (t *UsageTracker) PromptOptimizations(ctx context.Context, filter ReportFilter) ([]PromptOptimization, error) {
	start, end := analyticsWindow(filter, 30)
	filter.StartTime = start
	filter.EndTime = end

	records, err := t.storage.QueryUsage(ctx, filter)
	if err != nil {
		return nil, err
	}

	grouped := make(map[rollupKey]*promptAggregate)
	for _, record := range records {
		meta := parseUsageMetadata(record.Metadata)
		key := rollupKey{Tenant: record.Tenant, Project: record.Project, Provider: record.Provider, Model: record.Model}
		aggregate, ok := grouped[key]
		if !ok {
			aggregate = &promptAggregate{
				tenant:   record.Tenant,
				project:  record.Project,
				provider: record.Provider,
				model:    record.Model,
			}
			grouped[key] = aggregate
		}
		aggregate.count++
		aggregate.totalPromptChars += meta.PromptChars
		aggregate.totalPromptTokens += meta.PromptTokensEstimate
		aggregate.totalRatio += meta.InputOutputRatio
		aggregate.totalRepeatedRatio += meta.RepeatedLineRatio
		if meta.LargeStaticContext {
			aggregate.largeContextCount++
		}
		if meta.CachedContextCandidate {
			aggregate.cachedContextCount++
		}
	}

	var suggestions []PromptOptimization
	for _, aggregate := range grouped {
		if aggregate.count == 0 {
			continue
		}
		avgPromptChars := float64(aggregate.totalPromptChars) / float64(aggregate.count)
		avgPromptTokens := float64(aggregate.totalPromptTokens) / float64(aggregate.count)
		avgRatio := aggregate.totalRatio / float64(aggregate.count)
		avgRepeatedRatio := aggregate.totalRepeatedRatio / float64(aggregate.count)
		largeContextRatio := float64(aggregate.largeContextCount) / float64(aggregate.count)
		cachedContextRatio := float64(aggregate.cachedContextCount) / float64(aggregate.count)

		switch {
		case avgRatio >= 6:
			suggestions = append(suggestions, promptSuggestion(aggregate, "warning", "Reduce oversized prompts or split requests", fmt.Sprintf("Average input/output token ratio is %.2fx.", avgRatio), "High"))
		case avgPromptChars >= 6000 || avgPromptTokens >= 1500:
			suggestions = append(suggestions, promptSuggestion(aggregate, "warning", "Compress long static context", fmt.Sprintf("Average prompt length is %.0f chars / %.0f tokens.", avgPromptChars, avgPromptTokens), "High"))
		case avgRepeatedRatio >= 0.20:
			suggestions = append(suggestions, promptSuggestion(aggregate, "info", "Deduplicate repeated instructions", fmt.Sprintf("Repeated content ratio averages %.0f%%.", avgRepeatedRatio*100), "Medium"))
		case largeContextRatio >= 0.50 || cachedContextRatio >= 0.50:
			suggestions = append(suggestions, promptSuggestion(aggregate, "info", "Cache reusable context blocks", fmt.Sprintf("Reusable context detected in %.0f%% of sampled calls.", math.Max(largeContextRatio, cachedContextRatio)*100), "Medium"))
		}
	}

	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Severity != suggestions[j].Severity {
			return suggestions[i].Severity > suggestions[j].Severity
		}
		return suggestions[i].Provider < suggestions[j].Provider
	})
	return suggestions, nil
}

func (t *UsageTracker) maybeSendAnomalyAlert(ctx context.Context, record *UsageRecord) {
	if t.budget == nil || len(t.budget.notifiers) == 0 {
		return
	}

	filter := ReportFilter{
		Tenant:   record.Tenant,
		Provider: record.Provider,
		Model:    record.Model,
		Project:  record.Project,
	}
	anomalies, err := t.DetectAnomalies(ctx, filter)
	if err != nil || len(anomalies) == 0 {
		return
	}

	anomaly := anomalies[0]
	if anomaly.Severity != "critical" {
		return
	}

	alert := alerts.Alert{
		Kind:         "anomaly",
		Level:        alerts.AlertCritical,
		Tenant:       anomaly.Tenant,
		Project:      anomaly.Project,
		Provider:     anomaly.Provider,
		Model:        anomaly.Model,
		CurrentSpend: anomaly.ObservedCostUSD,
		LimitUSD:     anomaly.BaselineCostUSD,
		Message:      anomaly.Message,
	}
	for _, notifier := range t.budget.notifiers {
		if err := notifier.Send(ctx, alert); err != nil {
			t.logger.Error("send anomaly alert failed", "notifier", notifier.Name(), "error", err)
		}
	}
}

func analyticsWindow(filter ReportFilter, fallbackDays int) (time.Time, time.Time) {
	start := filter.StartTime
	end := filter.EndTime
	if end.IsZero() {
		end = time.Now().UTC().Add(24 * time.Hour)
	}
	if start.IsZero() {
		start = end.AddDate(0, 0, -fallbackDays)
	}
	return start, end
}

func stats(values []float64) (mean float64, stddev float64) {
	if len(values) == 0 {
		return 0, 0
	}
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))
	if len(values) == 1 {
		return mean, 0
	}
	var variance float64
	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}
	variance /= float64(len(values) - 1)
	return mean, math.Sqrt(variance)
}

func collapseDailySeries(series []UsageRollup) []UsageRollup {
	byDay := make(map[time.Time]UsageRollup)
	for _, item := range series {
		rollup := byDay[item.BucketStart]
		rollup.Tenant = item.Tenant
		rollup.Project = item.Project
		rollup.Granularity = item.Granularity
		rollup.BucketStart = item.BucketStart
		rollup.CostUSD += item.CostUSD
		rollup.InputTokens += item.InputTokens
		rollup.OutputTokens += item.OutputTokens
		rollup.RequestCount += item.RequestCount
		byDay[item.BucketStart] = rollup
	}

	result := make([]UsageRollup, 0, len(byDay))
	for _, item := range byDay {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].BucketStart.Before(result[j].BucketStart)
	})
	return result
}

func computeEWMA(values []float64, alpha float64) float64 {
	if len(values) == 0 {
		return 0
	}
	value := values[0]
	for _, item := range values[1:] {
		value = alpha*item + (1-alpha)*value
	}
	return value
}

func computeDailySlope(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	return (values[len(values)-1] - values[0]) / float64(len(values)-1)
}

func confidenceLabel(points int) string {
	switch {
	case points >= 21:
		return "high"
	case points >= 10:
		return "medium"
	default:
		return "low"
	}
}

func normalizeModelFamily(modelName string) string {
	name := strings.ToLower(modelName)
	switch {
	case strings.Contains(name, "gpt-4o"):
		return "gpt-4o"
	case strings.Contains(name, "gpt-4.1"):
		return "gpt-4.1"
	case strings.Contains(name, "sonnet"):
		return "sonnet"
	case strings.Contains(name, "haiku"):
		return "haiku"
	case strings.Contains(name, "opus"):
		return "opus"
	case strings.Contains(name, "gemini") && strings.Contains(name, "flash"):
		return "gemini-flash"
	case strings.Contains(name, "gemini") && strings.Contains(name, "pro"):
		return "gemini-pro"
	case strings.Contains(name, "nova-lite"):
		return "nova-lite"
	case strings.Contains(name, "nova-pro"):
		return "nova-pro"
	default:
		return ""
	}
}

func parseUsageMetadata(raw string) model.UsageMetadata {
	if strings.TrimSpace(raw) == "" {
		return model.UsageMetadata{}
	}
	var metadata model.UsageMetadata
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return model.UsageMetadata{}
	}
	return metadata
}

func promptSuggestion(aggregate *promptAggregate, severity, suggestion, evidence, impact string) PromptOptimization {
	return PromptOptimization{
		Tenant:          aggregate.tenant,
		Project:         aggregate.project,
		Provider:        aggregate.provider,
		Model:           aggregate.model,
		Severity:        severity,
		Suggestion:      suggestion,
		Evidence:        evidence,
		EstimatedImpact: impact,
		AverageRatio:    aggregate.totalRatio / float64(aggregate.count),
	}
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
