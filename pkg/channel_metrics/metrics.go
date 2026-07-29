package channelmetrics

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/channel_metrics_setting"
)

var hotBuckets sync.Map

const summaryRecentBucketLimit = 24

func Init() {
	go flushLoop()
}

func RecordChannelSample(channelId int, success bool, latencyMs int64) {
	setting := channel_metrics_setting.GetSetting()
	if !setting.Enabled || channelId <= 0 {
		return
	}
	if latencyMs < 0 {
		latencyMs = 0
	}
	key := bucketKey{
		channelId: channelId,
		bucketTs:  bucketStart(time.Now().Unix()),
	}
	actual, _ := hotBuckets.LoadOrStore(key, &atomicBucket{})
	actual.(*atomicBucket).add(Sample{ChannelId: channelId, Success: success, LatencyMs: latencyMs})
}

func QueryChannelSummary(channelIds []int, hours int) (map[int]ChannelSummary, error) {
	if len(channelIds) == 0 || !channel_metrics_setting.GetSetting().Enabled {
		return map[int]ChannelSummary{}, nil
	}
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(hours)*3600

	idSet := make(map[int]struct{}, len(channelIds))
	uniqueIds := make([]int, 0, len(channelIds))
	for _, id := range channelIds {
		if id <= 0 {
			continue
		}
		if _, exists := idSet[id]; exists {
			continue
		}
		idSet[id] = struct{}{}
		uniqueIds = append(uniqueIds, id)
	}
	if len(uniqueIds) == 0 {
		return map[int]ChannelSummary{}, nil
	}

	rows, err := model.GetChannelMetricsSummaryBuckets(uniqueIds, startTs, endTs)
	if err != nil {
		return nil, err
	}

	channelBuckets := map[int]map[int64]counters{}
	for _, row := range rows {
		mergeChannelBucket(channelBuckets, row.ChannelId, row.BucketTs, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
		})
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if _, ok := idSet[k.channelId]; !ok {
			return true
		}
		snap := value.(*atomicBucket).snapshot()
		if snap.requestCount == 0 {
			return true
		}
		mergeChannelBucket(channelBuckets, k.channelId, k.bucketTs, snap)
		return true
	})

	return buildChannelSummaries(channelBuckets), nil
}

func buildChannelSummaries(channelBuckets map[int]map[int64]counters) map[int]ChannelSummary {
	out := make(map[int]ChannelSummary, len(channelBuckets))
	for id, buckets := range channelBuckets {
		var total counters
		for _, c := range buckets {
			total.requestCount += c.requestCount
			total.successCount += c.successCount
			total.totalLatencyMs += c.totalLatencyMs
		}
		if total.requestCount == 0 {
			continue
		}
		out[id] = ChannelSummary{
			SuccessRate:         math.Round(successRate(total)*100) / 100,
			RecentSuccessRates:  recentSuccessRates(buckets, summaryRecentBucketLimit),
			RecentBucketTs:      recentBucketTimestamps(buckets, summaryRecentBucketLimit),
			RecentSuccessCounts: recentSuccessCounts(buckets, summaryRecentBucketLimit),
			RecentFailureCounts: recentFailureCounts(buckets, summaryRecentBucketLimit),
			LatestBucketTs:      latestBucketTs(buckets),
			BucketSeconds:       channel_metrics_setting.GetBucketSeconds(),
		}
	}
	return out
}

func mergeChannelBucket(channelBuckets map[int]map[int64]counters, channelId int, bucketTs int64, value counters) {
	if value.requestCount == 0 {
		return
	}
	if _, ok := channelBuckets[channelId]; !ok {
		channelBuckets[channelId] = map[int64]counters{}
	}
	current := channelBuckets[channelId][bucketTs]
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	channelBuckets[channelId][bucketTs] = current
}

func recentSuccessRates(buckets map[int64]counters, limit int) []float64 {
	if len(buckets) == 0 || limit <= 0 {
		return nil
	}
	timestamps := make([]int64, 0, len(buckets))
	for ts := range buckets {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})
	if len(timestamps) > limit {
		timestamps = timestamps[len(timestamps)-limit:]
	}
	rates := make([]float64, 0, len(timestamps))
	for _, ts := range timestamps {
		rates = append(rates, math.Round(successRate(buckets[ts])*100)/100)
	}
	return rates
}

// recentBucketTimestamps returns the timestamps of the most recent buckets in
// the same order as recentSuccessRates, so the frontend can zip each status-bar
// segment to its bucket time for hover tooltips.
func recentBucketTimestamps(buckets map[int64]counters, limit int) []int64 {
	if len(buckets) == 0 || limit <= 0 {
		return nil
	}
	timestamps := make([]int64, 0, len(buckets))
	for ts := range buckets {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})
	if len(timestamps) > limit {
		timestamps = timestamps[len(timestamps)-limit:]
	}
	return timestamps
}

func recentSuccessCounts(buckets map[int64]counters, limit int) []int64 {
	timestamps := recentBucketTimestamps(buckets, limit)
	if len(timestamps) == 0 {
		return nil
	}
	counts := make([]int64, 0, len(timestamps))
	for _, ts := range timestamps {
		counts = append(counts, buckets[ts].successCount)
	}
	return counts
}

func recentFailureCounts(buckets map[int64]counters, limit int) []int64 {
	timestamps := recentBucketTimestamps(buckets, limit)
	if len(timestamps) == 0 {
		return nil
	}
	counts := make([]int64, 0, len(timestamps))
	for _, ts := range timestamps {
		value := buckets[ts]
		failed := value.requestCount - value.successCount
		if failed < 0 {
			failed = 0
		}
		counts = append(counts, failed)
	}
	return counts
}

func latestBucketTs(buckets map[int64]counters) int64 {
	var latest int64
	for ts, value := range buckets {
		if value.requestCount == 0 {
			continue
		}
		if ts > latest {
			latest = ts
		}
	}
	return latest
}

func successRate(value counters) float64 {
	if value.requestCount <= 0 {
		return 0
	}
	return float64(value.successCount) / float64(value.requestCount) * 100
}

func bucketStart(ts int64) int64 {
	bucketSeconds := channel_metrics_setting.GetBucketSeconds()
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	return ts - (ts % bucketSeconds)
}
