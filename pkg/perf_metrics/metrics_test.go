package perfmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	for i := int64(1); i <= 16; i++ {
		buckets[i] = counters{
			requestCount: 20,
			successCount: i,
		}
	}

	rates := recentSuccessRates(buckets, summaryRecentBucketLimit)

	assert.Len(t, rates, summaryRecentBucketLimit)
	assert.Equal(t, float64(15), rates[0])
	assert.Equal(t, float64(80), rates[len(rates)-1])
}
