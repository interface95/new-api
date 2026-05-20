package setting

var (
	BepusdtEnabled    bool
	BepusdtGatewayURL string
	BepusdtAuthToken  string
	BepusdtTradeType  string = "usdt.trc20"
	BepusdtFiat       string = "CNY"
	BepusdtReturnURL  string
	BepusdtUnitPrice  float64 = 1.0
	BepusdtMinTopUp   int     = 1
)
