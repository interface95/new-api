package perfmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestBucketTsIgnoresEmptyBuckets(t *testing.T) {
	buckets := map[int64]counters{
		100: {requestCount: 1},
		200: {},
		300: {requestCount: 2},
	}

	assert.Equal(t, int64(300), latestBucketTs(buckets))
}

func TestRecentSuccessRatesKeepsLatestLimit(t *testing.T) {
	buckets := map[int64]counters{}
	total := int64(summaryRecentBucketLimit + 5)
	for i := int64(1); i <= total; i++ {
		buckets[i] = counters{requestCount: 100, successCount: i}
	}

	rates := recentSuccessRates(buckets, summaryRecentBucketLimit)

	assert.Len(t, rates, summaryRecentBucketLimit)
	// Keeps the most recent `limit` buckets (ts total-limit+1 .. total); with
	// successCount=i and requestCount=100 the rate equals the bucket ts.
	assert.Equal(t, float64(total-int64(summaryRecentBucketLimit)+1), rates[0])
	assert.Equal(t, float64(total), rates[len(rates)-1])
}

func TestRecentBucketTimestampsAlignWithRates(t *testing.T) {
	buckets := map[int64]counters{}
	total := int64(summaryRecentBucketLimit + 5)
	for i := int64(1); i <= total; i++ {
		buckets[i] = counters{requestCount: 100, successCount: i}
	}

	rates := recentSuccessRates(buckets, summaryRecentBucketLimit)
	ts := recentBucketTimestamps(buckets, summaryRecentBucketLimit)

	// The frontend zips ts[i] to rates[i]; they must be the same length and order.
	require.Len(t, ts, len(rates))
	assert.Equal(t, total-int64(summaryRecentBucketLimit)+1, ts[0])
	assert.Equal(t, total, ts[len(ts)-1])
	for i := range ts {
		assert.Equal(t, float64(ts[i]), rates[i])
	}
}
