package channelmetrics

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildChannelSummariesComputesRateAndRecent(t *testing.T) {
	channelBuckets := map[int]map[int64]counters{
		7: {
			100: {requestCount: 2, successCount: 1, totalLatencyMs: 40},
			200: {requestCount: 3, successCount: 3, totalLatencyMs: 30},
		},
		9: {
			300: {requestCount: 0, successCount: 0},
		},
	}

	out := buildChannelSummaries(channelBuckets)

	require.Contains(t, out, 7)
	assert.NotContains(t, out, 9)
	assert.Equal(t, float64(80), out[7].SuccessRate)
	assert.Equal(t, []float64{50, 100}, out[7].RecentSuccessRates)
	assert.Equal(t, []int64{1, 3}, out[7].RecentSuccessCounts)
	assert.Equal(t, []int64{1, 0}, out[7].RecentFailureCounts)
	assert.Equal(t, int64(200), out[7].LatestBucketTs)
}

func TestRecentSuccessRatesKeepsLatestLimit(t *testing.T) {
	buckets := map[int64]counters{}
	total := int64(summaryRecentBucketLimit + 5)
	for i := int64(1); i <= total; i++ {
		buckets[i] = counters{requestCount: 100, successCount: i}
	}

	rates := recentSuccessRates(buckets, summaryRecentBucketLimit)

	assert.Len(t, rates, summaryRecentBucketLimit)
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

func TestLatestBucketTsIgnoresEmptyBuckets(t *testing.T) {
	buckets := map[int64]counters{
		100: {requestCount: 1},
		200: {},
		300: {requestCount: 2},
	}

	assert.Equal(t, int64(300), latestBucketTs(buckets))
}

func TestSuccessRateZeroRequests(t *testing.T) {
	assert.Equal(t, float64(0), successRate(counters{}))
	assert.Equal(t, float64(50), successRate(counters{requestCount: 4, successCount: 2}))
}

func TestQueryChannelSummaryReturnsEmptyWhenDisabled(t *testing.T) {
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"channel_metrics_setting.enabled": "false",
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
			"channel_metrics_setting.enabled": "true",
		}))
	})

	got, err := QueryChannelSummary([]int{1}, 24)

	require.NoError(t, err)
	assert.Empty(t, got)
}
