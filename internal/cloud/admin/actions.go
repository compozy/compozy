// SPDX-License-Identifier: FSL-1.1-MIT

package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/billing"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

type confirmation struct {
	Kind, Label, Consequence              string
	AccountID, Target, Provider, Hostname string
	Generation                            int64
	Cap                                   int32
	Policy                                remoteproto.GatePolicy
	GraceUntil                            string
}

func (c *console) confirm(w http.ResponseWriter, r *http.Request) {
	ctx, query := r.Context(), r.URL.Query()
	action := confirmation{
		Kind: r.PathValue("action"), AccountID: query.Get("account_id"),
		Target: query.Get("target"), Provider: query.Get("provider"),
	}
	data := page{Confirm: &action, Back: "/admin"}
	switch action.Kind {
	case "gate-policy":
		policy, err := c.db.GatePolicy(ctx)
		if err != nil {
			c.fail(w, err)
			return
		}
		action.Label = "Change gate policy"
		action.Consequence = "Applies to installations that fetch the cloud policy."
		action.Policy = policy
		if policy.Until != nil {
			action.GraceUntil = policy.Until.UTC().Format("2006-01-02T15:04")
		}
	case "delete-tunnel":
		tunnel, err := c.db.AdminTunnel(ctx, strings.ToLower(strings.TrimSpace(action.Target)))
		if err != nil {
			c.fail(w, err)
			return
		}
		action.AccountID, action.Hostname, action.Generation = tunnel.AccountID, tunnel.Hostname, tunnel.Generation
		action.Label = "Delete tunnel"
		action.Consequence = "Removes " + tunnel.Hostname + " for " + tunnel.Email + " and prevents it from being recreated."
		data.Back = "/admin/accounts/" + tunnel.AccountID
	default:
		account, err := c.db.AdminAccount(ctx, action.AccountID)
		if err != nil {
			c.fail(w, err)
			return
		}
		if !accountConfirmation(&action, account) {
			c.fail(w, store.ErrNotFound)
			return
		}
		data.Back = "/admin/accounts/" + account.ID
	}
	data.Title = action.Label
	c.render(w, http.StatusOK, data)
}

func accountConfirmation(a *confirmation, account store.AdminAccount) bool {
	email := account.Email
	switch a.Kind {
	case "suspend":
		a.Label, a.Consequence = "Suspend", "Signs out every installation and stops every remote session of "+email+"."
	case "unsuspend":
		a.Label, a.Consequence = "Unsuspend", "Lets "+email+" sign in again."
	case "revoke-device":
		a.Label, a.Consequence = "Revoke device", "Removes "+a.Target+" from "+email+" and ends its remote sessions."
	case "revoke-session":
		a.Label, a.Consequence = "Revoke session", "Signs out session "+a.Target+" for "+email+"."
	case "hosted-route-cap":
		a.Label, a.Consequence = "Hosted-route cap", "Raises the hosted-route cap for "+email+"."
		a.Cap = account.CloudRoutesMax
	case "complimentary-pro":
		a.Label, a.Consequence = "Grant complimentary Pro", "Grants Pro to "+email+" until the date you choose."
	case "detach-identity":
		a.Label, a.Consequence = "Detach identity", "Removes the "+a.Provider+" identity "+a.Target+" from "+email+"."
	case "start-deletion":
		a.Label, a.Consequence = "Start deletion", "Cancels Pro and deletes the cloud account for "+email+"."
	default:
		return false
	}
	return true
}

func (c *console) apply(w http.ResponseWriter, r *http.Request) {
	if !parseForm(w, r) || r.PostForm.Get("confirm") != "yes" {
		c.render(w, http.StatusBadRequest, page{Title: "Confirm this action before continuing."})
		return
	}
	id, err := c.applyAction(r)
	if err != nil {
		c.fail(w, err)
		return
	}
	if id == 0 {
		back := "/admin"
		if account := r.PostForm.Get("account_id"); strings.HasPrefix(account, "acc_") {
			back += "/accounts/" + account
		}
		c.render(w, http.StatusOK, page{Title: "No changes to save", Back: back})
		return
	}
	// #nosec G710 -- A fixed relative path with a formatted event ID.
	http.Redirect(w, r, eventURL(id), http.StatusSeeOther)
}

func (c *console) applyAction(r *http.Request) (int64, error) {
	ctx, form := r.Context(), r.PostForm
	account, email := form.Get("account_id"), operator(r)
	switch r.PathValue("action") {
	case "suspend", "unsuspend":
		return c.accounts.SetAdminSuspension(ctx, account, email, r.PathValue("action") == "suspend")
	case "revoke-device":
		return c.accounts.AdminRevokeDevice(ctx, account, email, remoteproto.DeviceID(form.Get("target")))
	case "revoke-session":
		return c.accounts.AdminRevokeSession(ctx, account, email, form.Get("target"))
	case "delete-tunnel":
		return c.takeDownTunnel(r)
	case "hosted-route-cap":
		cap, err := strconv.ParseInt(form.Get("cap"), 10, 32)
		if err != nil {
			return 0, control.ErrInvalidInput
		}
		return c.accounts.AdminSetHostedRouteCap(ctx, account, email, int32(cap))
	case "complimentary-pro":
		end, ok := parseAdminTime(form.Get("ends_at"))
		if !ok {
			return 0, billing.ErrComplimentaryEnd
		}
		return c.billing.GrantComplimentary(ctx, account, "admin:"+email, end)
	case "detach-identity":
		return c.accounts.AdminDetachIdentity(ctx, account, email,
			form.Get("provider"), form.Get("target"), form.Get("written_request"))
	case "start-deletion":
		return c.accounts.StartAccountDeletion(ctx, account, email)
	case "gate-policy":
		return c.changeGatePolicy(r)
	default:
		return 0, store.ErrNotFound
	}
}

func (c *console) takeDownTunnel(r *http.Request) (int64, error) {
	form := r.PostForm
	target, err := c.db.AdminTunnel(r.Context(), strings.ToLower(strings.TrimSpace(form.Get("hostname"))))
	if err != nil {
		return 0, err
	}
	if strconv.FormatInt(target.Generation, 10) != form.Get("generation") ||
		target.AccountID != form.Get("account_id") {
		return 0, control.ErrInvalidInput
	}
	return c.accounts.AdminTakeDownTunnel(r.Context(), target, operator(r))
}

func (c *console) changeGatePolicy(r *http.Request) (int64, error) {
	form := r.PostForm
	policy := remoteproto.GatePolicy{
		Mode:    remoteproto.GateMode(form.Get("mode")),
		Message: strings.TrimSpace(form.Get("message")),
	}
	if policy.Mode == remoteproto.GateGrace {
		until, ok := parseAdminTime(form.Get("until"))
		if !ok {
			return 0, remoteproto.ErrGatePolicyInvalid
		}
		policy.Until = &until
	}
	return c.db.ChangeGatePolicy(r.Context(), "admin:"+operator(r), policy, c.config.Now())
}

func parseAdminTime(raw string) (time.Time, bool) {
	value, err := time.Parse("2006-01-02T15:04", raw)
	if err != nil {
		value, err = time.Parse(time.RFC3339, raw)
	}
	return value, err == nil
}

func parseForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	return r.ParseForm() == nil
}

func eventURL(id int64) string { return "/admin/events/" + strconv.FormatInt(id, 10) }
