package sshsetup

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
)

type remoteStatus struct {
	ComputerID remoteproto.ComputerID `json:"computer_id"`
	Enabled    bool                   `json:"enabled"`
	Listener   string                 `json:"listener"`
}

func (m *Manager) link(job *setupRun, t target) error {
	m.mu.Lock()
	if err := job.ctx.Err(); err != nil {
		m.mu.Unlock()
		return err
	}
	job.linking = make(chan struct{})
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		close(job.linking)
		job.linking = nil
		m.mu.Unlock()
	}()

	status, err := m.remoteStatus(job, t)
	if err != nil {
		return err
	}
	expected := remoteproto.ComputerID(job.row.ComputerID)
	if status.ComputerID != "" && (expected == "" || expected == status.ComputerID) {
		if err := m.checkLinkedComputer(job, status.ComputerID); err != nil {
			return err
		}
		if status.Enabled {
			return m.commitLink(job, status.ComputerID, expected == "")
		}
	}
	if expected != "" && status.ComputerID != "" && expected != status.ComputerID {
		return linkMismatch()
	}
	token, err := m.linkToken(job)
	if err != nil {
		return err
	}
	result, err := m.remote(job, t, binaryCommand(job)+" remote link --token-stdin --json",
		strings.NewReader(token+"\n"), commandBudget)
	if err != nil {
		return sshFailure(t, result, err)
	}
	if err := json.Unmarshal(result.stdout, &status); err != nil || status.ComputerID == "" ||
		(expected != "" && expected != status.ComputerID) {
		return errInterrupted
	}
	if err := m.checkLinkedComputer(job, status.ComputerID); err != nil {
		return err
	}
	return m.commitLink(job, status.ComputerID, false)
}

func (m *Manager) checkLinkedComputer(job *setupRun, id remoteproto.ComputerID) error {
	return m.device(job.ctx, job.identity, func(ctx context.Context, creds cloudclient.Credentials) error {
		computers, err := m.cloud.Computers(ctx, creds)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(computers, func(item remoteproto.AccountComputer) bool {
			return item.ID == id && item.AccountID == job.row.AccountID
		}) {
			return linkMismatch()
		}
		return nil
	})
}

func linkMismatch() error {
	return failure(http.StatusConflict, "installation_linked_to_other_account",
		"This computer is linked to another account.")
}

func (m *Manager) commitLink(job *setupRun, id remoteproto.ComputerID, releaseUnused bool) error {
	if releaseUnused {
		if err := m.release(job.ctx, job.identity, job.row.ReservationID); err != nil && !errors.Is(err, ErrLinked) {
			return err
		}
	}
	job.row.ComputerID = string(id)
	ref := job.row.LinkTokenRef
	job.row.LinkTokenRef, job.row.LinkTokenExpiresAt = "", ""
	if err := m.advance(job, StepForward); err != nil {
		job.row.LinkTokenRef = ref
		return err
	}
	m.discardToken(ref)
	return nil
}

func (m *Manager) linkToken(job *setupRun) (string, error) {
	row := &job.row
	ref := row.LinkTokenRef
	if ref != "" {
		expiry, err := store.ParseTimestamp(row.LinkTokenExpiresAt)
		if row.ComputerID != "" || (err == nil && time.Until(expiry) > 0) {
			token, found, err := m.accounts.GetHostSecret(ref)
			if err != nil {
				return "", err
			}
			if found {
				return token, nil
			}
		}
	}
	var issued cloudclient.ComputerLinkToken
	err := m.device(job.ctx, job.identity, func(ctx context.Context, creds cloudclient.Credentials) error {
		var err error
		issued, err = m.cloud.NewComputerLinkToken(ctx, creds, row.ReservationID)
		return err
	})
	if err != nil {
		return "", err
	}
	if issued.Token == "" {
		return "", errInterrupted
	}
	if ref == "" {
		ref = "ssh-setup:" + row.ID
	}
	if err := m.accounts.PutHostSecret(ref, issued.Token); err != nil {
		return "", err
	}
	row.LinkTokenRef, row.LinkTokenExpiresAt = ref, store.FormatTimestamp(issued.ExpiresAt)
	if err := m.persist(job.ctx, row); err != nil {
		m.discardToken(ref)
		return "", err
	}
	return issued.Token, nil
}

func (m *Manager) remoteStatus(job *setupRun, t target) (remoteStatus, error) {
	result, err := m.remote(job, t, binaryCommand(job)+" remote status --json", nil, commandBudget)
	if err != nil {
		return remoteStatus{}, sshFailure(t, result, err)
	}
	var status remoteStatus
	if err := json.Unmarshal(result.stdout, &status); err != nil {
		return remoteStatus{}, errInterrupted
	}
	return status, nil
}

func (m *Manager) connectComputer(job *setupRun, t target) error {
	status, err := m.remoteStatus(job, t)
	if err != nil {
		return err
	}
	if status.ComputerID == "" || string(status.ComputerID) != job.row.ComputerID {
		return linkMismatch()
	}
	listener, err := netip.ParseAddrPort(status.Listener)
	if err != nil || !listener.Addr().Unmap().IsLoopback() || listener.Port() == 0 {
		return errInterrupted
	}
	route := t.route()
	route.SSH.TunnelAddress = listener.String()
	route.SSH.ServiceWarning = job.row.ServiceWarning
	if err := m.registry.RegisterSSHComputer(
		job.ctx,
		job.row.AccountID,
		job.row.Name,
		status.ComputerID,
		route,
	); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(job.ctx, 2*commandBudget)
	defer cancel()
	return m.registry.WaitForSSHComputer(ctx, job.row.AccountID, status.ComputerID)
}
