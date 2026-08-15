package channel_metrics_setting

import "github.com/QuantumNous/new-api/setting/config"

type ChannelMetricsSetting struct {
	Enabled       bool   `json:"enabled"`
	FlushInterval int    `json:"flush_interval"`
	BucketTime    string `json:"bucket_time"`
	RetentionDays int    `json:"retention_days"`
}

var channelMetricsSetting = ChannelMetricsSetting{
	Enabled:       true,
	FlushInterval: 5,
	BucketTime:    "minute",
	RetentionDays: 0,
}

func init() {
	config.GlobalConfig.Register("channel_metrics_setting", &channelMetricsSetting)
}

func GetSetting() ChannelMetricsSetting {
	return channelMetricsSetting
}

func GetBucketSeconds() int64 {
	switch channelMetricsSetting.BucketTime {
	case "minute":
		return 60
	case "5min":
		return 300
	case "hour":
		return 3600
	default:
		return 3600
	}
}

func GetFlushIntervalMinutes() int {
	if channelMetricsSetting.FlushInterval < 1 {
		return 1
	}
	return channelMetricsSetting.FlushInterval
}
