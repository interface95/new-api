package controller

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBepusdtPayUsesAuthenticatedDatabaseUser(t *testing.T) {
	db, dialect := openTaskDialectDatabase(t, &model.User{}, &model.TopUp{})
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedis, oldMaster := common.RedisEnabled, common.IsMasterNode
	oldEnabled, oldGateway, oldToken := setting.BepusdtEnabled, setting.BepusdtGatewayURL, setting.BepusdtAuthToken
	oldPrice, oldMin := setting.BepusdtUnitPrice, setting.BepusdtMinTopUp
	oldDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	oldDiscounts := operation_setting.GetPaymentSetting().AmountDiscount
	oldCallback, oldReturn := operation_setting.CustomCallbackAddress, setting.BepusdtReturnURL
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMain, oldLog)
		common.RedisEnabled, common.IsMasterNode = oldRedis, oldMaster
		setting.BepusdtEnabled, setting.BepusdtGatewayURL, setting.BepusdtAuthToken = oldEnabled, oldGateway, oldToken
		setting.BepusdtUnitPrice, setting.BepusdtMinTopUp = oldPrice, oldMin
		operation_setting.GetGeneralSetting().QuotaDisplayType = oldDisplay
		operation_setting.GetPaymentSetting().AmountDiscount = oldDiscounts
		operation_setting.CustomCallbackAddress, setting.BepusdtReturnURL = oldCallback, oldReturn
	})
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(dialect, dialect)
	common.RedisEnabled, common.IsMasterNode = false, false
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	setting.BepusdtEnabled, setting.BepusdtAuthToken = true, "metadata-test-token"
	setting.BepusdtUnitPrice, setting.BepusdtMinTopUp = 1, 1
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.GetPaymentSetting().AmountDiscount = nil
	operation_setting.CustomCallbackAddress = "https://merchant.example.com"
	setting.BepusdtReturnURL = "https://merchant.example.com/console/topup"
	confirmPaymentComplianceForTest(t)
	gin.SetMode(gin.TestMode)

	for index, tc := range []struct {
		name, body, username, displayName string
		invalidID                         int
		invalidMetadata                   bool
		updateProfile                     bool
	}{
		{"old client", `{"amount":10}`, "server-user", " 服务端姓名 ", 0, false, true},
		{"forged identity", `{"amount":10,"id":999,"user_id":"999","username":"forged","display_name":"forged"}`, "real-user", "真实姓名", 0, false, false},
		{"empty names", `{"amount":10,"username":"forged","display_name":"forged"}`, "", "", 0, false, false},
		{"blank database username", `{"amount":10}`, " \u3000", "valid-name", 0, false, false},
		{"profile space", `{"amount":10}`, "profile-space", " ", 0, false, true},
		{"profile fullwidth space", `{"amount":10}`, "profile-fullwidth", "\u3000", 0, false, true},
		{"profile unicode spaces", `{"amount":10}`, "profile-unicode", " \u00a0\u2003\u3000", 0, false, true},
		{"profile carriage return", `{"amount":10}`, "profile-cr", " \r\u3000", 0, true, true},
		{"profile newline", `{"amount":10}`, "profile-lf", " \n\u3000", 0, true, true},
		{"profile tab", `{"amount":10}`, "profile-tab", " \t\u3000", 0, true, true},
		{"missing context", `{"amount":10,"user_id":"999"}`, "unused", "unused", -1, false, false},
		{"missing database user", `{"amount":10,"user_id":"999"}`, "unused", "unused", 999, false, false},
		{"invalid database metadata", `{"amount":10,"username":"client-replacement"}`, "bad\nname", "合法姓名", 0, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := model.User{Username: tc.username, DisplayName: tc.displayName, Group: "default", Quota: 123, Status: common.UserStatusEnabled, AffCode: fmt.Sprintf("meta%d", index)}
			if tc.updateProfile {
				user.DisplayName = "Before"
			}
			if tc.invalidID == 0 {
				require.NoError(t, db.Create(&user).Error)
			}
			if tc.updateProfile {
				profile, err := common.Marshal(map[string]string{"username": user.Username, "display_name": tc.displayName})
				require.NoError(t, err)
				profileRecorder := httptest.NewRecorder()
				profileContext, _ := gin.CreateTestContext(profileRecorder)
				profileContext.Request = httptest.NewRequest(http.MethodPut, "/api/user/self", strings.NewReader(string(profile)))
				profileContext.Request.Header.Set("Content-Type", "application/json")
				profileContext.Set("id", user.Id)

				UpdateSelf(profileContext)

				require.Equal(t, http.StatusOK, profileRecorder.Code)
				var profileResponse struct {
					Success bool `json:"success"`
				}
				require.NoError(t, common.Unmarshal(profileRecorder.Body.Bytes(), &profileResponse))
				require.True(t, profileResponse.Success)
				storedUser, err := model.GetUserById(user.Id, false)
				require.NoError(t, err)
				require.Equal(t, tc.displayName, storedUser.DisplayName)
				require.Equal(t, user.Quota, storedUser.Quota)
			}
			requests := make(chan map[string]any, 1)
			server := newBepusdtMetadataTestGateway(t, requests)
			defer server.Close()
			setting.BepusdtGatewayURL = server.URL
			var beforeCount int64
			require.NoError(t, db.Model(&model.TopUp{}).Count(&beforeCount).Error)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/bepusdt/pay?user_id=999&username=forged", strings.NewReader(tc.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Request.Header.Set("New-Api-User", "999")
			ctx.Set("username", "stale-context-name")
			ctx.Set("display_name", "stale-context-display-name")
			if tc.invalidID == 0 {
				ctx.Set("id", user.Id)
			} else if tc.invalidID > 0 {
				ctx.Set("id", tc.invalidID)
			}

			RequestBepusdtPay(ctx)

			assert.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			if tc.invalidID != 0 {
				assert.Equal(t, "error", response.Message)
				assert.Empty(t, requests)
				var afterCount int64
				require.NoError(t, db.Model(&model.TopUp{}).Count(&afterCount).Error)
				assert.Equal(t, beforeCount, afterCount)
				return
			}
			if tc.invalidMetadata {
				assert.Equal(t, "error", response.Message)
				assert.Empty(t, requests)
				var topUp model.TopUp
				require.NoError(t, db.Where("user_id = ?", user.Id).First(&topUp).Error)
				assert.Equal(t, common.TopUpStatusFailed, topUp.Status)
				assert.Equal(t, int64(10), topUp.Amount)
				assert.Equal(t, float64(10), topUp.Money)
				storedUser, err := model.GetUserById(user.Id, false)
				require.NoError(t, err)
				assert.Equal(t, user.Quota, storedUser.Quota)
				return
			}
			require.Equal(t, "success", response.Message, recorder.Body.String())
			require.Len(t, requests, 1)
			request := <-requests
			assert.Equal(t, strconv.Itoa(user.Id), request["user_id"])
			for key, expected := range map[string]string{"username": tc.username, "display_name": tc.displayName} {
				if strings.TrimSpace(expected) == "" {
					assert.NotContains(t, request, key)
				} else {
					assert.Equal(t, expected, request[key])
				}
			}
			assert.Equal(t, service.SignBepusdtParams(request, setting.BepusdtAuthToken), request["signature"])
			for _, key := range []string{"user_id", "username", "display_name"} {
				tampered := maps.Clone(request)
				tampered[key] = "tampered"
				assert.NotEqual(t, request["signature"], service.SignBepusdtParams(tampered, setting.BepusdtAuthToken))
			}
			assert.Equal(t, float64(10), request["amount"])
			topUp := model.GetTopUpByTradeNo(request["order_id"].(string))
			require.NotNil(t, topUp)
			assert.Equal(t, user.Id, topUp.UserId)
			assert.Equal(t, int64(10), topUp.Amount)
			assert.Equal(t, float64(10), topUp.Money)
			assert.Equal(t, common.TopUpStatusPending, topUp.Status)
			storedUser, err := model.GetUserById(user.Id, false)
			require.NoError(t, err)
			assert.Equal(t, user.Quota, storedUser.Quota)
			assert.Equal(t, tc.username, storedUser.Username)
			assert.Equal(t, tc.displayName, storedUser.DisplayName)
		})
	}
}

