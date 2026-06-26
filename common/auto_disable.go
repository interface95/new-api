package common

func WarnAutomaticDisableFallbackIfNeeded(source string) {
	if !AutomaticDisableChannelEnabled || (RedisEnabled && RDB != nil) {
		return
	}
	SysError("automatic channel disable is using single-process memory fallback (" + source + "); configure Redis for shared multi-replica counting")
}
