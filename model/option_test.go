package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionMapParsesAutoDisableFailureWindowSettings(t *testing.T) {
	oldThreshold := common.AutomaticDisableFailureThreshold
	oldWindow := common.AutomaticDisableFailureWindowSeconds
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.AutomaticDisableFailureThreshold = oldThreshold
		common.AutomaticDisableFailureWindowSeconds = oldWindow
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	require.NoError(t, updateOptionMap("AutomaticDisableFailureThreshold", "3"))
	require.Equal(t, 3, common.AutomaticDisableFailureThreshold)

	require.NoError(t, updateOptionMap("AutomaticDisableFailureWindowSeconds", "120"))
	require.Equal(t, 120, common.AutomaticDisableFailureWindowSeconds)
}

func TestUpdateOptionMapClampsAutoDisableFailureWindowSettings(t *testing.T) {
	oldThreshold := common.AutomaticDisableFailureThreshold
	oldWindow := common.AutomaticDisableFailureWindowSeconds
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.AutomaticDisableFailureThreshold = oldThreshold
		common.AutomaticDisableFailureWindowSeconds = oldWindow
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	require.NoError(t, updateOptionMap("AutomaticDisableFailureThreshold", "0"))
	require.Equal(t, 1, common.AutomaticDisableFailureThreshold)

	require.NoError(t, updateOptionMap("AutomaticDisableFailureWindowSeconds", "-5"))
	require.Equal(t, 1, common.AutomaticDisableFailureWindowSeconds)
}
