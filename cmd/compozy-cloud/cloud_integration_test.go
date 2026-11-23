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
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/remoteproto"
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
