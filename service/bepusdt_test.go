package service

import (
	"testing"

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
