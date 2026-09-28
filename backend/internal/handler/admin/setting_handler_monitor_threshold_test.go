//go:build unit

package admin

import (
	"bytes"
	"log/slog"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

func TestUpdateSettingsChannelMonitorDegradedThresholdMs(t *testing.T) {
	cases := []struct {
		name string
		sent int
		want string
	}{
		{"0 表示不因耗时降级，必须原样保存", 0, "0"},
		{"正常值", 9000, "9000"},
		{"负数夹到 0", -5, "0"},
		{"超过请求超时夹到 45000", 99999, "45000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, repo := newStepUpSwitchTestHandler(t, map[string]string{
				service.SettingKeyChannelMonitorDegradedThresholdMs: "6000",
			})
			rec := doUpdateSettings(t, h, map[string]any{"channel_monitor_degraded_threshold_ms": tc.sent}, nil)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, tc.want, repo.values[service.SettingKeyChannelMonitorDegradedThresholdMs])
		})
	}
}

func TestUpdateSettingsKeepsChannelMonitorDegradedThresholdWhenOmitted(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyChannelMonitorDegradedThresholdMs: "0",
	})
	rec := doUpdateSettings(t, h, map[string]any{"risk_control_enabled": true}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "0", repo.values[service.SettingKeyChannelMonitorDegradedThresholdMs])
}

func TestDiffSettingsReportsChannelMonitorDegradedThreshold(t *testing.T) {
	before := &service.SystemSettings{ChannelMonitorDegradedThresholdMs: 6000}
	after := &service.SystemSettings{ChannelMonitorDegradedThresholdMs: 10000}
	require.Contains(t, diffSettings(before, after, nil, nil, UpdateSettingsRequest{}), "channel_monitor_degraded_threshold_ms")
}

// 审计比较的必须是落库后的值：原值 0 再提交 -5，落库仍是 0，不能记为变更。
func TestUpdateSettingsAuditUsesClampedDegradedThreshold(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyChannelMonitorDegradedThresholdMs: "0",
	})
	rec := doUpdateSettings(t, h, map[string]any{"channel_monitor_degraded_threshold_ms": -5}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "0", repo.values[service.SettingKeyChannelMonitorDegradedThresholdMs])
	require.NotContains(t, logs.String(), "channel_monitor_degraded_threshold_ms")

	rec = doUpdateSettings(t, h, map[string]any{"channel_monitor_degraded_threshold_ms": 9000}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, logs.String(), "channel_monitor_degraded_threshold_ms")
}
