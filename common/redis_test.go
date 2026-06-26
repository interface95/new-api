package common

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func withMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	server, err := miniredis.Run()
	require.NoError(t, err)

	oldEnabled := RedisEnabled
	oldRDB := RDB
	RedisEnabled = true
	RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() {
		if RDB != nil {
			require.NoError(t, RDB.Close())
		}
		RDB = oldRDB
		RedisEnabled = oldEnabled
		server.Close()
	})

	return server
}

func TestRedisIncrWithTTLRefreshesExpiry(t *testing.T) {
	server := withMiniRedis(t)

	count, err := RedisIncrWithTTL("auto-disable:test", 60)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	server.FastForward(30 * time.Second)

	count, err = RedisIncrWithTTL("auto-disable:test", 60)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	ttl := server.TTL("auto-disable:test")
	require.Greater(t, ttl, 55*time.Second)
	require.LessOrEqual(t, ttl, 60*time.Second)
}

func TestRedisIncrWithTTLReturnsErrorWhenRedisDisabled(t *testing.T) {
	oldEnabled := RedisEnabled
	oldRDB := RDB
	RedisEnabled = false
	RDB = nil
	t.Cleanup(func() {
		RedisEnabled = oldEnabled
		RDB = oldRDB
	})

	count, err := RedisIncrWithTTL("auto-disable:test", 60)
	require.Error(t, err)
	require.Equal(t, int64(0), count)
}
