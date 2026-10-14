package admin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 响应只能出现掩码与「是否已配置」，不得带出密文或明文 Key。
func TestUpstreamResponseNeverExposesKey(t *testing.T) {
	u := &service.Upstream{
		ID:              1,
		Name:            "主站",
		BaseURL:         "https://upstream.example",
		APIKeyEncrypted: "ciphertext-should-not-leak",
		APIKeyMasked:    service.MaskUpstreamAPIKey("sk-plaintext-secret"),
		Enabled:         true,
		IntervalMinutes: 15,
		CreatedAt:       time.Unix(0, 0),
		UpdatedAt:       time.Unix(0, 0),
	}
	raw, err := json.Marshal(upstreamToResponse(u))
	require.NoError(t, err)
	body := string(raw)
	require.NotContains(t, body, "ciphertext-should-not-leak")
	require.NotContains(t, body, "sk-plaintext-secret")
	require.NotContains(t, body, "api_key_encrypted")
	require.Contains(t, body, `"api_key_masked":"sk-p***"`)
	require.Contains(t, body, `"api_key_configured":true`)
}
