package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

var channelAutoDisableFailures = newAutoDisableFailureTracker()

func recordChannelAutoDisableFailure(channelError types.ChannelError) autoDisableDecision {
	threshold, window := normalizeAutoDisablePolicy(
		common.AutomaticDisableFailureThreshold,
		common.AutomaticDisableFailureWindowSeconds,
	)
	count, backend := incrChannelFailure(
		autoDisableFailureKey(channelError.ChannelId, channelError.UsingKey),
		window,
	)
	return autoDisableDecision{
		ShouldDisable: shouldDisableByFailureCount(count, threshold),
		Count:         count,
		Threshold:     threshold,
		Window:        window,
		Backend:       backend,
	}
}

func RecordChannelSuccess(channelId int, usingKey string) {
	if !common.AutomaticDisableChannelEnabled || common.AutomaticDisableFailureThreshold <= 1 {
		return
	}
	resetChannelFailure(autoDisableFailureKey(channelId, usingKey))
}

func DisableChannelImmediately(channelError types.ChannelError, reason string) {
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过直接禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}
	disableChannelNow(channelError, reason)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) bool {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，进入自动禁用检查，原因：%s", channelError.ChannelName, channelError.ChannelId, common.LocalLogPreview(reason)))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return false
	}

	decision := recordChannelAutoDisableFailure(channelError)
	if !decision.ShouldDisable {
		common.SysLog(fmt.Sprintf(
			"通道「%s」（#%d）命中自动禁用规则 %d/%d，计数后端：%s，TTL：%.0f 秒，暂不禁用，原因：%s",
			channelError.ChannelName,
			channelError.ChannelId,
			decision.Count,
			decision.Threshold,
			decision.Backend,
			decision.Window.Seconds(),
			common.LocalLogPreview(reason),
		))
		return false
	}

	reason = fmt.Sprintf(
		"%d/%d consecutive automatic-disable failures via %s within %.0fs; last error: %s",
		decision.Count,
		decision.Threshold,
		decision.Backend,
		decision.Window.Seconds(),
		reason,
	)
	return disableChannelNow(channelError, reason)
}

func disableChannelNow(channelError types.ChannelError, reason string) bool {
	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		resetChannelFailure(autoDisableFailureKey(channelError.ChannelId, channelError.UsingKey))
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
	return success
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		RecordChannelSuccess(channelId, usingKey)
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
