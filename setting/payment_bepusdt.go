package setting

import "strings"

var (
	BepusdtEnabled    bool
	BepusdtGatewayURL string
	BepusdtAuthToken  string
	BepusdtCurrencies string = "USDT"
	BepusdtFiat       string = "CNY"
	BepusdtReturnURL  string
	BepusdtUnitPrice  float64 = 1.0
	BepusdtMinTopUp   int     = 1
)

func NormalizeBepusdtCurrencies(value string) string {
	currencies := strings.Split(value, ",")
	normalized := make([]string, 0, len(currencies))
	seen := make(map[string]struct{}, len(currencies))
	for _, currency := range currencies {
		name := strings.ToUpper(strings.TrimSpace(currency))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	return strings.Join(normalized, ",")
}
