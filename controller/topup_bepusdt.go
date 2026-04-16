package controller

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
)

// bepusdtSign 计算 BEpusdt 签名
// 规则：过滤空值和 signature 字段 → 按 key ASCII 排序 → key=value& 拼接 → 末尾追加 token → MD5 小写
func bepusdtSign(params map[string]string, token string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k != "signature" && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	raw := strings.Join(parts, "&") + token
	return fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

// bepusdtVerifySign 验证 BEpusdt 回调签名
func bepusdtVerifySign(params map[string]string, token string) bool {
	expected := bepusdtSign(params, token)
	return params["signature"] == expected
}

type BepusdtPayRequest struct {
	Amount int64 `json:"amount"`
}

type bepusdtCreateOrderResp struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Data       struct {
		TradeId    string `json:"trade_id"`
		PaymentUrl string `json:"payment_url"`
	} `json:"data"`
}

// getBepusdtPayMoney 计算 USDT 应付金额
func getBepusdtPayMoney(amount int64, group string) float64 {
	return getPayMoney(amount, group) * setting.BepusdtUnitPrice
}

// RequestBepusdtPay 创建 BEpusdt 支付订单
func RequestBepusdtPay(c *gin.Context) {
	if !setting.BepusdtEnabled || setting.BepusdtHost == "" || setting.BepusdtToken == "" {
		c.JSON(200, gin.H{"message": "error", "data": "USDT 支付未启用"})
		return
	}

	var req BepusdtPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	minTopup := int64(setting.BepusdtMinTopUp)
	if req.Amount < minTopup {
		c.JSON(200, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", minTopup)})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(200, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getBepusdtPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(200, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	tradeNo := fmt.Sprintf("BEPUSDT-%d-%d-%s", id, time.Now().UnixMilli(), randstr.String(6))

	// 归一化 amount（Token 模式）
	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = int64(float64(req.Amount) / common.QuotaPerUnit)
		if amount < 1 {
			amount = 1
		}
	}

	// 创建本地订单
	topUp := &model.TopUp{
		UserId:        id,
		Amount:        amount,
		Money:         payMoney,
		TradeNo:       tradeNo,
		PaymentMethod: "bepusdt",
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		log.Printf("BEpusdt 创建本地订单失败: %v", err)
		c.JSON(200, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	// 构造回调和跳转地址
	callbackAddr := service.GetCallbackAddress()
	notifyUrl := callbackAddr + "/api/bepusdt/notify"
	if setting.BepusdtNotifyUrl != "" {
		notifyUrl = setting.BepusdtNotifyUrl
	}
	returnUrl := system_setting.ServerAddress + "/console/topup?show_history=true"
	if setting.BepusdtReturnUrl != "" {
		returnUrl = setting.BepusdtReturnUrl
	}

	// 构造签名参数
	params := map[string]string{
		"order_id":     tradeNo,
		"amount":       fmt.Sprintf("%.2f", payMoney),
		"notify_url":   notifyUrl,
		"redirect_url": returnUrl,
	}
	params["signature"] = bepusdtSign(params, setting.BepusdtToken)

	// 请求 BEpusdt 创建订单
	formData := url.Values{}
	for k, v := range params {
		formData.Set(k, v)
	}

	apiUrl := strings.TrimRight(setting.BepusdtHost, "/") + "/api/v1/order/create-transaction"
	resp, err := http.PostForm(apiUrl, formData)
	if err != nil {
		log.Printf("BEpusdt 请求失败: %v", err)
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(200, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result bepusdtCreateOrderResp
	if err := json.Unmarshal(body, &result); err != nil || result.StatusCode != 200 {
		log.Printf("BEpusdt 创建订单失败: %s", string(body))
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(200, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	log.Printf("BEpusdt 订单创建成功 - 用户: %d, 订单: %s, 金额: %.2f USDT", id, tradeNo, payMoney)
	c.JSON(200, gin.H{
		"message": "success",
		"data": gin.H{
			"payment_url": result.Data.PaymentUrl,
			"order_id":    tradeNo,
		},
	})
}

// BepusdtNotify 处理 BEpusdt 支付回调
// BEpusdt 要求响应体返回字符串 "success"
func BepusdtNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		log.Printf("BEpusdt 回调解析失败: %v", err)
		_, _ = c.Writer.WriteString("fail")
		return
	}

	params := make(map[string]string)
	for k := range c.Request.PostForm {
		params[k] = c.Request.PostForm.Get(k)
	}

	// 验证签名
	if !bepusdtVerifySign(params, setting.BepusdtToken) {
		log.Printf("BEpusdt 回调签名验证失败: %v", params)
		_, _ = c.Writer.WriteString("fail")
		return
	}

	// status: 1=等待支付 2=已支付 3=超时
	status := params["status"]
	orderId := params["order_id"]

	if status != "2" {
		log.Printf("BEpusdt 回调非支付成功状态: status=%s, order_id=%s", status, orderId)
		if status == "3" && orderId != "" {
			// 超时：标记为失败
			if topUp := model.GetTopUpByTradeNo(orderId); topUp != nil && topUp.Status == common.TopUpStatusPending {
				topUp.Status = common.TopUpStatusFailed
				_ = topUp.Update()
			}
		}
		_, _ = c.Writer.WriteString("success") // 告知 BEpusdt 已收到通知
		return
	}

	if orderId == "" {
		log.Printf("BEpusdt 回调缺少 order_id")
		_, _ = c.Writer.WriteString("fail")
		return
	}

	LockOrder(orderId)
	defer UnlockOrder(orderId)

	topUp := model.GetTopUpByTradeNo(orderId)
	if topUp == nil {
		log.Printf("BEpusdt 回调未找到订单: %s", orderId)
		_, _ = c.Writer.WriteString("fail")
		return
	}

	if topUp.Status == common.TopUpStatusSuccess {
		// 幂等：已处理过
		_, _ = c.Writer.WriteString("success")
		return
	}

	if topUp.Status != common.TopUpStatusPending {
		log.Printf("BEpusdt 订单状态异常: %s, order_id=%s", topUp.Status, orderId)
		_, _ = c.Writer.WriteString("fail")
		return
	}

	// 充值
	if err := model.RechargeByTradeNo(orderId, "bepusdt"); err != nil {
		log.Printf("BEpusdt 充值失败: %v, order_id=%s", err, orderId)
		_, _ = c.Writer.WriteString("fail")
		return
	}

	log.Printf("BEpusdt 充值成功 - 订单: %s", orderId)
	_, _ = c.Writer.WriteString("success")
}
