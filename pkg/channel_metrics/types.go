package channelmetrics

import "sync"

type Sample struct {
	ChannelId int
	Success   bool
	LatencyMs int64
}

// ChannelSummary is the per-channel aggregate consumed by channel status bars.
type ChannelSummary struct {
	SuccessRate         float64
	RecentSuccessRates  []float64
	RecentBucketTs      []int64
	RecentSuccessCounts []int64
	RecentFailureCounts []int64
	LatestBucketTs      int64
	BucketSeconds       int64
}

type bucketKey struct {
	channelId int
	bucketTs  int64
}

type counters struct {
	requestCount   int64
	successCount   int64
	totalLatencyMs int64
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
}
