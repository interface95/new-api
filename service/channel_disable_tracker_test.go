package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withDisabledRedis(t *testing.T) {
	t.Helper()
	oldEnabled := common.RedisEnabled
	oldRDB := common.RDB
	oldTracker := channelAutoDisableFailures
	common.RedisEnabled = false
	common.RDB = nil
	channelAutoDisableFailures = newAutoDisableFailureTracker()
	t.Cleanup(func() {
		common.RedisEnabled = oldEnabled
		common.RDB = oldRDB
		channelAutoDisableFailures = oldTracker
	})
}

func TestAutoDisableFailureKeyHashesUsingKey(t *testing.T) {
	key := autoDisableFailureKey(42, "sk-secret-value")

	assert.True(t, strings.HasPrefix(key, "auto_disable_failure:channel:42:key:"))
	assert.NotContains(t, key, "sk-secret-value")
	assert.Len(t, strings.TrimPrefix(key, "auto_disable_failure:channel:42:key:"), 16)
}

func TestAutoDisableFailureKeySingleKeyChannel(t *testing.T) {
	assert.Equal(t, "auto_disable_failure:channel:42", autoDisableFailureKey(42, ""))
}

func TestShouldDisableByFailureCount(t *testing.T) {
	assert.False(t, shouldDisableByFailureCount(0, 1))
	assert.True(t, shouldDisableByFailureCount(1, 1))
	assert.False(t, shouldDisableByFailureCount(2, 3))
	assert.True(t, shouldDisableByFailureCount(3, 3))
	assert.True(t, shouldDisableByFailureCount(4, 3))
	assert.True(t, shouldDisableByFailureCount(1, 0))
}

func TestMemoryFallbackCountsConsecutiveFailuresAndExpiresByTTL(t *testing.T) {
	tracker := newAutoDisableFailureTracker()
	key := autoDisableFailureKey(7, "sk-a")
	now := time.Unix(1_800_000_000, 0)

	require.Equal(t, 1, tracker.recordMemory(key, now, time.Minute))
	require.Equal(t, 2, tracker.recordMemory(key, now.Add(30*time.Second), time.Minute))
	require.Equal(t, 1, tracker.recordMemory(key, now.Add(2*time.Minute), time.Minute))
}

func TestMemoryFallbackResetsAtExactTTLBoundary(t *testing.T) {
	tracker := newAutoDisableFailureTracker()
	key := autoDisableFailureKey(7, "sk-a")
	now := time.Unix(1_800_000_000, 0)

	require.Equal(t, 1, tracker.recordMemory(key, now, time.Minute))
	require.Equal(t, 1, tracker.recordMemory(key, now.Add(time.Minute), time.Minute))
}

func TestIncrChannelFailureUsesMemoryFallbackWhenRedisDisabled(t *testing.T) {
	withDisabledRedis(t)

	key := autoDisableFailureKey(8, "sk-b")
	count, backend := incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendMemory, backend)

	count, backend = incrChannelFailure(key, time.Minute)
	require.Equal(t, 2, count)
	require.Equal(t, autoDisableBackendMemory, backend)
}

func TestResetChannelFailureClearsMemoryFallback(t *testing.T) {
	withDisabledRedis(t)

	key := autoDisableFailureKey(9, "sk-c")
	count, backend := incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendMemory, backend)

	resetChannelFailure(key)

	count, backend = incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendMemory, backend)
}

func TestRecordChannelSuccessClearsContinuousFailureCounter(t *testing.T) {
	withDisabledRedis(t)

	key := autoDisableFailureKey(12, "sk-success")
	count, backend := incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendMemory, backend)

	RecordChannelSuccess(12, "sk-success")

	count, backend = incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendMemory, backend)
}

func TestRecordChannelAutoDisableFailureUsesConfiguredThreshold(t *testing.T) {
	withDisabledRedis(t)

	oldThreshold := common.AutomaticDisableFailureThreshold
	oldWindow := common.AutomaticDisableFailureWindowSeconds
	common.AutomaticDisableFailureThreshold = 2
	common.AutomaticDisableFailureWindowSeconds = 60
	t.Cleanup(func() {
		common.AutomaticDisableFailureThreshold = oldThreshold
		common.AutomaticDisableFailureWindowSeconds = oldWindow
	})

	channelError := *types.NewChannelError(13, 1, "threshold-test", false, "sk-threshold", true)

	decision := recordChannelAutoDisableFailure(channelError)
	require.False(t, decision.ShouldDisable)
	require.Equal(t, 1, decision.Count)
	require.Equal(t, 2, decision.Threshold)
	require.Equal(t, autoDisableBackendMemory, decision.Backend)

	decision = recordChannelAutoDisableFailure(channelError)
	require.True(t, decision.ShouldDisable)
	require.Equal(t, 2, decision.Count)
	require.Equal(t, time.Minute, decision.Window)
	require.Equal(t, autoDisableBackendMemory, decision.Backend)
}
