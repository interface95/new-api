package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignBepusdtParamsSortsAndSkipsEmptyValues(t *testing.T) {
	signature := SignBepusdtParams(map[string]interface{}{
		"b":         "2",
		"a":         "1",
		"empty":     "",
		"signature": "ignored",
		"status":    2,
	}, "secret")

	assert.Equal(t, "0bad545464f089b2bdd07921d269bbfe", signature)
}

func TestVerifyBepusdtCallbackRequiresValidSuccessSignature(t *testing.T) {
	callback := BepusdtCallbackData{
		TradeID:            "TRADE1",
		OrderID:            "ORDER1",
		Amount:             "9.99",
		ActualAmount:       "12.3456",
		Token:              "TADDR",
		BlockTransactionID: "tx123",
		Status:             BepusdtStatusSuccess,
		Signature:          "d13184845479416140c01388919adbad",
	}

	require.NoError(t, VerifyBepusdtCallback("secret", &callback))

	callback.Signature = "bad"
	assert.ErrorIs(t, VerifyBepusdtCallback("secret", &callback), ErrBepusdtSignatureInvalid)
}

func TestVerifyBepusdtCallbackRejectsNonSuccessStatus(t *testing.T) {
	callback := BepusdtCallbackData{
		TradeID:            "TRADE1",
		OrderID:            "ORDER1",
		Amount:             "9.99",
		ActualAmount:       "12.3456",
		Token:              "TADDR",
		BlockTransactionID: "tx123",
		Status:             BepusdtStatusExpired,
	}
	callback.Signature = SignBepusdtParams(map[string]interface{}{
		"trade_id":             callback.TradeID,
		"order_id":             callback.OrderID,
		"amount":               callback.Amount,
		"actual_amount":        callback.ActualAmount,
		"token":                callback.Token,
		"block_transaction_id": callback.BlockTransactionID,
		"status":               callback.Status,
	}, "secret")

	assert.ErrorIs(t, VerifyBepusdtCallback("secret", &callback), ErrBepusdtStatusInvalid)
}

func TestCreateBepusdtOrderUsesCashierEndpointAndCurrencies(t *testing.T) {
	originalGatewayURL := setting.BepusdtGatewayURL
	originalAuthToken := setting.BepusdtAuthToken
	originalFiat := setting.BepusdtFiat
	t.Cleanup(func() {
		setting.BepusdtGatewayURL = originalGatewayURL
		setting.BepusdtAuthToken = originalAuthToken
		setting.BepusdtFiat = originalFiat
	})

	var requestPath string
	var requestParams map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		require.NoError(t, common.DecodeJson(r.Body, &requestParams))
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"status_code": 200,
			"message": "success",
			"data": {
				"fiat": "CNY",
				"trade_id": "TRADE1",
				"order_id": "ORDER1",
				"amount": "28.88",
				"expiration_time": 1200,
				"payment_url": "https://pay.example.com/pay/cashier/TRADE1"
			}
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	setting.BepusdtGatewayURL = server.URL
	setting.BepusdtAuthToken = "secret"
	setting.BepusdtFiat = "CNY"

	result, err := CreateBepusdtOrder(context.Background(), BepusdtCreateOrderParams{
		OrderID:     "ORDER1",
		Amount:      "28.88",
		NotifyURL:   "https://merchant.example.com/api/bepusdt/webhook",
		RedirectURL: "https://merchant.example.com/console/topup",
		Currencies:  "usdt, USDC , usdt",
		Name:        "TUC50",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "/api/v1/order/create-order", requestPath)
	assert.Equal(t, "USDT,USDC", requestParams["currencies"])
	assert.NotContains(t, requestParams, "trade_type")
	assert.Equal(t, "https://pay.example.com/pay/cashier/TRADE1", result.PaymentURL)
	assert.Equal(t, "TRADE1", result.TradeID)

	expectedSignature := SignBepusdtParams(map[string]interface{}{
		"order_id":     "ORDER1",
		"amount":       28.88,
		"notify_url":   "https://merchant.example.com/api/bepusdt/webhook",
		"redirect_url": "https://merchant.example.com/console/topup",
		"currencies":   "USDT,USDC",
		"fiat":         "CNY",
		"name":         "TUC50",
	}, "secret")
	assert.Equal(t, expectedSignature, requestParams["signature"])
}
