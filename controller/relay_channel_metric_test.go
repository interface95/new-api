package controller

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
)

func TestIsChannelSideError(t *testing.T) {
	assert.False(t, isChannelSideError(nil))

	ok := types.NewOpenAIError(errors.New("ok"), types.ErrorCodeBadResponseStatusCode, 200)
	assert.False(t, isChannelSideError(ok))

	weirdHigh := types.NewOpenAIError(errors.New("weird"), types.ErrorCodeBadResponseStatusCode, 600)
	assert.True(t, isChannelSideError(weirdHigh))
	weirdLow := types.NewOpenAIError(errors.New("weird"), types.ErrorCodeBadResponseStatusCode, 50)
	assert.True(t, isChannelSideError(weirdLow))

	upstream := types.NewOpenAIError(errors.New("bad gateway"), types.ErrorCodeBadResponseStatusCode, 502)
	assert.True(t, isChannelSideError(upstream))

	skipRetry := types.NewError(errors.New("bad body"), types.ErrorCodeBadResponseBody)
	assert.False(t, isChannelSideError(skipRetry))
}
