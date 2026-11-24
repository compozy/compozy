//go:build integration

package remotetest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/cloud/turnstile"
	"github.com/compozy/compozy/internal/remoteproto"
)

const postgresEnv = "COMPOZY_CLOUD_TEST_POSTGRES"

type Database struct {
	Schema string
	Pool   *pgxpool.Pool
	dsn    string
}

func NewDatabase(t *testing.T) *Database {
	t.Helper()
	dsn := os.Getenv(postgresEnv)
	if dsn == "" {
		if os.Getenv("CI") == "true" {
			t.Fatalf("%s must be set for cloud integration tests", postgresEnv)
		}
		t.Skipf("set %s to run the Postgres-backed cloud suite", postgresEnv)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("could not configure cloud test Postgres")
	}
	t.Cleanup(pool.Close)
	schema := "remote_" + strings.ToLower(rand.Text())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatalf("create cloud test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("drop cloud test schema: %v", err)
		}
	})
	return &Database{Schema: schema, Pool: pool, dsn: dsn}
}

func (db *Database) Config(t *testing.T, replica string) store.Config {
	t.Helper()
	return store.Config{
		PooledDSN: db.connectionURL(t, db.ApplicationName(replica, "pooled")),
		DirectDSN: db.connectionURL(t, db.ApplicationName(replica, "direct")),
	}
}

func (db *Database) Migrate(t *testing.T) store.MigrationReport {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	report, err := store.MigrateUp(ctx, db.Config(t, "migrate").DirectDSN)
	if err != nil {
		t.Fatalf("migrate cloud test schema: %v", err)
	}
	return report
}

func (db *Database) ApplicationName(replica, kind string) string {
	return db.Schema + "_" + replica + "_" + kind
}

func (db *Database) connectionURL(t *testing.T, application string) string {
	t.Helper()
	parsed, err := url.Parse(db.dsn)
	if err != nil || parsed.Hostname() == "" {
		t.Fatalf("%s must be a Postgres URL", postgresEnv)
	}
	query := parsed.Query()
	query.Set("search_path", db.Schema)
	query.Set("application_name", application)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

type Replica struct {
	App    *cloud.App
	Server *httptest.Server
}

func StartReplica(t *testing.T, cfg cloud.Config, register func(*cloud.App)) *Replica {
	t.Helper()
	return startReplica(t, cfg, register, false, "")
}

// StartLocalReplica uses the actual test listener as the issuer and DPoP origin.
func StartLocalReplica(t *testing.T, cfg cloud.Config, register func(*cloud.App)) *Replica {
	t.Helper()
	return startReplica(t, cfg, register, true, "")
}

// StartLocalReplicaOn also makes the fixture reachable from a container on the test network.
func StartLocalReplicaOn(t *testing.T, host string, cfg cloud.Config, register func(*cloud.App)) *Replica {
	t.Helper()
	return startReplica(t, cfg, register, true, host)
}

func startReplica(t *testing.T, cfg cloud.Config, register func(*cloud.App), localOrigin bool, host string) *Replica {
	t.Helper()
	if cfg.Keys == nil {
		var err error
		cfg.Keys, err = api.NewSigningKeys(NewSigner(t, "test-cloud"), nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	server := fixtureHTTPServer(t, host)
	t.Cleanup(server.Close)
	if localOrigin {
		cfg.Issuer = "http://" + server.Listener.Addr().String()
	}
	app, err := cloud.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	if register != nil {
		register(app)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.RunBackground(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	server.Config.Handler = app
	server.Start()
	t.Cleanup(func() {
		app.StartDrain()
		server.Close()
	})
	ready, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	if err := app.Control.WaitReady(ready); err != nil {
		t.Fatalf("wait for cloud test replica: %v", err)
	}
	return &Replica{App: app, Server: server}
}

func NewSigner(t *testing.T, kid string) *remoteproto.Signer {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := remoteproto.NewSigner(kid, key)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func ControlConfig(t *testing.T, now func() time.Time, keys *api.SigningKeys) cloud.Config {
	t.Helper()
	if keys == nil {
		var err error
		keys, err = api.NewSigningKeys(NewSigner(t, "test-control"), nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Control tests seed sessions and do not contact identity providers.
	client := auth.OAuthClient{ID: "test-client", Secret: auth.Secret()}
	oauth, err := auth.NewOAuth(client, client, auth.WithOAuthEndpoints(auth.OAuthEndpoints{}))
	if err != nil {
		t.Fatal(err)
	}
	botCheck, err := turnstile.New(
		auth.Secret(),
		"cloud.compozy.com",
		turnstile.WithEndpoint("https://turnstile.invalid"),
	)
	if err != nil {
		t.Fatal(err)
	}
	var sealKey [32]byte
	rand.Read(sealKey[:])
	return cloud.Config{
		Keys: keys, Now: now, Auth: &auth.Config{
			OAuth: oauth, EncryptionKey: sealKey[:], Turnstile: botCheck,
			TurnstileSiteKey: "test-site-key", HostedDomain: "compozy.run",
		},
	}
}
