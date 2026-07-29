package extensionpkg

import (
	"reflect"
	"strings"
	"testing"
)

func TestRegistryDevelopmentLinksPreservePublishedRows(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	t.Run("Should upsert and resolve a development overlay without mutating published state", func(t *testing.T) {
		t.Parallel()
		env := newRegistryTestEnv(t)
		dir, manifest, checksum := createRegistryTestExtension(t, "dev-overlay", registryManifestOptions{})
		if err := env.registry.Install(manifest, dir, checksum); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		before, err := env.registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("Get(before) error = %v", err)
		}

		first, err := env.registry.LinkDev(DevLinkRequest{
			Name:           manifest.Name,
			WorkspaceID:    "ws-a",
			OriginPath:     "/workspace/a/extension",
			GenerationHash: strings.Repeat("a", 64),
		})
		if err != nil {
			t.Fatalf("LinkDev(first) error = %v", err)
		}
		if first.BundleGeneration != strings.Repeat("a", 64) {
			t.Fatalf("LinkDev(first).BundleGeneration = %q", first.BundleGeneration)
		}

		second, err := env.registry.LinkDev(DevLinkRequest{
			Name:           manifest.Name,
			WorkspaceID:    "ws-a",
			OriginPath:     "/workspace/a/extension-v2",
			GenerationHash: strings.Repeat("b", 64),
		})
		if err != nil {
			t.Fatalf("LinkDev(second) error = %v", err)
		}
		if second.OriginPath != "/workspace/a/extension-v2" {
			t.Fatalf("LinkDev(second).OriginPath = %q", second.OriginPath)
		}

		links, err := env.registry.ListDevLinks()
		if err != nil {
			t.Fatalf("ListDevLinks() error = %v", err)
		}
		if len(links) != 1 {
			t.Fatalf("ListDevLinks() count = %d, want 1", len(links))
		}
		active, err := env.registry.ResolveActive(manifest.Name, "ws-a")
		if err != nil {
			t.Fatalf("ResolveActive(dev) error = %v", err)
		}
		if active.DevLink == nil || !active.OverridesPublished {
			t.Fatalf("ResolveActive(dev) = %#v, want overriding development link", active)
		}
		after, err := env.registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("Get(after) error = %v", err)
		}
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("published row changed after LinkDev: before=%#v after=%#v", before, after)
		}

		if err := env.registry.UnlinkDev(manifest.Name, "ws-a"); err != nil {
			t.Fatalf("UnlinkDev() error = %v", err)
		}
		active, err = env.registry.ResolveActive(manifest.Name, "ws-a")
		if err != nil {
			t.Fatalf("ResolveActive(published) error = %v", err)
		}
		if active.DevLink != nil || active.Published == nil || active.OverridesPublished {
			t.Fatalf("ResolveActive(published) = %#v", active)
		}
	})

	t.Run("Should never mint a development link from any install source", func(t *testing.T) {
		for _, source := range []ExtensionSource{SourceBundled, SourceUser, SourceWorkspace, SourceMarketplace} {
			t.Run("Should keep install source "+source.String()+" out of the overlay table", func(t *testing.T) {
				t.Parallel()
				env := newRegistryTestEnv(t)
				dir, manifest, checksum := createRegistryTestExtension(
					t,
					"install-no-link-"+source.String(),
					registryManifestOptions{},
				)
				if err := env.registry.Install(manifest, dir, checksum, WithInstallSource(source)); err != nil {
					t.Fatalf("Install(source=%s) error = %v", source, err)
				}
				links, err := env.registry.ListDevLinks()
				if err != nil {
					t.Fatalf("ListDevLinks() error = %v", err)
				}
				if len(links) != 0 {
					t.Fatalf("ListDevLinks() = %#v, want empty", links)
				}
			})
		}
	})
}
