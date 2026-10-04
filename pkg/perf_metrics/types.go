package perfmetrics

import "sync"

type Store interface {
	Record(sample Sample)
	Query(params QueryParams) (QueryResult, error)
}

type Sample struct {
	Model        string
	Group        string
	LatencyMs    int64
	TtftMs       int64
	HasTtft      bool
	Success      bool
	OutputTokens int64
	GenerationMs int64
}

type QueryParams struct {
	Model string
	Group string
	Hours int
	// AllowedGroups restricts both the per-group results and the model summary;
	// nil allows every group.
	AllowedGroups []string
}

// Summary is the request-weighted aggregate over every bucket in the window.
type Summary struct {
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

type BucketPoint struct {
	Ts           int64   `json:"ts"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

type GroupResult struct {
	Group        string        `json:"group"`
	AvgTtftMs    int64         `json:"avg_ttft_ms"`
	AvgLatencyMs int64         `json:"avg_latency_ms"`
	SuccessRate  float64       `json:"success_rate"`
	AvgTps       float64       `json:"avg_tps"`
	Series       []BucketPoint `json:"series"`
}

type QueryResult struct {
	ModelName    string        `json:"model_name"`
	SeriesSchema string        `json:"series_schema"`
	Summary      *Summary      `json:"summary"`
	Series       []BucketPoint `json:"series"`
	WindowStart  int64         `json:"window_start"`
	WindowEnd    int64         `json:"window_end"`
	Groups       []GroupResult `json:"groups"`
}

type SuccessRatePoint struct {
	Ts          int64   `json:"ts"`
	SuccessRate float64 `json:"success_rate"`
}

type ModelSummary struct {
	ModelName           string             `json:"model_name"`
	AvgLatencyMs        int64              `json:"avg_latency_ms"`
	SuccessRate         float64            `json:"success_rate"`
	AvgTps              float64            `json:"avg_tps"`
	RecentSuccessRates  []float64          `json:"recent_success_rates,omitempty"`
	RecentBucketTs      []int64            `json:"recent_bucket_ts,omitempty"`
	RecentSuccessCounts []int64            `json:"recent_success_counts,omitempty"`
	RecentFailureCounts []int64            `json:"recent_failure_counts,omitempty"`
	LatestBucketTs      int64              `json:"latest_bucket_ts,omitempty"`
	MetricBucketSeconds int64              `json:"metric_bucket_seconds,omitempty"`
	RequestCount        int64              `json:"-"`
	RecentSuccessSeries []SuccessRatePoint `json:"recent_success_series,omitempty"`
}

type SummaryAllResult struct {
	Summary     *Summary       `json:"summary"`
	WindowStart int64          `json:"window_start"`
	WindowEnd   int64          `json:"window_end"`
	Models      []ModelSummary `json:"models"`
}

type bucketKey struct {
	model    string
	group    string
	bucketTs int64
}

type counters struct {
	requestCount   int64
	successCount   int64
	totalLatencyMs int64
	ttftSumMs      int64
	ttftCount      int64
	outputTokens   int64
	generationMs   int64
}

type atomicBucket struct {
	mu       sync.Mutex
	counters counters
}

func (b *atomicBucket) add(sample Sample) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.counters.requestCount++
	if sample.Success {
		b.counters.successCount++
	}
	if sample.LatencyMs > 0 {
		b.counters.totalLatencyMs += sample.LatencyMs
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		b.counters.ttftSumMs += sample.TtftMs
		b.counters.ttftCount++
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		b.counters.outputTokens += sample.OutputTokens
		b.counters.generationMs += sample.GenerationMs
	}
}

func (b *atomicBucket) snapshot() counters {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.counters
}

func (b *atomicBucket) drain() counters {
	b.mu.Lock()
	defer b.mu.Unlock()

	drained := b.counters
	b.counters = counters{}
	return drained
}

func (b *atomicBucket) addCounters(c counters) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.counters.requestCount += c.requestCount
	b.counters.successCount += c.successCount
	b.counters.totalLatencyMs += c.totalLatencyMs
	b.counters.ttftSumMs += c.ttftSumMs
	b.counters.ttftCount += c.ttftCount
	b.counters.outputTokens += c.outputTokens
	b.counters.generationMs += c.generationMs
}
