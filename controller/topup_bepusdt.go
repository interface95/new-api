package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
)

type BepusdtPayRequest struct {
	Amount int64 `json:"amount"`
}

func RequestBepusdtAmount(c *gin.Context) {
	if !isBepusdtTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "BEpusdt 支付未启用"})
		return
	}

	var req BepusdtPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	minTopup := getBepusdtMinTopup()
	if req.Amount < minTopup {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", minTopup)})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getBepusdtPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "data": formatBepusdtAmount(payMoney)})
}

func RequestBepusdtPay(c *gin.Context) {
	if !isBepusdtTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "BEpusdt 支付未启用"})
		return
	}

	var req BepusdtPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	minTopup := getBepusdtMinTopup()
	if req.Amount < minTopup {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", minTopup)})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getBepusdtPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	tradeNo := fmt.Sprintf("BEPUSDT-%d-%d-%s", id, time.Now().UnixMilli(), randstr.String(6))
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          normalizeBepusdtTopUpAmount(req.Amount),
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodBepusdt,
		PaymentProvider: model.PaymentProviderBepusdt,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt 创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	callbackAddress := service.GetCallbackAddress()
	result, err := service.CreateBepusdtTransaction(c.Request.Context(), service.BepusdtCreateTransactionParams{
		OrderID:     tradeNo,
		Amount:      formatBepusdtAmount(payMoney),
		NotifyURL:   callbackAddress + "/api/bepusdt/webhook",
		RedirectURL: getBepusdtReturnURL(),
		TradeType:   setting.BepusdtTradeType,
		Fiat:        setting.BepusdtFiat,
		Name:        fmt.Sprintf("TUC%d", req.Amount),
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt 创建支付交易失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt 充值订单创建成功 user_id=%d trade_no=%s trade_id=%s amount=%d money=%.2f", id, tradeNo, result.TradeID, req.Amount, payMoney))
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{
		"checkout_url":  result.PaymentURL,
		"payment_url":   result.PaymentURL,
		"trade_id":      result.TradeID,
		"order_id":      tradeNo,
		"actual_amount": result.ActualAmount,
		"token":         result.Token,
	}})
}

func BepusdtWebhook(c *gin.Context) {
	if !isBepusdtWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.String(http.StatusForbidden, "fail")
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 读取请求体失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.String(http.StatusBadRequest, "fail")
		return
	}

	callback, err := service.ParseBepusdtCallback(bodyBytes)
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 解析失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.String(http.StatusBadRequest, "fail")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 收到请求 trade_no=%s trade_id=%s status=%d client_ip=%s", callback.OrderID, callback.TradeID, callback.Status, c.ClientIP()))

	if err := service.VerifyBepusdtCallbackSignature(setting.BepusdtAuthToken, callback); err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 验签失败 trade_no=%s trade_id=%s client_ip=%s error=%q", callback.OrderID, callback.TradeID, c.ClientIP(), err.Error()))
		c.String(http.StatusUnauthorized, "fail")
		return
	}

	LockOrder(callback.OrderID)
	defer UnlockOrder(callback.OrderID)

	if callback.Status == service.BepusdtStatusExpired {
		if err := model.UpdatePendingTopUpStatus(callback.OrderID, model.PaymentProviderBepusdt, common.TopUpStatusFailed); err != nil &&
			!errors.Is(err, model.ErrTopUpStatusInvalid) {
			logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt 标记过期订单失败 trade_no=%s trade_id=%s client_ip=%s error=%q", callback.OrderID, callback.TradeID, c.ClientIP(), err.Error()))
			c.String(http.StatusInternalServerError, "fail")
			return
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt 订单已过期 trade_no=%s trade_id=%s client_ip=%s", callback.OrderID, callback.TradeID, c.ClientIP()))
		c.String(http.StatusOK, "success")
		return
	}

	if callback.Status != service.BepusdtStatusSuccess {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 状态无效 trade_no=%s trade_id=%s status=%d client_ip=%s", callback.OrderID, callback.TradeID, callback.Status, c.ClientIP()))
		c.String(http.StatusBadRequest, "fail")
		return
	}

	topUp := model.GetTopUpByTradeNo(callback.OrderID)
	if topUp == nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 订单不存在 trade_no=%s trade_id=%s client_ip=%s", callback.OrderID, callback.TradeID, c.ClientIP()))
		c.String(http.StatusNotFound, "fail")
		return
	}
	if topUp.PaymentProvider != model.PaymentProviderBepusdt {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 订单支付网关不匹配 trade_no=%s order_provider=%s trade_id=%s client_ip=%s", callback.OrderID, topUp.PaymentProvider, callback.TradeID, c.ClientIP()))
		c.String(http.StatusBadRequest, "fail")
		return
	}
	if !bepusdtAmountMatchesTopUp(string(callback.Amount), topUp.Money) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 金额不匹配 trade_no=%s callback_amount=%s order_money=%.2f trade_id=%s client_ip=%s", callback.OrderID, callback.Amount, topUp.Money, callback.TradeID, c.ClientIP()))
		c.String(http.StatusBadRequest, "fail")
		return
	}

	if err := model.RechargeBepusdt(callback.OrderID, c.ClientIP()); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt 充值处理失败 trade_no=%s trade_id=%s client_ip=%s error=%q", callback.OrderID, callback.TradeID, c.ClientIP(), err.Error()))
		c.String(http.StatusInternalServerError, "fail")
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt 充值成功 trade_no=%s trade_id=%s client_ip=%s", callback.OrderID, callback.TradeID, c.ClientIP()))
	c.String(http.StatusOK, "success")
}

func getBepusdtPayMoney(amount int64, group string) float64 {
	dAmount := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount = dAmount.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	}
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok && ds > 0 {
		discount = ds
	}
	return dAmount.
		Mul(decimal.NewFromFloat(setting.BepusdtUnitPrice)).
		Mul(decimal.NewFromFloat(topupGroupRatio)).
		Mul(decimal.NewFromFloat(discount)).
		InexactFloat64()
}

func getBepusdtMinTopup() int64 {
	minTopup := int64(setting.BepusdtMinTopUp)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		return decimal.NewFromInt(minTopup).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	}
	return minTopup
}

func normalizeBepusdtTopUpAmount(amount int64) int64 {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		return amount
	}
	return decimal.NewFromInt(amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
}

func formatBepusdtAmount(payMoney float64) string {
	return decimal.NewFromFloat(payMoney).StringFixed(2)
}

func getBepusdtReturnURL() string {
	if strings.TrimSpace(setting.BepusdtReturnURL) != "" {
		return setting.BepusdtReturnURL
	}
	return paymentReturnPath("/console/topup?show_history=true")
}

func bepusdtAmountMatchesTopUp(callbackAmount string, topUpMoney float64) bool {
	actual, err := decimal.NewFromString(callbackAmount)
	if err != nil {
		return false
	}
	expected, err := decimal.NewFromString(formatBepusdtAmount(topUpMoney))
	if err != nil {
		return false
	}
	return actual.Equal(expected)
}
