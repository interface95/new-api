package common

import "sync"

type autoDisableFallbackWarningCondition struct {
	Enabled        bool
	RedisAvailable bool
}

var (
	autoDisableFallbackWarningMu   sync.Mutex
	autoDisableFallbackWarningLast = map[string]autoDisableFallbackWarningCondition{}
)

func WarnAutomaticDisableFallbackIfNeeded(source string) {
	condition := autoDisableFallbackWarningCondition{
		Enabled:        AutomaticDisableChannelEnabled,
		RedisAvailable: RedisEnabled && RDB != nil,
	}

	autoDisableFallbackWarningMu.Lock()
	lastCondition, ok := autoDisableFallbackWarningLast[source]
	if ok && lastCondition == condition {
		autoDisableFallbackWarningMu.Unlock()
		return
	}
	autoDisableFallbackWarningLast[source] = condition
	autoDisableFallbackWarningMu.Unlock()

	if !condition.Enabled || condition.RedisAvailable {
		return
	}

	SysError("automatic channel disable is using single-process memory fallback (" + source + "); configure Redis for shared multi-replica counting")
}
