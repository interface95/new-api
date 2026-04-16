package setting

var (
	BepusdtEnabled   bool
	BepusdtHost      string  // BEpusdt 服务地址，如 https://pay.example.com
	BepusdtToken     string  // BEpusdt API Token
	BepusdtUnitPrice float64 = 1.0 // 每单位额度对应的 USDT 金额
	BepusdtMinTopUp  int     = 1
	BepusdtNotifyUrl string  // 可选：覆盖默认回调地址
	BepusdtReturnUrl string  // 可选：覆盖默认跳转地址
)
