//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type creatingProxyRepoStub struct {
	*proxyRepoStub
	created *Proxy
}

func (s *creatingProxyRepoStub) Create(_ context.Context, proxy *Proxy) error {
	s.created = proxy
	return nil
}

func TestAdminProxyCreate_UsernameTemplateRequiresPassword(t *testing.T) {
	create := func(username, password string) (*creatingProxyRepoStub, error) {
		repo := &creatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}}
		svc := &adminServiceImpl{proxyRepo: repo}
		_, err := svc.CreateProxy(context.Background(), &CreateProxyInput{
			Name: "resin", Protocol: "http", Host: "resin", Port: 2260, Username: username, Password: password,
		})
		return repo, err
	}

	repo, err := create("Default.{account_id}", "")
	require.Equal(t, "PROXY_TEMPLATE_PASSWORD_REQUIRED", infraerrors.Reason(err))
	require.Nil(t, repo.created, "拒绝时不能落库")

	repo, err = create("Default.{account_id}", "token")
	require.NoError(t, err)
	require.Equal(t, "Default.{account_id}", repo.created.Username, "落库的是模板原文")

	// 兼容：普通用户名不带密码仍允许创建。
	_, err = create("user-only", "")
	require.NoError(t, err)
}

func TestAdminProxyUpdate_UsernameTemplateRequiresPassword(t *testing.T) {
	update := func(input UpdateProxyInput) (*updatingProxyRepoStub, error) {
		repo := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}, proxy: &Proxy{
			ID: 9, Username: "old-user", Password: "old-pass", FallbackMode: FallbackModeNone,
		}}
		svc := &adminServiceImpl{proxyRepo: repo}
		_, err := svc.UpdateProxy(context.Background(), 9, &input)
		return repo, err
	}
	template, empty := "Default.{account_id}", ""

	repo, err := update(UpdateProxyInput{Username: &template})
	require.NoError(t, err, "沿用已有密码")
	require.Equal(t, 1, repo.updateCalls)

	repo, err = update(UpdateProxyInput{Username: &template, Password: &empty})
	require.Equal(t, "PROXY_TEMPLATE_PASSWORD_REQUIRED", infraerrors.Reason(err))
	require.Zero(t, repo.updateCalls)
}
