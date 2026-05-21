package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetBepusdtMinTopupHonorsTokenDisplay(t *testing.T) {
	originalMinTopUp := setting.BepusdtMinTopUp
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	t.Cleanup(func() {
		setting.BepusdtMinTopUp = originalMinTopUp
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
	})

	setting.BepusdtMinTopUp = 2

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	require.Equal(t, int64(2), getBepusdtMinTopup())

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	require.Equal(t, int64(common.QuotaPerUnit*2), getBepusdtMinTopup())
}

func TestNormalizeBepusdtTopUpAmountDoesNotRoundPartialTokenUnitUp(t *testing.T) {
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
	})

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens

	require.Equal(t, int64(0), normalizeBepusdtTopUpAmount(int64(common.QuotaPerUnit/100)))
	require.Equal(t, int64(1), normalizeBepusdtTopUpAmount(int64(common.QuotaPerUnit)))
	require.Equal(t, int64(3), normalizeBepusdtTopUpAmount(int64(common.QuotaPerUnit*3)))
}

func TestRequestBepusdtAmountRejectsDisabledPayment(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.BepusdtEnabled
	originalGatewayURL := setting.BepusdtGatewayURL
	originalAuthToken := setting.BepusdtAuthToken
	t.Cleanup(func() {
		setting.BepusdtEnabled = originalEnabled
		setting.BepusdtGatewayURL = originalGatewayURL
		setting.BepusdtAuthToken = originalAuthToken
	})

	setting.BepusdtEnabled = false
	setting.BepusdtGatewayURL = "https://pay.example.com"
	setting.BepusdtAuthToken = "secret"

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/bepusdt/amount", strings.NewReader(`{"amount":10}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestBepusdtAmount(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "BEpusdt 支付未启用")
}

func TestGetTopUpInfoUsesBepusdtTokenDisplayMinimum(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.BepusdtEnabled
	originalGatewayURL := setting.BepusdtGatewayURL
	originalAuthToken := setting.BepusdtAuthToken
	originalMinTopUp := setting.BepusdtMinTopUp
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	t.Cleanup(func() {
		setting.BepusdtEnabled = originalEnabled
		setting.BepusdtGatewayURL = originalGatewayURL
		setting.BepusdtAuthToken = originalAuthToken
		setting.BepusdtMinTopUp = originalMinTopUp
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
	})

	setting.BepusdtEnabled = true
	setting.BepusdtGatewayURL = "https://pay.example.com"
	setting.BepusdtAuthToken = "secret"
	setting.BepusdtMinTopUp = 2
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)

	GetTopUpInfo(ctx)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			BepusdtMinTopUp int64               `json:"bepusdt_min_topup"`
			PayMethods      []map[string]string `json:"pay_methods"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	expectedMinTopup := fmt.Sprintf("%.0f", common.QuotaPerUnit*2)
	require.Equal(t, int64(common.QuotaPerUnit*2), response.Data.BepusdtMinTopUp)
	require.Contains(t, response.Data.PayMethods, map[string]string{
		"name":      "USDT(TRC20)",
		"type":      model.PaymentMethodBepusdt,
		"color":     "rgba(var(--semi-green-5), 1)",
		"min_topup": expectedMinTopup,
	})
}

func TestBepusdtWebhookMarksExpiredOrderFailed(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}))
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.BepusdtEnabled
	originalGatewayURL := setting.BepusdtGatewayURL
	originalAuthToken := setting.BepusdtAuthToken
	t.Cleanup(func() {
		setting.BepusdtEnabled = originalEnabled
		setting.BepusdtGatewayURL = originalGatewayURL
		setting.BepusdtAuthToken = originalAuthToken
	})

	setting.BepusdtEnabled = true
	setting.BepusdtGatewayURL = "https://pay.example.com"
	setting.BepusdtAuthToken = "secret"
	require.NoError(t, db.Create(&model.User{Id: 701, Username: "bepusdt-expired", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, (&model.TopUp{
		UserId:          701,
		Amount:          1,
		Money:           9.99,
		TradeNo:         "bepusdt-expired-order",
		PaymentMethod:   model.PaymentMethodBepusdt,
		PaymentProvider: model.PaymentProviderBepusdt,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}).Insert())

	body := signedBepusdtCallbackBody(t, setting.BepusdtAuthToken, service.BepusdtCallbackData{
		TradeID:            "trade-expired",
		OrderID:            "bepusdt-expired-order",
		Amount:             "9.99",
		ActualAmount:       "9.99",
		Token:              "token-address",
		BlockTransactionID: "tx-expired",
		Status:             service.BepusdtStatusExpired,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/bepusdt/webhook", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	BepusdtWebhook(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	topUp := model.GetTopUpByTradeNo("bepusdt-expired-order")
	require.NotNil(t, topUp)
	require.Equal(t, common.TopUpStatusFailed, topUp.Status)
}

func signedBepusdtCallbackBody(t *testing.T, authToken string, callback service.BepusdtCallbackData) string {
	t.Helper()
	callback.Signature = service.SignBepusdtParams(map[string]interface{}{
		"trade_id":             callback.TradeID,
		"order_id":             callback.OrderID,
		"amount":               callback.Amount,
		"actual_amount":        callback.ActualAmount,
		"token":                callback.Token,
		"block_transaction_id": callback.BlockTransactionID,
		"status":               callback.Status,
	}, authToken)
	body, err := common.Marshal(callback)
	require.NoError(t, err)
	return string(body)
}