func TestCreateBepusdtOrderOptionalUserMetadata(t *testing.T) {
	oldGateway, oldToken := setting.BepusdtGatewayURL, setting.BepusdtAuthToken
	t.Cleanup(func() {
		setting.BepusdtGatewayURL, setting.BepusdtAuthToken = oldGateway, oldToken
	})
	setting.BepusdtAuthToken = "metadata-test-token"
	for _, tc := range []struct {
		name, userID, username, displayName string
		invalidField                        string
	}{
		{"legacy caller", "", "", "", ""},
		{"id only", "42", "", "", ""},
		{"leading zero id", "0042", "", "", ""},
		{"unicode boundaries", strings.Repeat("9", 64), strings.Repeat("名", 64), strings.Repeat("姓", 128), ""},
		{"unchanged names", "42", " server-user ", " 姓名 & name=值 ", ""},
		{"unchanged unicode padded names", "42", "\u3000server-user\u00a0", "\u2003姓名\u3000", ""},
		{"trailing ampersand", "42", "alice&", "Alice", ""},
		{"trailing ampersands", "42", "alice&&", "Alice", ""},
		{"overlong id", strings.Repeat("9", 65), "valid-user", "valid-name", "user_id"},
		{"overlong username", "42", strings.Repeat("名", 65), "valid-name", "username"},
		{"overlong display name", "42", "valid-user", strings.Repeat("姓", 129), "display_name"},
		{"control in id", "4\n2", "valid-user", "valid-name", "user_id"},
		{"control in username", "42", "user\x00name", "valid-name", "username"},
		{"control in display name", "42", "valid-user", "name\u0085", "display_name"},
		{"invalid utf8 username", "42", "user\xff", "valid-name", "username"},
		{"invalid utf8 display name", "42", "valid-user", "name\xff", "display_name"},
		{"invalid utf8 id", "4\xff", "valid-user", "valid-name", "user_id"},
		{"nondecimal id", "4a", "valid-user", "valid-name", "user_id"},
		{"blank id", " ", "valid-user", "valid-name", "user_id"},
		{"unicode blank id", "\u3000", "valid-user", "valid-name", "user_id"},
		{"padded id", " 42 ", "valid-user", "valid-name", "user_id"},
		{"signed id", "+42", "valid-user", "valid-name", "user_id"},
		{"negative id", "-42", "valid-user", "valid-name", "user_id"},
		{"unicode digits id", "\uff14\uff12", "valid-user", "valid-name", "user_id"},
		{"blank username", "42", " ", "valid-name", ""},
		{"blank display name", "42", "valid-user", "\u3000", ""},
		{"unicode blank names", "42", " \u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a", "\u2028\u2029\u202f\u205f\u3000", ""},
		{"carriage return username", "42", " \r\u3000", "valid-name", "username"},
		{"newline username", "42", " \n\u3000", "valid-name", "username"},
		{"tab username", "42", " \t\u3000", "valid-name", "username"},
		{"carriage return display name", "42", "valid-user", " \r\u3000", "display_name"},
		{"newline display name", "42", "valid-user", " \n\u3000", "display_name"},
		{"tab display name", "42", "valid-user", " \t\u3000", "display_name"},
		{"next line display name", "42", "valid-user", " \u0085\u3000", "display_name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := service.BepusdtCreateOrderParams{
				OrderID: "ORDER1", Amount: "28.88", Fiat: "CNY", Currencies: "USDT", Name: "TUC50",
				NotifyURL: "https://merchant.example.com/api/bepusdt/webhook", RedirectURL: "https://merchant.example.com/console/topup",
				UserID: tc.userID, Username: tc.username, DisplayName: tc.displayName,
			}
			metadata := map[string]string{"user_id": tc.userID, "username": tc.username, "display_name": tc.displayName}
			requests := make(chan map[string]any, 1)
			server := newBepusdtMetadataTestGateway(t, requests)
			defer server.Close()
			setting.BepusdtGatewayURL = server.URL

			result, err := service.CreateBepusdtOrder(context.Background(), params)

			if tc.invalidField != "" {
				assert.Empty(t, requests)
				assert.Nil(t, result)
				require.ErrorIs(t, err, service.ErrBepusdtConfigInvalid)
				assert.EqualError(t, err, "bepusdt config invalid: invalid "+tc.invalidField)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "https://pay.example.com/pay/cashier/TRADE1", result.PaymentURL)
			require.Len(t, requests, 1)
			request := <-requests
			expected := map[string]any{
				"order_id": params.OrderID, "amount": 28.88, "fiat": "CNY", "currencies": "USDT", "name": "TUC50",
				"notify_url": params.NotifyURL, "redirect_url": params.RedirectURL,
				"user_id": tc.userID, "username": tc.username, "display_name": tc.displayName,
			}
			for key, value := range metadata {
				if value == "" || key != "user_id" && strings.TrimSpace(value) == "" {
					delete(expected, key)
				}
			}
			expected["signature"] = service.SignBepusdtParams(expected, setting.BepusdtAuthToken)
			assert.Equal(t, expected, request)
			if tc.name == "legacy caller" {
				encoded, err := common.Marshal(params)
				require.NoError(t, err)
				var serialized map[string]any
				require.NoError(t, common.Unmarshal(encoded, &serialized))
				for key := range metadata {
					assert.NotContains(t, serialized, key)
				}
			}
		})
	}
}

