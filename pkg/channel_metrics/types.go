package channelmetrics

import "sync/atomic"

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
	requestCount   atomic.Int64
	successCount   atomic.Int64
	totalLatencyMs atomic.Int64
}

func (b *atomicBucket) add(sample Sample) {
	b.requestCount.Add(1)
	if sample.Success {
		b.successCount.Add(1)
	}
	if sample.LatencyMs > 0 {
		b.totalLatencyMs.Add(sample.LatencyMs)
	}
}

func (b *atomicBucket) snapshot() counters {
	return counters{
		requestCount:   b.requestCount.Load(),
		successCount:   b.successCount.Load(),
		totalLatencyMs: b.totalLatencyMs.Load(),
	}
}

func (b *atomicBucket) drain() counters {
	return counters{
		requestCount:   b.requestCount.Swap(0),
		successCount:   b.successCount.Swap(0),
		totalLatencyMs: b.totalLatencyMs.Swap(0),
	}
}

func (b *atomicBucket) addCounters(c counters) {
	if c.requestCount != 0 {
		b.requestCount.Add(c.requestCount)
	}
	if c.successCount != 0 {
		b.successCount.Add(c.successCount)
	}
	if c.totalLatencyMs != 0 {
		b.totalLatencyMs.Add(c.totalLatencyMs)
	}
}
