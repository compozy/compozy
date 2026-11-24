package core

import (
	"context"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/remoteproto"
)

type AccountService interface {
	Status() account.Installation
	StartLoginWithConsent(context.Context, string, bool) (account.Login, error)
	SetMarketingConsent(context.Context, bool, string) (account.Installation, error)
	DeleteAccount(context.Context, string) (remoteproto.AccountDeleteResponse, error)
	DismissSignInNotices(context.Context) error
	CancelLogin(context.Context, string) error
	Logout(context.Context) error
	Subscribe(string) (account.Subscription, error)
	SubscribeSignIn() (<-chan account.SignInEvent, func())
	Devices(context.Context) ([]remoteproto.AccountDevice, error)
	PatchDevice(context.Context, string, remoteproto.AccountDevicePatch) (remoteproto.AccountDevice, error)
	RevokeDevice(context.Context, string) error
	Approvals(context.Context) ([]remoteproto.AccountApproval, error)
	Approval(context.Context, string) (remoteproto.AccountApproval, error)
	DecideApproval(context.Context, string, bool) (remoteproto.ApprovalDelivery, error)
	Checkout(context.Context, string) (string, error)
	BillingPortal(context.Context) (string, error)
}

var _ AccountService = (*account.Service)(nil)