func TestBepusdtUserMetadataGatewaySignatureVector(t *testing.T) {
	// Shared with BEpusdt's receiver test; do not derive the expected hash using the signer under test.
	for _, username := range []string{"alice&", "alice&&"} {
		t.Run(username, func(t *testing.T) {
			params := map[string]any{
				"amount": 70, "fiat": "CNY", "notify_url": "https://merchant.example/notify",
				"order_id": "metadata-signature-vector", "redirect_url": "https://merchant.example/return",
				"user_id": "485", "username": username, "display_name": "测试用户",
			}
			assert.Equal(t, "c2a44ccf737837b9bd6339637b0d7b58", service.SignBepusdtParams(params, "metadata-vector-token"))
			assert.Equal(t, username, params["username"], "signing must not rewrite the user snapshot")
		})
	}
}

func newBepusdtMetadataTestGateway(t *testing.T, requests chan<- map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/order/create-order", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var request map[string]any
		if !assert.NoError(t, common.DecodeJson(r.Body, &request)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request
		if !assert.Equal(t, service.SignBepusdtParams(request, setting.BepusdtAuthToken), request["signature"]) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"status_code":200,"data":{"trade_id":"TRADE1","payment_url":"https://pay.example.com/pay/cashier/TRADE1"}}`))
		assert.NoError(t, err)
	}))
}

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
	originalCurrencies := setting.BepusdtCurrencies
	originalMinTopUp := setting.BepusdtMinTopUp
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	t.Cleanup(func() {
		setting.BepusdtEnabled = originalEnabled
		setting.BepusdtGatewayURL = originalGatewayURL
		setting.BepusdtAuthToken = originalAuthToken
		setting.BepusdtCurrencies = originalCurrencies
		setting.BepusdtMinTopUp = originalMinTopUp
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
	})

	setting.BepusdtEnabled = true
	setting.BepusdtGatewayURL = "https://pay.example.com"
	setting.BepusdtAuthToken = "secret"
	setting.BepusdtCurrencies = "USDT,USDC"
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
		"name":      "USDT / USDC",
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
