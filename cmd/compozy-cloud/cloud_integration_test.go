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

	"github.com/gorilla/websocket"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	cloudProcessMode = "COMPOZY_TEST_CLOUD_PROCESS"
	linkPath         = "/v1/computers/control"
)

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
		ctx, cancel := context.WithTimeout(t.Context(), 65*time.Second)
		defer cancel()
		firstURL, process := startDrainFixture(t, ctx, db)
		peer := remotetest.StartReplica(t, cloud.Config{Postgres: db.Config(t, "peer")}, registerDrainLink)

		link := dialCloudLink(t, ctx, firstURL)
		if got := readControl(t, link); got.Type != remoteproto.ControlSnapshot || got.SecurityEpoch != 1 {
			t.Fatalf("first frame of the retiring replica = %+v", got)
		}
		started := time.Now()
		if err := process.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		if got := readControl(t, link); got.Type != remoteproto.ControlGoingAway {
			t.Fatalf("drain did not send going_away: %+v", got)
		}
		reconnected := dialCloudLink(t, ctx, peer.Server.URL)
		if got := readControl(t, reconnected); got.Type != remoteproto.ControlSnapshot || got.SecurityEpoch != 1 {
			t.Fatalf("first frame after reconnect = %+v", got)
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
	registerDrainLink(app)
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

func registerDrainLink(app *cloud.App) {
	app.Routes().HandleFunc("GET "+linkPath, func(w http.ResponseWriter, r *http.Request) {
		sub, ok := app.Control.Subscribe("acc_fixture", "cmp_fixture")
		if !ok {
			api.WriteError(w, http.StatusServiceUnavailable, api.CodeUnavailable, "Cloud is unavailable.")
			return
		}
		defer app.Control.Release(sub)
		upgrader := websocket.Upgrader{}
		link, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer link.Close()
		snapshot := remoteproto.SecuritySnapshot{AccountID: "acc_fixture", SecurityEpoch: 1, Linked: true}
		token, err := app.Keys.Signer().SignSnapshot(snapshot)
		if err != nil {
			return
		}
		data, err := json.Marshal(token)
		if err != nil {
			return
		}
		if err := link.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
			return
		}
		first := remoteproto.ControlMessage{Type: remoteproto.ControlSnapshot, SecurityEpoch: 1, Data: data}
		if err := link.WriteJSON(first); err != nil {
			return
		}
		select {
		case reason := <-sub.Closing:
			if reason == control.GoingAway {
				if err := link.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
					return
				}
				link.WriteJSON(remoteproto.ControlMessage{Type: remoteproto.ControlGoingAway})
			}
		case <-r.Context().Done():
		}
	})
}

func dialCloudLink(t *testing.T, ctx context.Context, base string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	link, resp, err := dialer.DialContext(ctx, "ws"+strings.TrimPrefix(base, "http")+linkPath, nil)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatalf("dial fixture control link: %v", err)
	}
	t.Cleanup(func() { link.Close() })
	if err := link.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return link
}

func readControl(t *testing.T, link *websocket.Conn) remoteproto.ControlMessage {
	t.Helper()
	_, body, err := link.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var msg remoteproto.ControlMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		t.Fatal(err)
	}
	return msg
}
