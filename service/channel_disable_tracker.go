package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	autoDisableBackendRedis  = "redis"
	autoDisableBackendMemory = "memory"
)

type autoDisableDecision struct {
	ShouldDisable bool
	Count         int
	Threshold     int
	Window        time.Duration
	Backend       string
}

type autoDisableMemoryRecord struct {
	Count         int
	LastFailureAt time.Time
}

type autoDisableFailureTracker struct {
	mu      sync.Mutex
	records map[string]autoDisableMemoryRecord
}

func newAutoDisableFailureTracker() *autoDisableFailureTracker {
	return &autoDisableFailureTracker{
		records: make(map[string]autoDisableMemoryRecord),
	}
}

func autoDisableFailureKey(channelId int, usingKey string) string {
	if usingKey == "" {
		return fmt.Sprintf("auto_disable_failure:channel:%d", channelId)
	}
	sum := sha256.Sum256([]byte(usingKey))
	keyHash := hex.EncodeToString(sum[:])[:16]
	return fmt.Sprintf("auto_disable_failure:channel:%d:key:%s", channelId, keyHash)
}

func normalizeAutoDisablePolicy(threshold int, windowSeconds int) (int, time.Duration) {
	if threshold < 1 {
		threshold = 1
	}
	if windowSeconds < 1 {
		windowSeconds = 1
	}
	return threshold, time.Duration(windowSeconds) * time.Second
}

func shouldDisableByFailureCount(count, threshold int) bool {
	if threshold < 1 {
		threshold = 1
	}
	return count >= threshold
}

func incrChannelFailure(key string, ttl time.Duration) (int, string) {
	if ttl <= 0 {
		ttl = time.Second
	}
	ttlSeconds := int(ttl.Seconds())
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}

	if common.RedisEnabled && common.RDB != nil {
		count, err := common.RedisIncrWithTTL(key, ttlSeconds)
		if err == nil {
			channelAutoDisableFailures.clear(key)
			return int(count), autoDisableBackendRedis
		}
		common.SysError(fmt.Sprintf("auto-disable redis counter failed: key=%s, err=%s; using memory fallback", key, err.Error()))
	}

	return channelAutoDisableFailures.recordMemory(key, time.Now(), ttl), autoDisableBackendMemory
}

func resetChannelFailure(key string) {
	channelAutoDisableFailures.clear(key)
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	if err := common.RedisDel(key); err != nil {
		common.SysError(fmt.Sprintf("auto-disable redis counter reset failed: key=%s, err=%s", key, err.Error()))
	}
}

func (tracker *autoDisableFailureTracker) recordMemory(key string, now time.Time, ttl time.Duration) int {
	if ttl <= 0 {
		ttl = time.Second
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	record := tracker.records[key]
	if record.LastFailureAt.IsZero() || now.Sub(record.LastFailureAt) >= ttl {
		record.Count = 0
	}
	record.Count++
	record.LastFailureAt = now
	tracker.records[key] = record
	return record.Count
}

func (tracker *autoDisableFailureTracker) clear(key string) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	delete(tracker.records, key)
}
