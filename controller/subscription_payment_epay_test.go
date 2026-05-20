package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionEpayNotifyRejectsWhenWebhookDisabled(t *testing.T) {
	setupSubscriptionEpayCallbackTest(t)
	tradeNo := "sub-epay-disabled-notify"
	insertSubscriptionEpayOrderForTest(t, tradeNo)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/subscription/epay/notify?"+signedSubscriptionEpayCallbackQuery(tradeNo), nil)

	SubscriptionEpayNotify(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "fail", recorder.Body.String())
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	require.NotNil(t, order)
	require.Equal(t, common.TopUpStatusPending, order.Status)
}

func TestSubscriptionEpayReturnRejectsWhenWebhookDisabled(t *testing.T) {
	setupSubscriptionEpayCallbackTest(t)
	tradeNo := "sub-epay-disabled-return"
	insertSubscriptionEpayOrderForTest(t, tradeNo)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/subscription/epay/return?"+signedSubscriptionEpayCallbackQuery(tradeNo), nil)

	SubscriptionEpayReturn(ctx)

	require.Equal(t, http.StatusFound, recorder.Code)
	require.Contains(t, recorder.Header().Get("Location"), "/console/topup?pay=fail")
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	require.NotNil(t, order)
	require.Equal(t, common.TopUpStatusPending, order.Status)
}

func setupSubscriptionEpayCallbackTest(t *testing.T) {
	t.Helper()
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.TopUp{},
		&model.Log{},
	))
	confirmPaymentComplianceForTest(t)

	originalEnabled := operation_setting.EpayEnabled
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		operation_setting.EpayEnabled = originalEnabled
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
	})

	operation_setting.EpayEnabled = true
	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = "epay_key"
	operation_setting.PayMethods = nil

	require.NoError(t, db.Create(&model.User{Id: 801, Username: "sub-epay-callback", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.SubscriptionPlan{
		Id:            901,
		Title:         "Epay disabled callback plan",
		PriceAmount:   9.99,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}).Error)
}

func insertSubscriptionEpayOrderForTest(t *testing.T, tradeNo string) {
	t.Helper()
	require.NoError(t, (&model.SubscriptionOrder{
		UserId:          801,
		PlanId:          901,
		Money:           9.99,
		TradeNo:         tradeNo,
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}).Insert())
}

func signedSubscriptionEpayCallbackQuery(tradeNo string) string {
	params := epay.GenerateParams(map[string]string{
		"pid":          operation_setting.EpayId,
		"trade_no":     "epay-" + tradeNo,
		"out_trade_no": tradeNo,
		"type":         "alipay",
		"name":         "SUB:test",
		"money":        "9.99",
		"trade_status": epay.StatusTradeSuccess,
	}, operation_setting.EpayKey)
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return values.Encode()
}
