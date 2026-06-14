package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
	"github.com/shopspring/decimal"
)

const (
	BepusdtStatusWaiting = 1
	BepusdtStatusSuccess = 2
	BepusdtStatusExpired = 3

	bepusdtCreateOrderPath = "/api/v1/order/create-order"
	BepusdtDefaultFiat     = "CNY"
)

var (
	ErrBepusdtConfigInvalid    = errors.New("bepusdt config invalid")
	ErrBepusdtRequestFailed    = errors.New("bepusdt request failed")
	ErrBepusdtResponseInvalid  = errors.New("bepusdt response invalid")
	ErrBepusdtSignatureInvalid = errors.New("bepusdt signature invalid")
	ErrBepusdtStatusInvalid    = errors.New("bepusdt status invalid")
)

type BepusdtCreateOrderParams struct {
	OrderID     string `json:"order_id"`
	Amount      string `json:"amount"`
	NotifyURL   string `json:"notify_url"`
	RedirectURL string `json:"redirect_url"`
	Currencies  string `json:"currencies"`
	Fiat        string `json:"fiat"`
	Name        string `json:"name,omitempty"`
}

type BepusdtCreateTransactionResult struct {
	TradeID      string `json:"trade_id"`
	OrderID      string `json:"order_id"`
	Amount       string `json:"amount"`
	ActualAmount string `json:"actual_amount"`
	Token        string `json:"token"`
	PaymentURL   string `json:"payment_url"`
}

type BepusdtCallbackData struct {
	TradeID            string          `json:"trade_id"`
	OrderID            string          `json:"order_id"`
	Amount             dto.StringValue `json:"amount"`
	ActualAmount       dto.StringValue `json:"actual_amount"`
	Token              string          `json:"token"`
	BlockTransactionID string          `json:"block_transaction_id"`
	Signature          string          `json:"signature"`
	Status             int             `json:"status"`
}

func SignBepusdtParams(params map[string]interface{}, authToken string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "signature" || isBepusdtEmptyValue(value) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%v", key, params[key]))
	}

	sum := md5.Sum([]byte(strings.Join(pairs, "&") + authToken))
	return strings.ToLower(hex.EncodeToString(sum[:]))
}

func ParseBepusdtCallback(body []byte) (*BepusdtCallbackData, error) {
	if len(body) == 0 {
		return nil, ErrBepusdtResponseInvalid
	}
	var data BepusdtCallbackData
	if err := common.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBepusdtResponseInvalid, err)
	}
	return &data, nil
}

func VerifyBepusdtCallback(authToken string, data *BepusdtCallbackData) error {
	if err := VerifyBepusdtCallbackSignature(authToken, data); err != nil {
		return err
	}
	if data.Status != BepusdtStatusSuccess {
		return ErrBepusdtStatusInvalid
	}
	return nil
}

func VerifyBepusdtCallbackSignature(authToken string, data *BepusdtCallbackData) error {
	if strings.TrimSpace(authToken) == "" || data == nil {
		return ErrBepusdtConfigInvalid
	}
	if strings.TrimSpace(data.OrderID) == "" || strings.TrimSpace(data.TradeID) == "" || strings.TrimSpace(data.Signature) == "" {
		return ErrBepusdtResponseInvalid
	}

	expected := SignBepusdtParams(map[string]interface{}{
		"trade_id":             data.TradeID,
		"order_id":             data.OrderID,
		"amount":               data.Amount,
		"actual_amount":        data.ActualAmount,
		"token":                data.Token,
		"block_transaction_id": data.BlockTransactionID,
		"status":               data.Status,
	}, authToken)

	if subtle.ConstantTimeCompare([]byte(strings.ToLower(expected)), []byte(strings.ToLower(data.Signature))) != 1 {
		return ErrBepusdtSignatureInvalid
	}
	return nil
}

func CreateBepusdtOrder(ctx context.Context, params BepusdtCreateOrderParams) (*BepusdtCreateTransactionResult, error) {
	gatewayURL := strings.TrimRight(strings.TrimSpace(setting.BepusdtGatewayURL), "/")
	authToken := strings.TrimSpace(setting.BepusdtAuthToken)
	if gatewayURL == "" || authToken == "" || params.OrderID == "" || params.Amount == "" || params.NotifyURL == "" || params.RedirectURL == "" {
		return nil, ErrBepusdtConfigInvalid
	}
	if params.Fiat == "" {
		params.Fiat = setting.BepusdtFiat
	}
	if strings.TrimSpace(params.Fiat) == "" {
		params.Fiat = BepusdtDefaultFiat
	}
	if params.Currencies == "" {
		params.Currencies = setting.BepusdtCurrencies
	}
	params.Currencies = setting.NormalizeBepusdtCurrencies(params.Currencies)

	amountDecimal, err := decimal.NewFromString(params.Amount)
	if err != nil || !amountDecimal.IsPositive() {
		return nil, fmt.Errorf("%w: invalid amount", ErrBepusdtConfigInvalid)
	}
	amountValue, _ := amountDecimal.Float64()

	requestParams := map[string]interface{}{
		"order_id":     params.OrderID,
		"amount":       amountValue,
		"notify_url":   params.NotifyURL,
		"redirect_url": params.RedirectURL,
		"fiat":         params.Fiat,
	}
	if strings.TrimSpace(params.Currencies) != "" {
		requestParams["currencies"] = params.Currencies
	}
	if strings.TrimSpace(params.Name) != "" {
		requestParams["name"] = params.Name
	}

	return createBepusdtPayment(ctx, gatewayURL, authToken, bepusdtCreateOrderPath, requestParams)
}

func createBepusdtPayment(ctx context.Context, gatewayURL string, authToken string, path string, requestParams map[string]interface{}) (*BepusdtCreateTransactionResult, error) {
	requestParams["signature"] = SignBepusdtParams(requestParams, authToken)

	body, err := common.Marshal(requestParams)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBepusdtRequestFailed, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gatewayURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBepusdtRequestFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBepusdtRequestFailed, err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBepusdtResponseInvalid, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: http status %d", ErrBepusdtResponseInvalid, resp.StatusCode)
	}

	var result struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			TradeID      string `json:"trade_id"`
			OrderID      string `json:"order_id"`
			Amount       string `json:"amount"`
			ActualAmount string `json:"actual_amount"`
			Token        string `json:"token"`
			PaymentURL   string `json:"payment_url"`
		} `json:"data"`
	}
	if err := common.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBepusdtResponseInvalid, err)
	}
	if result.StatusCode != http.StatusOK || strings.TrimSpace(result.Data.PaymentURL) == "" {
		return nil, fmt.Errorf("%w: %s", ErrBepusdtResponseInvalid, result.Message)
	}

	return &BepusdtCreateTransactionResult{
		TradeID:      result.Data.TradeID,
		OrderID:      result.Data.OrderID,
		Amount:       result.Data.Amount,
		ActualAmount: result.Data.ActualAmount,
		Token:        result.Data.Token,
		PaymentURL:   result.Data.PaymentURL,
	}, nil
}

func isBepusdtEmptyValue(value interface{}) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case dto.StringValue:
		return strings.TrimSpace(string(v)) == ""
	default:
		return false
	}
}
