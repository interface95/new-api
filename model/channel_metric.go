package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChannelMetric stores aggregated per-channel relay success/failure metrics
// for channel status bars. Samples are per attempt and only include channel-side errors.
type ChannelMetric struct {
	Id             int   `json:"id" gorm:"primaryKey"`
	ChannelId      int   `json:"channel_id" gorm:"uniqueIndex:idx_channel_metric_channel_bucket,priority:1"`
	BucketTs       int64 `json:"bucket_ts" gorm:"uniqueIndex:idx_channel_metric_channel_bucket,priority:2;index:idx_channel_metric_bucket_ts"`
	RequestCount   int64 `json:"-" gorm:"default:0"`
	SuccessCount   int64 `json:"-" gorm:"default:0"`
	TotalLatencyMs int64 `json:"-" gorm:"default:0"`
}

func (ChannelMetric) TableName() string {
	return "channel_metrics"
}

func UpsertChannelMetric(metric *ChannelMetric) error {
	if metric == nil || metric.RequestCount == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "channel_id"},
			{Name: "bucket_ts"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"request_count":    gorm.Expr("channel_metrics.request_count + ?", metric.RequestCount),
			"success_count":    gorm.Expr("channel_metrics.success_count + ?", metric.SuccessCount),
			"total_latency_ms": gorm.Expr("channel_metrics.total_latency_ms + ?", metric.TotalLatencyMs),
		}),
	}).Create(metric).Error
}

type ChannelMetricSummaryBucket struct {
	ChannelId      int   `json:"channel_id"`
	BucketTs       int64 `json:"bucket_ts"`
	RequestCount   int64 `json:"request_count"`
	SuccessCount   int64 `json:"success_count"`
	TotalLatencyMs int64 `json:"total_latency_ms"`
}

func GetChannelMetricsSummaryBuckets(channelIds []int, startTs int64, endTs int64) ([]ChannelMetricSummaryBucket, error) {
	var summaries []ChannelMetricSummaryBucket
	if len(channelIds) == 0 {
		return summaries, nil
	}
	err := DB.Model(&ChannelMetric{}).
		Select("channel_id, bucket_ts, SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms").
		Where("channel_id IN ? AND bucket_ts >= ? AND bucket_ts <= ?", channelIds, startTs, endTs).
		Group("channel_id, bucket_ts").
		Having("SUM(request_count) > 0").
		Order("bucket_ts ASC").
		Find(&summaries).Error
	return summaries, err
}

func DeleteChannelMetricsBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&ChannelMetric{}).Error
}
