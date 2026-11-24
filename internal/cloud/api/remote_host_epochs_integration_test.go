// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil"
)

func TestCloudRemoteHostEpochs(t *testing.T) {
	t.Run("Should discard a stale snapshot and stop mints behind an incremental gap", func(t *testing.T) {
		// IT-081. Safety Invariants 6, 22 and 23.
		f, first, second, relay := remoteFailoverFixture(t)
		lab := newRemoteHostLab(t, f, t.TempDir())
		other := enrollRemoteDevice(t, f, remoteFixtureEmail, "remote-lab-epoch-revocation")
		f.db.Exec(t, "UPDATE %s.accounts SET security_epoch = 10 WHERE id = $1", lab.owner.account)
		tunnel := lab.start(t, lab.db, nil)
		epoch := &remoteEpochScript{
			ctx: t.Context(), first: make(chan struct{}, 1), missing: make(chan struct{}, 1),
			requested: make(chan remoteproto.SnapshotRequest, 4), release: make(chan struct{}),
		}
		relay.rules(epoch.deliver, epoch.recordRequest)
		replicaOne, replicaTwo := f, f
		replicaOne.replica, replicaTwo.replica = first, second
		changeRemoteScopes(t, replicaOne, lab, lab.member, remoteproto.ScopeSessions, remoteproto.ScopeFilesR)
		receiveRemote(t, epoch.first, 2*time.Second)
		changeRemoteScopes(t, replicaTwo, lab, lab.member, remoteproto.ScopeSessions, remoteproto.ScopeFilesW)
		testutil.Eventually(t, "epoch 12 applied ahead of delayed 11", 2*time.Second, func() bool {
			snapshot := lab.readSnapshot(t)
			return snapshot.SecurityEpoch == 12 &&
				slices.Contains(snapshotScopes(snapshot, lab.member.id), remoteproto.ScopeFilesW)
		})

		changeRemoteScopes(t, replicaOne, lab, lab.member, remoteproto.ScopeSessions)
		receiveRemote(t, epoch.missing, 2*time.Second)
		remoteCloudRequest(t, replicaTwo, lab.owner, http.MethodDelete, "/v1/devices/"+string(other.id), nil).
			expect(t, http.StatusNoContent, "")
		request := receiveRemote(t, epoch.requested, 2*time.Second)
		if request.AppliedEpoch != 12 {
			t.Fatalf("gap repair requested at %d, want 12", request.AppliedEpoch)
		}
		grant := fetchRemoteGrant(t, f, lab.member, lab.host.Status().ComputerID)
		status, body := tunnel.Mint(t, lab.member.key, grant, f.clock.Now())
		if status != http.StatusServiceUnavailable || !bytes.Contains(body, []byte("security_snapshot_pending")) {
			t.Fatalf("future mint was not held: %d %s", status, body)
		}
		close(epoch.release)
		testutil.Eventually(t, "epoch 14 repaired before mint", 2*time.Second, func() bool {
			snapshot := lab.readSnapshot(t)
			return snapshot.SecurityEpoch == 14 && snapshotScopes(snapshot, other.id) == nil
		})
		lab.mint(t, tunnel, lab.member)
	})
}

type remoteEpochScript struct {
	ctx            context.Context
	mu             sync.Mutex
	delayed        remoteproto.ControlMessage
	first, missing chan struct{}
	requested      chan remoteproto.SnapshotRequest
	release        chan struct{}
}

func (s *remoteEpochScript) deliver(message remoteproto.ControlMessage) []remoteproto.ControlMessage {
	switch message.SecurityEpoch {
	case 11, 12:
		return s.reverseSnapshots(message)
	case 13:
		if message.Type == remoteproto.ControlSnapshot {
			select {
			case s.missing <- struct{}{}:
			default:
			}
		}
		return nil
	case 14:
		if message.Type == remoteproto.ControlSnapshot {
			select {
			case <-s.release:
			case <-s.ctx.Done():
				return nil
			}
		}
	}
	return []remoteproto.ControlMessage{message}
}

func (s *remoteEpochScript) reverseSnapshots(message remoteproto.ControlMessage) []remoteproto.ControlMessage {
	if message.Type == remoteproto.ControlScopesChanged {
		return nil
	}
	if message.Type != remoteproto.ControlSnapshot {
		return []remoteproto.ControlMessage{message}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if message.SecurityEpoch == 11 {
		s.delayed = message
		select {
		case s.first <- struct{}{}:
		default:
		}
		return nil
	}
	if s.delayed.Type != "" {
		older := s.delayed
		s.delayed = remoteproto.ControlMessage{}
		return []remoteproto.ControlMessage{message, older}
	}
	return []remoteproto.ControlMessage{message}
}

func (s *remoteEpochScript) recordRequest(message remoteproto.ControlMessage) {
	if message.Type != remoteproto.ControlSnapshotRequest {
		return
	}
	var request remoteproto.SnapshotRequest
	if json.Unmarshal(message.Data, &request) == nil {
		select {
		case s.requested <- request:
		default:
		}
	}
}
