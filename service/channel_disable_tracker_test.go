package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
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

func withServiceMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	server, err := miniredis.Run()
	require.NoError(t, err)

	oldEnabled := common.RedisEnabled
	oldRDB := common.RDB
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	common.RDB = client
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		common.RDB = oldRDB
		common.RedisEnabled = oldEnabled
		server.Close()
	})

	return server
}

func withAutoDisablePolicy(t *testing.T, enabled bool, threshold int) {
	t.Helper()
	oldEnabled := common.AutomaticDisableChannelEnabled
	oldThreshold := common.AutomaticDisableFailureThreshold
	common.AutomaticDisableChannelEnabled = enabled
	common.AutomaticDisableFailureThreshold = threshold
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = oldEnabled
		common.AutomaticDisableFailureThreshold = oldThreshold
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

func TestIncrChannelFailureUsesRedisWhenAvailable(t *testing.T) {
	server := withServiceMiniRedis(t)

	key := autoDisableFailureKey(18, "sk-redis")
	count, backend := incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendRedis, backend)
	require.True(t, server.Exists(key))

	count, backend = incrChannelFailure(key, time.Minute)
	require.Equal(t, 2, count)
	require.Equal(t, autoDisableBackendRedis, backend)
	require.Greater(t, server.TTL(key), 55*time.Second)
}

func TestIncrChannelFailureFallsBackToMemoryWhenRedisFails(t *testing.T) {
	server := withServiceMiniRedis(t)
	server.Close()

	key := autoDisableFailureKey(19, "sk-redis-down")
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

func TestResetChannelFailureClearsRedisCounter(t *testing.T) {
	server := withServiceMiniRedis(t)

	key := autoDisableFailureKey(20, "sk-reset-redis")
	count, backend := incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendRedis, backend)

	resetChannelFailure(key)

	require.False(t, server.Exists(key))
}

func TestRecordChannelSuccessClearsContinuousFailureCounter(t *testing.T) {
	withDisabledRedis(t)
	withAutoDisablePolicy(t, true, 2)

	key := autoDisableFailureKey(12, "sk-success")
	count, backend := incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendMemory, backend)

	RecordChannelSuccess(12, "sk-success")

	count, backend = incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendMemory, backend)
}

func TestRecordChannelSuccessSkipsResetWhenContinuousPolicyInactive(t *testing.T) {
	server := withServiceMiniRedis(t)

	withAutoDisablePolicy(t, false, 2)
	key := autoDisableFailureKey(14, "sk-disabled")
	count, err := common.RedisIncrWithTTL(key, 60)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	RecordChannelSuccess(14, "sk-disabled")

	require.True(t, server.Exists(key))

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableFailureThreshold = 1
	key = autoDisableFailureKey(15, "sk-threshold-one")
	count, err = common.RedisIncrWithTTL(key, 60)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	RecordChannelSuccess(15, "sk-threshold-one")

	require.True(t, server.Exists(key))
}

func TestDisableChannelNowClearsRedisFailureCounter(t *testing.T) {
	truncate(t)
	server := withServiceMiniRedis(t)
	require.NoError(t, model.DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM users").Error)
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{}))
	require.NoError(t, model.DB.Create(&model.User{
		Id:       100,
		Username: "root",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "root-reset",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id:     21,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-channel",
		Status: common.ChannelStatusEnabled,
		Name:   "redis-reset-channel",
	}).Error)

	key := autoDisableFailureKey(21, "sk-disable")
	count, backend := incrChannelFailure(key, time.Minute)
	require.Equal(t, 1, count)
	require.Equal(t, autoDisableBackendRedis, backend)

	disabled := disableChannelNow(*types.NewChannelError(21, constant.ChannelTypeOpenAI, "redis-reset-channel", false, "sk-disable", true), "test disable")

	require.True(t, disabled)
	require.False(t, server.Exists(key))
}

func TestDisableChannelReportsWhenThresholdNotReached(t *testing.T) {
	withDisabledRedis(t)
	withAutoDisablePolicy(t, true, 2)

	disabled := DisableChannel(
		*types.NewChannelError(22, constant.ChannelTypeOpenAI, "threshold-test", false, "sk-threshold", true),
		"test failure",
	)

	require.False(t, disabled)
}

func TestDisableChannelNowReportsFailedStatusUpdate(t *testing.T) {
	truncate(t)

	disabled := disableChannelNow(
		*types.NewChannelError(999999, constant.ChannelTypeOpenAI, "missing-channel", false, "", true),
		"test failure",
	)

	require.False(t, disabled)
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
