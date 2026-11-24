package remotehost

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

const linkDigestRef = "link-token-digest"

type LinkResult struct {
	ComputerID remoteproto.ComputerID
	Name       string
	Enabled    bool
	Login      *account.Login
}

func (h *Host) Link(ctx context.Context, token string) (LinkResult, error) {
	if token == "" && h.account.Status().State != account.SignedIn && h.Status().ComputerID == "" {
		return h.beginLinkLogin(ctx)
	}
	return h.link(ctx, token)
}

func (h *Host) beginLinkLogin(ctx context.Context) (LinkResult, error) {
	events, release := h.account.SubscribeSignIn()
	login, err := h.account.StartLogin(ctx, "")
	if err != nil {
		release()
		return LinkResult{}, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		release()
		return LinkResult{}, context.Canceled
	}
	h.workers.Go(func() {
		defer release()
		h.finishLinkLogin(events, login)
	})
	return LinkResult{Login: &login}, nil
}

func (h *Host) finishLinkLogin(events <-chan account.SignInEvent, login account.Login) {
	for {
		select {
		case <-h.ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.AttemptID != login.AttemptID {
				continue
			}
			switch event.State {
			case "signed_in":
				if _, err := h.link(h.ctx, ""); err != nil {
					h.opts.Logger.WarnContext(h.ctx, "Could not link this computer", "code", hostFailureCode(err))
				}
				return
			case "failed", "cancelled", "expired": //nolint:misspell // Public protocol state.
				return
			}
		}
	}
}

func (h *Host) link(ctx context.Context, token string) (LinkResult, error) {
	identity, err := h.registerComputer(ctx, token)
	if err != nil {
		return LinkResult{}, err
	}
	status := h.Status()
	if !status.Enabled {
		if status, err = h.Patch(ctx, Patch{Enabled: new(true)}); err != nil {
			return LinkResult{}, err
		}
	}
	return LinkResult{ComputerID: identity.ComputerID, Name: h.name, Enabled: status.Enabled}, nil
}

func (h *Host) registerComputer(ctx context.Context, token string) (Identity, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return Identity{}, context.Canceled
	}
	h.forgetUnlinkedLocked()
	if h.identity.ComputerID != "" {
		return h.existingLink(token)
	}
	keys, err := loadHostKeys(h.account, Identity{}, h.opts.Now())
	if err != nil {
		return Identity{}, err
	}
	certificate, err := keys.ca.certificate(h.opts.Now())
	if err != nil {
		return Identity{}, err
	}
	jwk, err := remoteproto.NewPublicJWK(keys.signing.Public())
	if err != nil {
		return Identity{}, err
	}
	request := cloudclient.ComputerLinkRequest{
		Name: h.name, Platform: h.platform, Version: h.version, ComputerJWK: jwk,
		TLSRootSHA256: keys.ca.fingerprint(), LinkToken: token,
	}
	reply, err := h.submitLink(ctx, request, keys)
	if err != nil {
		return Identity{}, err
	}
	if reply.AccountID == "" || reply.ComputerID == "" {
		return Identity{}, errors.New("cloud returned an incomplete computer identity")
	}
	identity := Identity{ComputerID: reply.ComputerID, AccountID: reply.AccountID,
		ComputerKeyRef: computerKeyRef, TLSRootRef: tlsRootRef, TLSRootSHA256: request.TLSRootSHA256,
		TLSLeafNotAfter: certificate.Leaf.NotAfter, LinkedAt: h.opts.Now().UTC()}
	save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := h.store.SaveRemoteIdentity(save, identity); err != nil {
		return Identity{}, err
	}
	h.identity = identity
	return identity, nil
}

func (h *Host) existingLink(token string) (Identity, error) {
	if token != "" {
		digest, present, err := h.account.GetHostSecret(linkDigestRef)
		if err != nil {
			return Identity{}, err
		}
		if !present || !ConstantTimeCredentialMatch(token, digest) {
			return Identity{}, conflict("computer_already_linked", "This computer is already linked to an account.")
		}
	} else if current := h.account.Status().AccountID; current != "" && current != h.identity.AccountID {
		return Identity{}, conflict(
			"installation_linked_to_other_account",
			"Unlink this computer before linking another account.",
		)
	}
	return h.identity, nil
}

func (h *Host) submitLink(
	ctx context.Context,
	request cloudclient.ComputerLinkRequest,
	keys hostKeys,
) (cloudclient.ComputerLink, error) {
	if request.LinkToken != "" {
		if err := h.account.PutHostSecret(linkDigestRef, HashCredential(request.LinkToken)); err != nil {
			return cloudclient.ComputerLink{}, err
		}
		return h.cloud.LinkComputer(ctx, request, cloudclient.Credentials{}, keys.signing)
	}
	if err := h.account.DeleteHostSecret(linkDigestRef); err != nil {
		return cloudclient.ComputerLink{}, err
	}
	var reply cloudclient.ComputerLink
	err := h.account.UseDevice(ctx, func(ctx context.Context, creds cloudclient.Credentials) error {
		var err error
		reply, err = h.cloud.LinkComputer(ctx, request, creds, keys.signing)
		return err
	})
	return reply, err
}

func (h *Host) Unlink(ctx context.Context) error {
	if err := h.unlink(ctx); err != nil {
		return err
	}
	_, err := h.Patch(ctx, Patch{Enabled: new(false), CloudRoute: new(false)})
	return err
}

func (h *Host) unlink(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.forgetUnlinkedLocked()
	identity := h.identity
	if identity.ComputerID == "" {
		return nil
	}
	if err := h.account.UseDevice(ctx, func(ctx context.Context, creds cloudclient.Credentials) error {
		if h.account.Status().AccountID != identity.AccountID {
			return account.ErrSignInRequired
		}
		return h.cloud.UnlinkComputer(ctx, identity.ComputerID, creds)
	}); err != nil && cloudclient.FailureCode(err) != "computer_not_found" {
		return err
	}
	if h.run != nil {
		h.run.authority.retire()
	}
	h.stopLocked()
	save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := h.store.WriteRemote(
		save,
		func(ctx context.Context, tx Tx) error { return tx.ClearLink(ctx) },
	); err != nil {
		return err
	}
	h.deleteKeys(save, identity)
	h.identity = Identity{}
	return nil
}

func hostFailureCode(err error) string {
	if failure, ok := errors.AsType[*cloudclient.Failure](err); ok {
		return failure.Code
	}
	return DescribeError(err).Code
}
