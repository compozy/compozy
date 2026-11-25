// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration && !windows

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json/v2"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const cloudProcessMode = "COMPOZY_TEST_CLOUD_PROCESS"

func TestMain(m *testing.M) {
	switch os.Getenv(cloudProcessMode) {
	case "command":
		main()
		os.Exit(0)
	case "drain":
		if err := serveDrainFixture(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	default:
		os.Exit(m.Run())
	}
}

func TestCloudMigrations(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize concurrent migrate up commands against an empty schema (IT-110)", func(t *testing.T) {
		t.Parallel()
		db := remotetest.NewDatabase(t)
		ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
		defer cancel()
		var commands []*exec.Cmd
		var outputs [2]bytes.Buffer
		for i := range outputs {
			cmd := migrateCommand(t, ctx, db.Config(t, fmt.Sprintf("migrate_%d", i)).DirectDSN)
			cmd.Stdout, cmd.Stderr = &outputs[i], &outputs[i]
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					cmd.Process.Kill()
					cmd.Wait()
				}
			})
			commands = append(commands, cmd)
		}
		var applied [2]int
		var versions [2]int64
		for i, cmd := range commands {
			if err := cmd.Wait(); err != nil {
				t.Fatalf("migration %d: %v, %s", i, err, outputs[i].String())
			}
			versions[i], applied[i] = parseMigrationReport(t, outputs[i].String())
		}
		var version int64
		var count int
		err := db.Pool.QueryRow(ctx,
			"SELECT max(version_id), count(*) - 1 FROM "+db.Schema+".goose_db_version WHERE is_applied",
		).Scan(&version, &count)
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(applied[:])
		if count < 3 || applied != [2]int{0, count} || versions != [2]int64{version, version} {
			t.Fatalf("processes applied %v at versions %v, database version = %d with %d migrations",
				applied, versions, version, count)
		}
		output, err := migrateCommand(t, ctx, db.Config(t, "migrate_again").DirectDSN).CombinedOutput()
		if err != nil {
			t.Fatalf("repeat migration: %v, %s", err, output)
		}
		if gotVersion, gotCount := parseMigrationReport(t, string(output)); gotVersion != version || gotCount != 0 {
			t.Fatalf("third migration: %s", output)
		}
	})
}

func TestCloudDrain(t *testing.T) {
	t.Parallel()

	t.Run("Should send going_away on SIGTERM and leave another replica ready (IT-113)", func(t *testing.T) {
		t.Parallel()
		db := remotetest.NewDatabase(t)
		db.Migrate(t)
		now := time.Now().UTC()
		config := remotetest.ControlConfig(t, time.Now, nil)
		db.SeedAccount(t, "acc_drain")
		device := db.SeedControlDevice(t, "acc_drain", "dev_drain", true, config.Keys.Signer(), now)
		computerKey := db.SeedControlComputer(t, device.AccountID, "cmp_drain", now)
		ctx, cancel := context.WithTimeout(t.Context(), 65*time.Second)
		defer cancel()
		firstURL, process := startDrainFixture(t, ctx, db)
		config.Postgres = db.Config(t, "peer")
		peer := remotetest.StartReplica(t, config, nil)

		link := remotetest.AuthenticateControlLink(t, ctx, firstURL, "cmp_drain", computerKey)
		first := remotetest.ReadControlMessage(t, link, 5*time.Second)
		if first.Type != remoteproto.ControlSnapshot || first.SecurityEpoch != 1 {
			t.Fatalf("first authenticated frame of the retiring replica = %s epoch %d", first.Type, first.SecurityEpoch)
		}
		remotetest.SendControl(
			t,
			link,
			remoteproto.ControlAck,
			remoteproto.SecurityAck{SecurityEpoch: first.SecurityEpoch},
		)
		lease := remotetest.ReadControlMessage(t, link, 5*time.Second)
		if lease.Type != remoteproto.ControlLease || lease.SecurityEpoch != 1 {
			t.Fatalf("retiring replica lease = %s epoch %d", lease.Type, lease.SecurityEpoch)
		}
		started := time.Now()
		if err := process.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		verifyGrantDuringDrain(t, ctx, peer, device)
		for {
			if message := remotetest.ReadControlMessage(
				t,
				link,
				5*time.Second,
			); message.Type == remoteproto.ControlGoingAway {
				break
			}
		}
		reconnected := remotetest.AuthenticateControlLink(t, ctx, peer.Server.URL, "cmp_drain", computerKey)
		if got := remotetest.ReadControlMessage(
			t,
			reconnected,
			5*time.Second,
		); got.Type != remoteproto.ControlSnapshot ||
			got.SecurityEpoch != first.SecurityEpoch {
			t.Fatalf("first authenticated frame after reconnect = %s epoch %d", got.Type, got.SecurityEpoch)
		}
		if err := process.Wait(); err != nil || time.Since(started) > cloud.DrainTimeout {
			t.Fatalf("retiring replica exited after %v with %v", time.Since(started), err)
		}
		var dialer net.Dialer
		conn, err := dialer.DialContext(ctx, "tcp", strings.TrimPrefix(firstURL, "http://"))
		if err == nil {
			conn.Close()
			t.Fatal("retired replica still accepts connections")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, peer.Server.URL+"/healthz", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := peer.Server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != http.StatusOK || string(body) != `{"status":"ok"}` {
			t.Fatalf("peer health = %d, %s, %v", resp.StatusCode, body, err)
		}
	})
}

func verifyGrantDuringDrain(
	t *testing.T, ctx context.Context, peer *remotetest.Replica, device remotetest.ControlDevice,
) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, peer.Server.URL+"/v1/computers/cmp_drain/grants",
		strings.NewReader(`{"route":"direct"}`))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	device.AuthorizeRequest(t, req, now)
	response, err := peer.Server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var grant control.Grant
	if err := json.UnmarshalRead(response.Body, &grant); err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("grant through the ready replica status=%d, %v", response.StatusCode, err)
	}
	verifier, err := remoteproto.NewJWSVerifier(remoteproto.CloudIssuer, peer.App.Keys.JWKS(), nil)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := verifier.VerifyGrant(ctx, grant.Grant, now)
	if err != nil || claims.Audience != "cmp_drain" || claims.Cnf.JKT != device.JKT {
		t.Fatalf("grant through the ready replica = %+v, %v", claims, err)
	}
}

