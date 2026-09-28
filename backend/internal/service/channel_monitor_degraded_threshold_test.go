//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFinalizeOperationalOrDegradedThreshold(t *testing.T) {
	cases := []struct {
		name      string
		latency   time.Duration
		threshold time.Duration
		want      string
	}{
		{"默认阈值以下", 5999 * time.Millisecond, 6 * time.Second, MonitorStatusOperational},
		{"默认阈值边界", 6 * time.Second, 6 * time.Second, MonitorStatusDegraded},
		{"放宽到 10s 后 7s 正常", 7 * time.Second, 10 * time.Second, MonitorStatusOperational},
		{"放宽到 10s 后 12s 降级", 12 * time.Second, 10 * time.Second, MonitorStatusDegraded},
		{"阈值 0 不因耗时降级", 44 * time.Second, 0, MonitorStatusOperational},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := finalizeOperationalOrDegraded(&CheckResult{}, tc.latency, tc.threshold, int(tc.latency/time.Millisecond))
			require.Equal(t, tc.want, res.Status)
		})
	}
}

func TestParseChannelMonitorDegradedThresholdMs(t *testing.T) {
	cases := map[string]int{
		"":      6000,
		"abc":   6000,
		"-5":    0,
		"0":     0,
		"9000":  9000,
		"99999": 45000,
	}
	for raw, want := range cases {
		require.Equal(t, want, parseChannelMonitorDegradedThresholdMs(raw), "raw=%q", raw)
	}
}

// 阈值要从运行时配置一路传到判定：默认配置下慢响应仍判 degraded，关闭后不再判。
func TestRunChecksConcurrentUsesRuntimeDegradedThreshold(t *testing.T) {
	swapMonitorHTTPClient(t)
	inner := &captureHandler{respondText: "pong"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	monitor := &ChannelMonitor{
		Provider:         MonitorProviderAnthropic,
		Endpoint:         srv.URL,
		APIKey:           "sk-fake",
		PrimaryModel:     "claude-x",
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride:     map[string]any{"model": "x", "messages": []any{}},
	}
	svc := &ChannelMonitorService{}

	slow := svc.runChecksConcurrent(context.Background(), monitor, ChannelMonitorRuntime{DegradedThresholdMs: 10})
	require.Equal(t, MonitorStatusDegraded, slow[0].Status)

	disabled := svc.runChecksConcurrent(context.Background(), monitor, ChannelMonitorRuntime{DegradedThresholdMs: 0})
	require.Equal(t, MonitorStatusOperational, disabled[0].Status)
}

// RunCheck 必须把自己读到的运行时阈值传给检测，而不是零值（零值会悄悄变成永不降级）。
func TestRunCheckPassesRuntimeDegradedThresholdToProbe(t *testing.T) {
	swapMonitorHTTPClient(t)
	inner := &captureHandler{respondText: "pong"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	repo := &quotaModeRepoStub{monitor: &ChannelMonitor{
		ID:               7,
		Provider:         MonitorProviderAnthropic,
		Endpoint:         srv.URL,
		APIKey:           "OLD:sk-fake",
		PrimaryModel:     "claude-x",
		Enabled:          true,
		IntervalSeconds:  60,
		CheckMode:        MonitorCheckModeProbe,
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride:     map[string]any{"model": "x", "messages": []any{}},
	}}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(channelMonitorRuntimeStub{rt: ChannelMonitorRuntime{
		Enabled:             true,
		Mode:                ChannelMonitorModeV1,
		DegradedThresholdMs: 10,
	}})

	results, err := svc.RunCheck(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, MonitorStatusDegraded, results[0].Status)
	require.Len(t, repo.history, 1)
	require.Equal(t, MonitorStatusDegraded, repo.history[0].Status)
}

func TestChannelMonitorRuntimeDefaultsDegradedThreshold(t *testing.T) {
	var nilService *SettingService
	require.Equal(t, 6000, nilService.GetChannelMonitorRuntime(context.Background()).DegradedThresholdMs)
}
