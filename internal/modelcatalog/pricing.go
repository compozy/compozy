package modelcatalog

import "math"

const costCurrencyUSD = "USD"

// TokenBuckets carries the usage buckets priced by the merged model catalog.
type TokenBuckets struct {
	Input      *int64
	Output     *int64
	CacheRead  *int64
	CacheWrite *int64
	Reasoning  *int64
}

// CostStatus identifies how AGH obtained a cost result.
type CostStatus string

const (
	CostStatusActual    CostStatus = "actual"
	CostStatusEstimated CostStatus = "estimated"
	CostStatusIncluded  CostStatus = "included"
	CostStatusUnknown   CostStatus = "unknown"
)

// CostSource identifies the authority behind a cost result.
type CostSource string

const (
	CostSourceAgentReported CostSource = "agent_reported"
	CostSourceCatalogConfig CostSource = "catalog_config"
	CostSourceModelsDev     CostSource = "models_dev"
	CostSourceBuiltin       CostSource = "builtin"
	CostSourceNone          CostSource = "none"
)

// CostStatusValues returns the closed public cost-status enum.
func CostStatusValues() []string {
	return []string{
		string(CostStatusActual),
		string(CostStatusEstimated),
		string(CostStatusIncluded),
		string(CostStatusUnknown),
	}
}

// CostSourceValues returns the closed public cost-source enum.
func CostSourceValues() []string {
	return []string{
		string(CostSourceAgentReported),
		string(CostSourceCatalogConfig),
		string(CostSourceModelsDev),
		string(CostSourceBuiltin),
		string(CostSourceNone),
	}
}

// CostResult is one truthful monetary projection for a usage update.
type CostResult struct {
	Amount   float64
	Currency string
	Status   CostStatus
	Source   CostSource
}

// EstimateCost joins usage with independent merged catalog rates for every token bucket.
func EstimateCost(model *Model, usage TokenBuckets) (CostResult, bool) {
	if model == nil {
		return CostResult{}, false
	}
	buckets := []usagePriceBucket{
		{tokens: usage.Input, rate: model.CostInputPerMillion, source: model.CostInputSource},
		{tokens: usage.Output, rate: model.CostOutputPerMillion, source: model.CostOutputSource},
		{tokens: usage.CacheRead, rate: model.CostCacheReadPerMillion, source: model.CostCacheReadSource},
		{tokens: usage.CacheWrite, rate: model.CostCacheWritePerMillion, source: model.CostCacheWriteSource},
		{tokens: usage.Reasoning, rate: model.CostReasoningPerMillion, source: model.CostReasoningSource},
	}
	amount, source, ok := priceUsageBuckets(buckets)
	if !ok {
		return CostResult{}, false
	}
	return CostResult{
		Amount:   amount,
		Currency: costCurrencyUSD,
		Status:   CostStatusEstimated,
		Source:   source,
	}, true
}

type usagePriceBucket struct {
	tokens *int64
	rate   *float64
	source SourceKind
}

type pricedUsageBucket struct {
	tokens int64
	source CostSource
}

func priceUsageBuckets(buckets []usagePriceBucket) (float64, CostSource, bool) {
	priced := make([]pricedUsageBucket, 0, len(buckets))
	var amount float64
	for _, bucket := range buckets {
		tokens, ok := usageBucketTokens(bucket.tokens)
		if !ok {
			return 0, CostSourceNone, false
		}
		cost, source, ok := priceUsageBucket(tokens, bucket.rate, bucket.source)
		if !ok {
			return 0, CostSourceNone, false
		}
		amount += cost
		if math.IsNaN(amount) || math.IsInf(amount, 0) {
			return 0, CostSourceNone, false
		}
		priced = append(priced, pricedUsageBucket{tokens: tokens, source: source})
	}
	source, ok := usageCostSource(priced)
	return amount, source, ok
}

func usageBucketTokens(value *int64) (int64, bool) {
	if value == nil {
		return 0, true
	}
	return *value, *value >= 0
}

func priceUsageBucket(tokens int64, rate *float64, kind SourceKind) (float64, CostSource, bool) {
	if tokens == 0 {
		if rate == nil {
			return 0, CostSourceNone, true
		}
		source, ok := costSourceForKind(kind)
		return 0, source, ok
	}
	if rate == nil || math.IsNaN(*rate) || math.IsInf(*rate, 0) || *rate < 0 {
		return 0, CostSourceNone, false
	}
	source, ok := costSourceForKind(kind)
	if !ok {
		return 0, CostSourceNone, false
	}
	cost := float64(tokens) * *rate / 1_000_000
	if math.IsNaN(cost) || math.IsInf(cost, 0) {
		return 0, CostSourceNone, false
	}
	return cost, source, true
}

func usageCostSource(buckets []pricedUsageBucket) (CostSource, bool) {
	active := false
	for _, bucket := range buckets {
		active = active || bucket.tokens > 0
	}
	source := CostSourceNone
	for _, bucket := range buckets {
		if active && bucket.tokens == 0 {
			continue
		}
		var ok bool
		source, ok = compatibleCostSource(source, bucket.source)
		if !ok {
			return CostSourceNone, false
		}
	}
	return source, source != CostSourceNone
}

func compatibleCostSource(left, right CostSource) (CostSource, bool) {
	switch {
	case left == CostSourceNone:
		return right, right != CostSourceNone
	case right == CostSourceNone:
		return left, true
	case left == right:
		return left, true
	default:
		return CostSourceNone, false
	}
}

func costSourceForKind(kind SourceKind) (CostSource, bool) {
	switch kind {
	case SourceKindConfig, SourceKindProviderLive, SourceKindExtension, SourceKindACPSession:
		return CostSourceCatalogConfig, true
	case SourceKindModelsDev:
		return CostSourceModelsDev, true
	case SourceKindBuiltin:
		return CostSourceBuiltin, true
	default:
		return CostSourceNone, false
	}
}