func migrateCommand(t *testing.T, ctx context.Context, direct string) *exec.Cmd {
	t.Helper()
	cmd := testCloudCommand(t, ctx, "command", "migrate", "up")
	cmd.Env = append(cmd.Env, envDirectDSN+"="+direct)
	return cmd
}

func testCloudCommand(t *testing.T, ctx context.Context, mode string, args ...string) *exec.Cmd {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), cloudProcessMode+"="+mode, envEnvironment+"="+development)
	return cmd
}

func parseMigrationReport(t *testing.T, output string) (int64, int) {
	t.Helper()
	var version int64
	var applied int
	if _, err := fmt.Sscanf(output, "Cloud schema version %d (%d migrations applied)", &version, &applied); err != nil {
		t.Fatalf("parse migration report %q: %v", output, err)
	}
	return version, applied
}

func startDrainFixture(t *testing.T, ctx context.Context, db *remotetest.Database) (string, *exec.Cmd) {
	t.Helper()
	socket, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	file, err := socket.File()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	cmd := testCloudCommand(t, ctx, "drain")
	dsn := db.Config(t, "drain")
	var seed [ed25519.SeedSize]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(cmd.Env, envPooledDSN+"="+dsn.PooledDSN, envDirectDSN+"="+dsn.DirectDSN,
		envSigningKID+"=test-drain", envSigningSeed+"="+base64.RawURLEncoding.EncodeToString(seed[:]))
	cmd.ExtraFiles = []*os.File{file}
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "ready\n" {
		t.Fatalf("cloud subprocess readiness = %q, %v", line, err)
	}
	return "http://" + socket.Addr().String(), cmd
}

func serveDrainFixture() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	settings, err := loadServeConfig(os.Getenv)
	if err != nil {
		return err
	}
	app, err := cloud.Open(ctx, settings.Cloud)
	if err != nil {
		return err
	}
	defer app.Close()
	inherited := os.NewFile(3, "cloud-test-listener")
	listener, err := net.FileListener(inherited)
	closeErr := inherited.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		listener.Close()
		return closeErr
	}
	go func() {
		wait, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		if err := app.Control.WaitReady(wait); err == nil {
			fmt.Fprintln(os.Stdout, "ready")
		} else {
			stop()
		}
	}()
	return app.Serve(ctx, listener)
}

