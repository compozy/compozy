package remotetest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"time"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/compozy/compozy/internal/update"
)

const (
	SSHFixtureVersion = "0.8.0"
	testIssuer        = "https://issuer.example.test"
	testIdentity      = "release@example.test"
)

type Release struct {
	Server       *httptest.Server
	Verifier     update.BundleVerifier
	TrustRoot    []byte
	BadSigner    atomic.Bool
	BadArchive   atomic.Bool
	Downloads    atomic.Int64
	ManifestGate atomic.Pointer[ManifestGate]
}

type ManifestGate struct {
	Requested chan struct{}
	Resume    <-chan struct{}
}

func StartRelease(t Fixture, platform, version string, archive []byte) *Release {
	t.Helper()
	return StartReleaseOn(t, "", platform, version, archive)
}

func StartReleaseOn(t Fixture, host, platform, version string, archive []byte) *Release {
	t.Helper()
	authority, err := ca.NewVirtualSigstore()
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), platform))
	valid := signManifest(t, authority, manifest, testIdentity)
	invalid := signManifest(t, authority, manifest, "untrusted@example.test")
	logs := authority.RekorLogs()
	for id, log := range logs {
		log.ID, err = hex.DecodeString(id)
		if err != nil {
			t.Fatal(err)
		}
	}
	trusted, err := root.NewTrustedRoot(root.TrustedRootMediaType01, authority.FulcioCertificateAuthorities(),
		nil, nil, logs)
	if err != nil {
		t.Fatal(err)
	}
	trustRoot, err := trusted.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := ReleaseVerifier(trustRoot)
	if err != nil {
		t.Fatal(err)
	}
	r := &Release{Verifier: verifier, TrustRoot: trustRoot, Server: fixtureHTTPServer(t, host)}
	r.Server.Config.Handler = r.handler(t, "/v"+version+"/", platform, manifest, valid, invalid, archive)
	t.Cleanup(r.Server.Close)
	r.Server.Start()
	return r
}

func (r *Release) handler(
	t Fixture, prefix, platform string, manifest, valid, invalid, archive []byte,
) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		path, ok := strings.CutPrefix(req.URL.Path, prefix)
		if !ok {
			http.NotFound(w, req)
			return
		}
		var data []byte
		switch path {
		case "checksums.txt":
			if gate := r.ManifestGate.Swap(nil); gate != nil {
				close(gate.Requested)
				select {
				case <-gate.Resume:
				case <-req.Context().Done():
					return
				}
			}
			data = manifest
		case "checksums.txt.sigstore.json":
			data = valid
			if r.BadSigner.Load() {
				data = invalid
			}
		case platform:
			r.Downloads.Add(1)
			data = archive
			if r.BadArchive.Load() {
				data = []byte("tampered release archive")
			}
		default:
			http.NotFound(w, req)
			return
		}
		if _, err := w.Write(data); err != nil && req.Context().Err() == nil {
			t.Logf("send fixture release %s: %v", path, err)
		}
	}
}

func fixtureHTTPServer(t Fixture, host string) *httptest.Server {
	t.Helper()
	if host == "" {
		return httptest.NewUnstartedServer(nil)
	}
	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp4", net.JoinHostPort(host, "0"))
	if err != nil {
		t.Fatal(err)
	}
	return &httptest.Server{Listener: listener, Config: &http.Server{ReadHeaderTimeout: time.Second}}
}

func ReleaseVerifier(trustRoot []byte) (update.BundleVerifier, error) {
	trusted, err := root.NewTrustedRootFromJSON(trustRoot)
	if err != nil {
		return nil, err
	}
	identity, err := verify.NewShortCertificateIdentity(testIssuer, "", testIdentity, "")
	if err != nil {
		return nil, err
	}
	// The virtual CA supplies Fulcio and Rekor. Production additionally requires an SCT.
	verifier, err := verify.NewVerifier(trusted, verify.WithObserverTimestamps(1), verify.WithTransparencyLog(1))
	if err != nil {
		return nil, err
	}
	return &releaseVerifier{verifier, identity}, nil
}

type releaseVerifier struct {
	verifier *verify.Verifier
	identity verify.CertificateIdentity
}

var _ update.BundleVerifier = (*releaseVerifier)(nil)

func (v *releaseVerifier) VerifyChecksums(ctx context.Context, sum, proof string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// #nosec G304 -- only the fixture's downloaded manifest is read here.
	manifest, err := os.ReadFile(sum)
	if err != nil {
		return err
	}
	signed, err := bundle.LoadJSONFromPath(proof)
	if err != nil {
		return err
	}
	_, err = v.verifier.Verify(signed, verify.NewPolicy(
		verify.WithArtifact(bytes.NewReader(manifest)), verify.WithCertificateIdentity(v.identity)))
	return err
}

func signManifest(t Fixture, authority *ca.VirtualSigstore, manifest []byte, identity string) []byte {
	t.Helper()
	signed, err := authority.SignWithVersion(identity, testIssuer, manifest, "v0.1")
	if err != nil {
		t.Fatal(err)
	}
	content, err := signed.VerificationContent()
	if err != nil {
		t.Fatal(err)
	}
	signature, err := signed.SignatureContent()
	if err != nil {
		t.Fatal(err)
	}
	logs, err := signed.TlogEntries()
	if err != nil || len(logs) != 1 {
		t.Fatalf("release fixture needs one Rekor entry: %v", err)
	}
	entry := proto.CloneOf(logs[0].TransparencyLogEntry())
	entry.KindVersion = &protorekor.KindVersion{Kind: "hashedrekord", Version: "0.0.1"}
	// VirtualSigstore signs a promise but keeps that signature outside its protobuf.
	promise, err := authority.RekorSignPayload(tlog.RekorPayload{
		Body: logs[0].Body(), LogID: hex.EncodeToString(entry.LogId.KeyId),
		LogIndex: entry.LogIndex, IntegratedTime: entry.IntegratedTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	entry.InclusionPromise = &protorekor.InclusionPromise{SignedEntryTimestamp: promise}
	return marshalReleaseBundle(t, content, signature, entry)
}

func marshalReleaseBundle(
	t Fixture,
	content verify.VerificationContent,
	signature verify.SignatureContent,
	log *protorekor.TransparencyLogEntry,
) []byte {
	t.Helper()
	digest := signature.MessageSignatureContent().Digest()
	mediaType, err := bundle.MediaTypeString("0.1")
	if err != nil {
		t.Fatal(err)
	}
	wire := &protobundle.Bundle{
		MediaType: mediaType,
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_X509CertificateChain{
				X509CertificateChain: &protocommon.X509CertificateChain{
					Certificates: []*protocommon.X509Certificate{{RawBytes: content.Certificate().Raw}},
				},
			},
			TlogEntries: []*protorekor.TransparencyLogEntry{log},
		},
		Content: &protobundle.Bundle_MessageSignature{MessageSignature: &protocommon.MessageSignature{
			MessageDigest: &protocommon.HashOutput{Algorithm: protocommon.HashAlgorithm_SHA2_256, Digest: digest},
			Signature:     signature.Signature(),
		}},
	}
	data, err := protojson.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
