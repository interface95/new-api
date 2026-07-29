package channel_metrics_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultBucketTimeIsMinute(t *testing.T) {
	assert.Equal(t, "minute", GetSetting().BucketTime)
	assert.Equal(t, int64(60), GetBucketSeconds())
}
