package repository

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 同一批账号共用一条模板代理时，每个账号要拿到按自己渲染的代理副本。
func TestAccountsToService_RendersSharedTemplateProxyPerAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := newAccountRepositoryWithSQL(client, db, nil)

	proxyID := int64(7)
	mock.ExpectQuery(`FROM "proxies"`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "protocol", "host", "port", "username", "password", "status"}).
			AddRow(proxyID, "resin", "http", "resin", 2260, "Default.{account_id}", "token", service.StatusActive),
	)
	mock.ExpectQuery(`FROM "account_groups"`).WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id", "priority"}))

	newAccount := func(id int64, name string) *dbent.Account {
		return &dbent.Account{
			ID:          id,
			Name:        name,
			Platform:    service.PlatformAnthropic,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{},
			Extra:       map[string]any{},
			Status:      service.StatusActive,
			ProxyID:     &proxyID,
		}
	}

	got, err := repo.accountsToService(context.Background(), []*dbent.Account{newAccount(11, "a"), newAccount(12, "b")})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, got, 2)
	require.Equal(t, "Default.11", got[0].Proxy.Username)
	require.Equal(t, "Default.12", got[1].Proxy.Username)
	require.Equal(t, "http://Default.11:token@resin:2260", got[0].Proxy.URL())
}

// GetByIDs 走 ent 预加载的 proxy 边，也要按账号渲染。
func TestGetByIDs_RendersTemplateProxyPerAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := newAccountRepositoryWithSQL(client, db, nil)

	mock.ExpectQuery(`FROM "accounts"`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "platform", "type", "credentials", "extra", "status", "proxy_id"}).
			AddRow(11, "a", service.PlatformAnthropic, service.AccountTypeOAuth, []byte(`{}`), []byte(`{}`), service.StatusActive, 7).
			AddRow(12, "b", service.PlatformAnthropic, service.AccountTypeOAuth, []byte(`{}`), []byte(`{}`), service.StatusActive, 7),
	)
	mock.ExpectQuery(`FROM "proxies"`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "protocol", "host", "port", "username", "password", "status"}).
			AddRow(7, "resin", "http", "resin", 2260, "Default.{account_id}", "token", service.StatusActive),
	)
	mock.ExpectQuery(`FROM "account_groups"`).WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id", "priority"}))

	got, err := repo.GetByIDs(context.Background(), []int64{11, 12})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, got, 2)
	require.Equal(t, "Default.11", got[0].Proxy.Username)
	require.Equal(t, "Default.12", got[1].Proxy.Username)
}

// 探测/用量快照写库前会拿库里的代理行和 account.Proxy 比对身份；
// account.Proxy 已按账号渲染，库里是模板原文，必须按同一账号渲染后再比。
func TestLockAndMatchProbeProxyIdentity_TemplateUsername(t *testing.T) {
	tests := []struct {
		name       string
		dbUsername string
		want       bool
	}{
		{name: "same template", dbUsername: "Default.{account_id}", want: true},
		{name: "template changed", dbUsername: "Other.{account_id}", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })

			mock.ExpectQuery(`(?s)SELECT protocol, host, port.*FOR SHARE`).WithArgs(int64(9)).WillReturnRows(
				sqlmock.NewRows([]string{"protocol", "host", "port", "username", "password", "status"}).
					AddRow("http", "resin", 2260, tt.dbUsername, "token", service.StatusActive),
			)
			proxyID := int64(9)
			account := &service.Account{ID: 17, ProxyID: &proxyID}
			account.Proxy = (&service.Proxy{
				ID: proxyID, Protocol: "http", Host: "resin", Port: 2260,
				Username: "Default.{account_id}", Password: "token", Status: service.StatusActive,
			}).ForAccount(account)

			got, err := lockAndMatchProbeProxyIdentity(context.Background(), client, account)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