func TestCloudRestore(t *testing.T) {
	t.Parallel()
	t.Run("Should publish coverage only after all parts of a large security archive are durable", func(t *testing.T) {
		// AST-005: A bounded R2 upload retries the full fenced backlog without advancing on partial failure.
		t.Parallel()
		source := remotetest.NewDatabase(t)
		source.Migrate(t)
		var failPart atomic.Bool
		env := restoreFixtureEnvironment(t, func(key string) bool {
			return failPart.Load() && strings.HasSuffix(key, "/1.json")
		})
		env[envNeonDirect] = source.Config(t, "archive-backlog").DirectDSN
		point, err := parseRestoreTime(strings.TrimSpace(runRestoreCommand(t, env, "backup", "security-events")))
		if err != nil {
			t.Fatal(err)
		}
		const eventCount, detailSize = 6, 6 << 20
		source.Exec(t, `INSERT INTO %s.security_events (type, actor, at, details_json)
			SELECT 'archive_fixture', 'system', clock_timestamp(), jsonb_build_object('detail', repeat('x', $1))
			FROM generate_series(1, $2)`, detailSize, eventCount)
		failPart.Store(true)
		var output bytes.Buffer
		if err := shipEvents(t.Context(), func(key string) string { return env[key] }, &output); err == nil ||
			!strings.Contains(err.Error(), "HTTP 503") || output.Len() != 0 {
			t.Fatalf("partial shipment = %q, %v", &output, err)
		}
		bucket, err := archiveBucket(func(key string) string { return env[key] }, development)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := bucket.RestoreEvents(t.Context(), point, point.Add(time.Microsecond)); err == nil {
			t.Fatal("partial shipment advanced the restore cutoff")
		}
		failPart.Store(false)
		through, err := parseRestoreTime(strings.TrimSpace(runRestoreCommand(t, env, "backup", "security-events")))
		if err != nil {
			t.Fatal(err)
		}
		last, events, err := bucket.RestoreEvents(t.Context(), point, through)
		if err != nil || len(events) != eventCount || last != events[eventCount-1].ID {
			t.Fatalf("restored %d events at cursor %d: %v", len(events), last, err)
		}
		for _, event := range events {
			if detail, ok := event.Details["detail"].(string); !ok || len(detail) != detailSize {
				t.Fatalf("restore lost part of archived event %d", event.ID)
			}
		}
	})

	t.Run("Should finalize a copied database without reviving revoked access (IT-107)", func(t *testing.T) {
		// Safety Invariant 32. The retry must keep the 1000-epoch bump and reconcile both external providers.
		t.Parallel()
		source, restored := remotetest.NewDatabase(t), remotetest.NewDatabase(t)
		source.Migrate(t)
		restored.Migrate(t)
		now := time.Now
		at := now().UTC()
		cfg := remotetest.ControlConfig(t, now, nil)
		cfg.Postgres = source.Config(t, "restore-source")
		const account, other = "acc_restore", "acc_restore_other"
		const computer remoteproto.ComputerID = "cmp_restore"
		source.SeedAccount(t, account)
		source.SeedAccount(t, other)
		owner := source.SeedControlDevice(t, account, "dev_restore_keep", true, cfg.Keys.Signer(), at)
		revoke := source.SeedControlDevice(t, account, "dev_restore_revoke", false, cfg.Keys.Signer(), at)
		computerKey := source.SeedControlComputer(t, account, computer, at)
		const hostname = "restore-abuse.compozy.run"
		source.Exec(t, `INSERT INTO %s.tunnels (computer_id, hostname, state, created_at)
			VALUES ($1, $2, 'deleted', $3)`, computer, hostname, at)
		source.Exec(t, `INSERT INTO %s.identities (provider, subject, account_id, email)
			VALUES ('github', 'restore-subject', $1, 'restore@example.test')`, account)
		source.Exec(t, "UPDATE %s.accounts SET security_epoch = 6 WHERE id = $1", account)
		source.Exec(t, `INSERT INTO %s.cloud_web_sessions (id_sha256, account_id, expires_at) VALUES ($1, $2, $3)`,
			[]byte("fixture-web-session"), account, at.Add(time.Hour))
		source.Exec(t, `INSERT INTO %s.subscriptions (id, account_id, source, status, price_id, interval)
            VALUES ('sub_restore', $1, 'stripe', 'active', $2, 'month')`, account, remotetest.StripeMonthly)

		env := restoreFixtureEnvironment(t, nil)
		env[envNeonDirect] = cfg.Postgres.DirectDSN
		runRestoreCommand(t, env, "backup", "security-events")
		copyCloudRestorePoint(t, source, restored)
		point := now().UTC().Truncate(time.Microsecond)
		replica := remotetest.StartReplica(t, cfg, nil)
		if err := replica.App.Accounts.RevokeDevice(t.Context(), control.DevicePrincipal{
			AccountID: account, DeviceID: owner.ID, JKT: owner.JKT, SessionID: "ds_" + string(owner.ID),
		}, revoke.ID); err != nil {
			t.Fatal(err)
		}
		// The archived takedown is for a later generation of the hostname in the backup.
		if err := replica.App.Store.AppendSecurityEvent(t.Context(), store.SecurityEvent{
			AccountID: account, Actor: "admin:operator@example.test", Type: "hosted_tunnel_takedown", At: now(),
			Details: map[string]any{"computer_id": computer, "hostname": hostname, "generation": 2},
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := replica.App.Accounts.AdminDetachIdentity(t.Context(), account, "operator@example.test",
			"github", "restore-subject", "Please detach this GitHub identity."); err != nil {
			t.Fatal(err)
		}
		env[envRestoreThrough] = strings.TrimSpace(runRestoreCommand(t, env, "backup", "security-events"))
		env[envNeonDirect] = restored.Config(t, "restore-finalize").DirectDSN
		stripe := remotetest.StartStripe(t, now, false)
		env["COMPOZY_CLOUD_STRIPE_API_ORIGIN"] = stripe.Origin
		stripe.Put(remotetest.StripeSubscription{
			ID: "sub_restore", Account: account, Status: "canceled",
			Price: remotetest.StripeMonthly, Interval: "month", PeriodEnd: at,
		})
		events := []map[string]any{{
			"id": "evt_restore_cancel", "type": "customer.subscription.deleted",
			"data": map[string]any{"object": map[string]string{"id": "sub_restore"}},
		}}
		stripe.SetReplayEvents(events, true)
		cloudflare, orphanDeleted := startRestoreCloudflare(t)
		env["COMPOZY_CLOUD_CLOUDFLARE_API_ORIGIN"] = cloudflare
		cfg.Postgres = restored.Config(t, "restored")
		cfg.ExpectedRestore = point
		assertCloudRestoreRefused(t, cfg)

		args := []string{"restore", "finalize", "--restore-point", point.Format(time.RFC3339Nano)}
		var out bytes.Buffer
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		if err := run(t.Context(), args, func(key string) string { return env[key] }, &out, logger); err == nil ||
			!strings.Contains(err.Error(), "replay Stripe events") {
			t.Fatalf("expected a resumable Stripe failure, got %v", err)
		}
		assertCloudRestoreRefused(t, cfg)
		stripe.SetReplayEvents(events, false)
		runRestoreCommand(t, env, args...)
		runRestoreCommand(t, env, args...)

		assertRestoreCount(t, restored, `SELECT count(*) FROM %s.accounts
            WHERE (id = 'acc_restore' AND security_epoch = 1006) OR (id = 'acc_restore_other' AND security_epoch = 1001)`, 2)
		assertRestoreCount(t, restored, "SELECT count(*) FROM %s.device_sessions WHERE revoked_at IS NOT NULL", 2)
		assertRestoreCount(t, restored, "SELECT count(*) FROM %s.cloud_web_sessions WHERE revoked_at IS NOT NULL", 1)
		assertRestoreCount(t, restored, "SELECT count(*) FROM %s.devices", 2)
		assertRestoreCount(t, restored, "SELECT count(*) FROM %s.device_keys", 2)
		assertRestoreCount(t, restored, "SELECT count(*) FROM %s.identities WHERE subject = 'restore-subject'", 0)
		assertRestoreCount(t, restored, `SELECT count(*) FROM %s.tunnels
			WHERE hostname = 'restore-abuse.compozy.run' AND generation = 1
			AND state = 'deleted' AND admin_takedown_at IS NOT NULL`, 1)
		assertRestoreCount(
			t,
			restored,
			"SELECT count(*) FROM %s.devices WHERE id = 'dev_restore_revoke' AND revoked_at IS NOT NULL",
			1,
		)
		assertRestoreCount(
			t,
			restored,
			"SELECT count(*) FROM %s.subscriptions WHERE id = 'sub_restore' AND status = 'canceled'",
			1,
		)
		select {
		case <-orphanDeleted:
		default:
			t.Fatal("restore left an orphaned hosted tunnel in Cloudflare")
		}
		reconnected := remotetest.StartReplica(t, cfg, nil)
		link := remotetest.AuthenticateControl(t, reconnected, computer, computerKey)
		frame := remotetest.ReadControlMessage(t, link, 5*time.Second)
		verifier, err := remoteproto.NewJWSVerifier(reconnected.App.Issuer, cfg.Keys.JWKS(), nil)
		if err != nil {
			t.Fatal(err)
		}
		var signed string
		if err := json.Unmarshal(frame.Data, &signed); err != nil {
			t.Fatal(err)
		}
		snapshot, err := verifier.VerifySnapshot(t.Context(), signed)
		if err != nil || frame.Type != remoteproto.ControlSnapshot || snapshot.SecurityEpoch != 1006 ||
			len(snapshot.Devices) != 1 || snapshot.Devices[0].DeviceID != owner.ID {
			t.Fatalf("unexpected snapshot after restore: %+v: %v", snapshot, err)
		}
		assertRestoredMint(t, cfg.Keys.Signer(), verifier, snapshot, computer, owner, 6)
	})
}

func assertCloudRestoreRefused(t *testing.T, cfg cloud.Config) {
	t.Helper()
	app, err := cloud.Open(t.Context(), cfg)
	if err == nil {
		app.Close()
		t.Fatal("cloud served a database awaiting restore finalization")
	}
	if !strings.Contains(err.Error(), "restore") {
		t.Fatalf("cloud refused startup for another reason: %v", err)
	}
}

func assertRestoreCount(t *testing.T, db *remotetest.Database, query string, expected int) {
	t.Helper()
	var got int
	if err := db.Pool.QueryRow(t.Context(), fmt.Sprintf(query, db.Schema)).Scan(&got); err != nil || got != expected {
		t.Fatalf("%s returned %d, expected %d: %v", query, got, expected, err)
	}
}

func copyCloudRestorePoint(t *testing.T, source, target *remotetest.Database) {
	t.Helper()
	for _, relation := range []string{"accounts", "devices", "device_keys", "device_sessions",
		"cloud_web_sessions", "computers", "subscriptions", "identities", "tunnels"} {
		statement := fmt.Sprintf(
			"INSERT INTO %s.%s SELECT * FROM %s.%s",
			target.Schema,
			relation,
			source.Schema,
			relation,
		)
		if _, err := source.Pool.Exec(t.Context(), statement); err != nil {
			t.Fatalf("copy %s at the restore point: %v", relation, err)
		}
	}
}

func assertRestoredMint(
	t *testing.T,
	signer *remoteproto.Signer,
	verifier *remoteproto.JWSVerifier,
	snapshot remoteproto.SecuritySnapshot,
	computer remoteproto.ComputerID,
	device remotetest.ControlDevice,
	oldEpoch uint64,
) {
	t.Helper()
	db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("close restored host fixture: %v", err)
		}
	})
	at := time.Now()
	host, err := remotehost.NewSessions(t.Context(), db, remotehost.Identity{
		ComputerID: computer, AccountID: snapshot.AccountID,
	}, verifier, remotehost.SessionOptions{Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.ApplySnapshot(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	err = host.ApplyLease(t.Context(), remoteproto.EntitlementLease{
		ComputerID:    computer,
		AccountID:     snapshot.AccountID,
		Plan:          "free",
		SecurityEpoch: snapshot.SecurityEpoch,
		Sequence:      1,
		ValidUntil:    at.Add(remoteproto.LeaseTTL).Unix(),
		AllowedRoutes: []remoteproto.RouteKind{remoteproto.RouteDirect},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, epoch := range []uint64{oldEpoch, snapshot.SecurityEpoch} {
		grant, err := signer.SignGrant(remoteproto.GrantClaims{
			Issuer: remoteproto.CloudIssuer, Audience: computer, Subject: device.ID, AccountID: snapshot.AccountID,
			Cnf: remoteproto.Confirmation{JKT: device.JKT}, Route: remoteproto.RouteDirect,
			Scopes: []remoteproto.Scope{remoteproto.ScopeSessions}, JTI: fmt.Sprintf("grant-restore-%d", epoch),
			IssuedAt: at.Unix(), ExpiresAt: at.Add(remoteproto.GrantTTL).Unix(), SecurityEpoch: epoch,
		})
		if err != nil {
			t.Fatal(err)
		}
		const endpoint = "https://host.test/remote/v1/mint"
		proof, err := remoteproto.BuildDPoP(device.Key, http.MethodPost, endpoint, "", at)
		if err != nil {
			t.Fatal(err)
		}
		result, err := host.Mint(t.Context(), remotehost.MintInput{Grant: grant, Proof: remotehost.DPoPProof{
			Token: proof, Method: http.MethodPost, URL: endpoint, Listener: remotehost.ListenerDirect,
		}})
		if epoch == oldEpoch {
			if !errors.Is(err, remotehost.ErrSnapshotPending) {
				t.Fatalf("pre-restore grant was not fenced: %v", err)
			}
		} else if err != nil || result.Credential == "" {
			t.Fatalf("current grant could not mint: %v", err)
		}
	}
}

func runRestoreCommand(t *testing.T, env map[string]string, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	err := run(t.Context(), args, func(key string) string { return env[key] }, &out, slog.Default())
	if err != nil {
		t.Fatalf("cloud %v: %v", args, err)
	}
	return out.String()
}

func restoreFixtureEnvironment(t *testing.T, rejectPut func(string) bool) map[string]string {
	t.Helper()
	var mu sync.Mutex
	objects := make(map[string][]byte)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/restore-fixture" && !strings.HasPrefix(r.URL.Path, "/restore-fixture/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/restore-fixture/")
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			type item struct{ Key string }
			var items []item
			for name := range objects {
				items = append(items, item{name})
			}
			if err := xml.NewEncoder(w).Encode(struct {
				XMLName  xml.Name `xml:"ListBucketResult"`
				Contents []item
			}{Contents: items}); err != nil {
				t.Error(err)
			}
		case r.Method == http.MethodPut && r.Header.Get("If-None-Match") == "*":
			if rejectPut != nil && rejectPut(key) {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if _, exists := objects[key]; exists {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			objects[key] = body
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && objects[key] != nil:
			if _, err := w.Write(objects[key]); err != nil {
				t.Error(err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return map[string]string{
		envEnvironment:                              development,
		"COMPOZY_CLOUD_R2_ENDPOINT":                 server.URL,
		"COMPOZY_CLOUD_R2_BACKUP_BUCKET":            "restore-fixture",
		"COMPOZY_CLOUD_R2_ACCESS_KEY_ID":            "fixture-access",
		"COMPOZY_CLOUD_R2_SECRET_ACCESS_KEY":        "fixture-secret",
		"COMPOZY_CLOUD_CLOUDFLARE_ACCOUNT_ID":       "fixture-account",
		"COMPOZY_CLOUD_CLOUDFLARE_RUN_ZONE_ID":      "fixture-zone",
		"COMPOZY_CLOUD_CLOUDFLARE_TUNNEL_API_TOKEN": "fixture-cloudflare",
		"COMPOZY_CLOUD_STRIPE_SECRET_KEY":           "sk_test_fixture",
		"COMPOZY_CLOUD_STRIPE_WEBHOOK_SECRET":       "whsec_fixture",
		"STRIPE_PRICE_PRO_MONTHLY":                  remotetest.StripeMonthly,
		"STRIPE_PRICE_PRO_ANNUAL":                   remotetest.StripeAnnual,
	}
}

func startRestoreCloudflare(t *testing.T) (string, <-chan struct{}) {
	t.Helper()
	const id = "00000000-0000-4000-8000-000000000107"
	const collection = "/accounts/fixture-account/cfd_tunnel"
	tunnel := map[string]string{"id": id, "name": "compozy-prod-restore-orphan-1"}
	deleted := make(chan struct{})
	var closeOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		switch {
		case r.Method == http.MethodGet && r.URL.Path == collection:
			select {
			case <-deleted:
				result = []any{}
			default:
				result = []any{tunnel}
			}
		case r.Method == http.MethodGet && r.URL.Path == collection+"/"+id:
			result = tunnel
		case r.Method == http.MethodGet && r.URL.Path == "/zones/fixture-zone/dns_records":
			result = []any{}
		case r.Method == http.MethodDelete && r.URL.Path == collection+"/"+id+"/connections":
			result = tunnel
		case r.Method == http.MethodDelete && r.URL.Path == collection+"/"+id:
			closeOnce.Do(func() { close(deleted) })
			result = tunnel
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "result": result})
	}))
	t.Cleanup(server.Close)
	return server.URL, deleted
}
