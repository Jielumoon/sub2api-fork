//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

func menuItemPayload(openMode string) map[string]any {
	return map[string]any{
		"custom_menu_items": []map[string]any{{
			"id":         "shop",
			"label":      "购买兑换码",
			"url":        "https://shop.example.com/s/abc",
			"visibility": "user",
			"sort_order": 0,
			"open_mode":  openMode,
		}},
	}
}

func TestUpdateSettingsCustomMenuOpenModeRoundTrips(t *testing.T) {
	for _, mode := range []string{"", "embed", "embed_clean", "new_tab"} {
		t.Run("mode="+mode, func(t *testing.T) {
			h, repo := newStepUpSwitchTestHandler(t, map[string]string{})

			rec := doUpdateSettings(t, h, menuItemPayload(mode), nil)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			var stored []dto.CustomMenuItem
			require.NoError(t, json.Unmarshal([]byte(repo.values[service.SettingKeyCustomMenuItems]), &stored))
			require.Len(t, stored, 1)
			require.Equal(t, mode, stored[0].OpenMode)
		})
	}
}

func TestUpdateSettingsRejectsUnknownCustomMenuOpenMode(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyCustomMenuItems: "[]",
	})

	rec := doUpdateSettings(t, h, menuItemPayload("popup"), nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "open_mode")
	require.Equal(t, "[]", repo.values[service.SettingKeyCustomMenuItems])
}
