package service

import (
	"net/url"
	"testing"
)

func TestProxyURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		proxy Proxy
		want  string
	}{
		{
			name: "without auth",
			proxy: Proxy{
				Protocol: "http",
				Host:     "proxy.example.com",
				Port:     8080,
			},
			want: "http://proxy.example.com:8080",
		},
		{
			name: "with auth",
			proxy: Proxy{
				Protocol: "socks5",
				Host:     "socks.example.com",
				Port:     1080,
				Username: "user",
				Password: "pass",
			},
			want: "socks5://user:pass@socks.example.com:1080",
		},
		{
			name: "username only keeps no auth for compatibility",
			proxy: Proxy{
				Protocol: "http",
				Host:     "proxy.example.com",
				Port:     8080,
				Username: "user-only",
			},
			want: "http://proxy.example.com:8080",
		},
		{
			name: "with special characters in credentials",
			proxy: Proxy{
				Protocol: "http",
				Host:     "proxy.example.com",
				Port:     3128,
				Username: "first last@corp",
				Password: "p@ ss:#word",
			},
			want: "http://first%20last%40corp:p%40%20ss%3A%23word@proxy.example.com:3128",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.proxy.URL(); got != tc.want {
				t.Fatalf("Proxy.URL() mismatch: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestProxyURL_SpecialCharactersRoundTrip(t *testing.T) {
	t.Parallel()

	proxy := Proxy{
		Protocol: "http",
		Host:     "proxy.example.com",
		Port:     3128,
		Username: "first last@corp",
		Password: "p@ ss:#word",
	}

	parsed, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatalf("parse proxy URL failed: %v", err)
	}
	if got := parsed.User.Username(); got != proxy.Username {
		t.Fatalf("username mismatch after parse: got=%q want=%q", got, proxy.Username)
	}
	pass, ok := parsed.User.Password()
	if !ok {
		t.Fatal("password missing after parse")
	}
	if pass != proxy.Password {
		t.Fatalf("password mismatch after parse: got=%q want=%q", pass, proxy.Password)
	}
}

func TestProxyForAccount(t *testing.T) {
	t.Parallel()

	account := &Account{ID: 42, Name: "main"}
	tests := []struct {
		name     string
		username string
		want     string
	}{
		{name: "resin identity", username: "Default.{account_id}", want: "Default.42"},
		{name: "residential session and repeated", username: "user-session-{account_id}-{account_id}", want: "user-session-42-42"},
		// 只支持 {account_id}：其他花括号原样保留
		{name: "account name is not a placeholder", username: "u-{account_name}-{account_id}", want: "u-{account_name}-42"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shared := &Proxy{Protocol: "http", Host: "resin", Port: 2260, Username: tc.username, Password: "token"}

			got := shared.ForAccount(account)
			if got.Username != tc.want {
				t.Fatalf("username mismatch: got=%q want=%q", got.Username, tc.want)
			}
			if shared.Username != tc.username {
				t.Fatalf("shared proxy mutated: %q", shared.Username)
			}
			parsed, err := url.Parse(got.URL())
			if err != nil {
				t.Fatalf("parse proxy URL failed: %v", err)
			}
			if parsed.User.Username() != tc.want {
				t.Fatalf("username mismatch after URL round trip: got=%q want=%q", parsed.User.Username(), tc.want)
			}
		})
	}
}

func TestRenderProxyUsername_EdgeCases(t *testing.T) {
	t.Parallel()

	parentID := int64(7)
	// 影子账号透传母账号凭据，必须和母账号走同一个出口。
	shadow := &Account{ID: 99, Name: "spark", ParentAccountID: &parentID}
	if got := RenderProxyUsername("Default.{account_id}", shadow); got != "Default.7" {
		t.Fatalf("shadow account should render parent id: got=%q", got)
	}

	rendered := (&Proxy{Protocol: "http", Host: "resin", Port: 2260, Username: "Default.{account_id}", Password: "token"}).ForAccount(shadow)
	if rendered.ForAccount(shadow) != rendered {
		t.Fatal("rendered proxy should not be rendered again")
	}

	if got := RenderProxyUsername("Default.{account_id}", nil); got != "Default.{account_id}" {
		t.Fatalf("nil account should leave username untouched: got=%q", got)
	}
}

func TestProxyForAccount_ReturnsSameProxyWithoutPlaceholder(t *testing.T) {
	t.Parallel()

	plain := &Proxy{Protocol: "http", Host: "proxy", Port: 8080, Username: "user", Password: "pass"}
	if got := plain.ForAccount(&Account{ID: 1}); got != plain {
		t.Fatal("proxy without placeholder should be returned as is")
	}

	template := &Proxy{Protocol: "http", Host: "proxy", Port: 8080, Username: "Default.{account_id}", Password: "pass"}
	if got := template.ForAccount(nil); got != template {
		t.Fatal("nil account should leave proxy untouched")
	}

	var nilProxy *Proxy
	if got := nilProxy.ForAccount(&Account{ID: 1}); got != nil {
		t.Fatal("nil proxy should stay nil")
	}
}

func TestProxyURL_StripsPlaceholdersWithoutAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		username string
		want     string
	}{
		{name: "resin template falls back to random routing", username: "Default.{account_id}", want: "http://Default.:token@resin:2260"},
		{name: "placeholder only drops auth", username: "{account_id}", want: "http://resin:2260"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			proxy := Proxy{Protocol: "http", Host: "resin", Port: 2260, Username: tc.username, Password: "token"}
			if got := proxy.URL(); got != tc.want {
				t.Fatalf("Proxy.URL() mismatch: got=%q want=%q", got, tc.want)
			}
		})
	}
}
