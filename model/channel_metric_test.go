package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetChannelMetricsSummaryBucketsFiltersChannelIDs(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&ChannelMetric{}))
	t.Cleanup(func() {
		DB.Exec("DELETE FROM channel_metrics")
	})

	rows := []*ChannelMetric{
		{ChannelId: 1, BucketTs: 100, RequestCount: 2, SuccessCount: 1, TotalLatencyMs: 20},
		{ChannelId: 2, BucketTs: 100, RequestCount: 3, SuccessCount: 3, TotalLatencyMs: 30},
		{ChannelId: 3, BucketTs: 200, RequestCount: 4, SuccessCount: 2, TotalLatencyMs: 40},
	}
	for _, row := range rows {
		require.NoError(t, DB.Create(row).Error)
	}

	got, err := GetChannelMetricsSummaryBuckets([]int{1, 3}, 0, 300)

	require.NoError(t, err)
	require.Len(t, got, 2)
	channelIDs := []int{got[0].ChannelId, got[1].ChannelId}
	assert.ElementsMatch(t, []int{1, 3}, channelIDs)
}

func TestUpsertChannelMetricIncrementsExistingBucket(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&ChannelMetric{}))
	t.Cleanup(func() {
		DB.Exec("DELETE FROM channel_metrics")
	})

	require.NoError(t, UpsertChannelMetric(&ChannelMetric{
		ChannelId:      9,
		BucketTs:       100,
		RequestCount:   1,
		SuccessCount:   1,
		TotalLatencyMs: 10,
	}))
	require.NoError(t, UpsertChannelMetric(&ChannelMetric{
		ChannelId:      9,
		BucketTs:       100,
		RequestCount:   2,
		SuccessCount:   1,
		TotalLatencyMs: 20,
	}))

	var row ChannelMetric
	require.NoError(t, DB.Where("channel_id = ? AND bucket_ts = ?", 9, 100).First(&row).Error)
	assert.Equal(t, int64(3), row.RequestCount)
	assert.Equal(t, int64(2), row.SuccessCount)
	assert.Equal(t, int64(30), row.TotalLatencyMs)
}
