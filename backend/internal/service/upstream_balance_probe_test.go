//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseUpstreamBalance(t *testing.T) {
	for _, tc := range []struct {
		name, kind, body, reason string
		remaining                float64
		unlimited                bool
	}{
		{name: "钱包零余额", kind: "wallet", body: `{"mode":"unrestricted","balance":0,"unit":"USD"}`},
		{name: "钱包欠费", kind: "wallet", body: `{"mode":"unrestricted","balance":-1}`, remaining: -1},
		{name: "不拿根剩余替代钱包", kind: "wallet", body: `{"mode":"unrestricted","remaining":10}`, reason: "unavailable"},
		{name: "额度模式不含钱包", kind: "wallet", body: `{"mode":"quota_limited","remaining":20}`, reason: "unavailable"},
		{name: "Key总额度", kind: "key_total", body: `{"mode":"quota_limited","quota":{"limit":20,"used":7,"remaining":13}}`, remaining: 13},
		{name: "Key周期选定窗口", kind: "key_1d", body: `{"mode":"quota_limited","rate_limits":[{"window":"5h","limit":10,"used":2,"remaining":8},{"window":"1d","limit":20,"used":3,"remaining":17}]}`, remaining: 17},
		{name: "未配置的窗口不可用", kind: "key_7d", body: `{"mode":"quota_limited","rate_limits":[]}`, reason: "unavailable"},
		{name: "订阅日额度", kind: "subscription_daily", body: `{"mode":"unrestricted","subscription":{"daily_limit_usd":12,"daily_usage_usd":4}}`, remaining: 8},
		{name: "订阅超额归零", kind: "subscription_weekly", body: `{"mode":"unrestricted","subscription":{"weekly_limit_usd":12,"weekly_usage_usd":14}}`},
		{name: "明确无周期限额", kind: "subscription_monthly", body: `{"mode":"unrestricted","subscription":{"monthly_limit_usd":null,"monthly_usage_usd":3}}`, unlimited: true},
		{name: "缺字段不是无限制", kind: "subscription_monthly", body: `{"mode":"unrestricted","subscription":{"monthly_usage_usd":3}}`, reason: "unavailable"},
		{name: "综合无限制", kind: "subscription_remaining", body: `{"mode":"unrestricted","remaining":-1,"subscription":{"daily_usage_usd":0}}`, unlimited: true},
		{name: "错误的负综合额度", kind: "subscription_remaining", body: `{"mode":"unrestricted","remaining":-2,"subscription":{"daily_usage_usd":0}}`, reason: "invalid_response"},
		{name: "非数值", kind: "wallet", body: `{"mode":"unrestricted","balance":"12"}`, reason: "invalid_response"},
		{name: "非有限值", kind: "wallet", body: `{"mode":"unrestricted","balance":1e999}`, reason: "invalid_response"},
		{name: "错误单位", kind: "wallet", body: `{"mode":"unrestricted","balance":12,"unit":"CNY"}`, reason: "invalid_response"},
		{name: "未知协议", kind: "wallet", body: `{"balance":12}`, reason: "unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, reason := parseUpstreamBalance([]byte(tc.body), tc.kind)
			require.Equal(t, tc.reason, reason)
			if reason != "" {
				require.Nil(t, data)
				return
			}
			require.Equal(t, "USD", data.Unit)
			require.Equal(t, tc.unlimited, data.Unlimited)
			if !tc.unlimited {
				require.NotNil(t, data.Remaining)
				require.Equal(t, tc.remaining, *data.Remaining)
			}
		})
	}
}
