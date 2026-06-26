package common

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWarnAutomaticDisableFallbackIfNeededSuppressesRepeatedSameState(t *testing.T) {
	oldEnabled := AutomaticDisableChannelEnabled
	oldRedisEnabled := RedisEnabled
	oldRDB := RDB
	autoDisableFallbackWarningMu.Lock()
	oldWarnings := autoDisableFallbackWarningLast
	autoDisableFallbackWarningLast = map[string]autoDisableFallbackWarningCondition{}
	autoDisableFallbackWarningMu.Unlock()
	AutomaticDisableChannelEnabled = true
	RedisEnabled = false
	RDB = nil
	t.Cleanup(func() {
		AutomaticDisableChannelEnabled = oldEnabled
		RedisEnabled = oldRedisEnabled
		RDB = oldRDB
		autoDisableFallbackWarningMu.Lock()
		autoDisableFallbackWarningLast = oldWarnings
		autoDisableFallbackWarningMu.Unlock()
	})

	var output bytes.Buffer
	LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &output
	LogWriterMu.Unlock()
	t.Cleanup(func() {
		LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		LogWriterMu.Unlock()
	})

	WarnAutomaticDisableFallbackIfNeeded("option update")
	WarnAutomaticDisableFallbackIfNeeded("option update")

	require.Equal(t, 1, strings.Count(output.String(), "single-process memory fallback"))
}
