// Suite: Slack app manifest contract
// Invariant: generated Slack manifests match actual AGH Slack capabilities and Slack's pinned schema.
// Boundary IN: deterministic bridge-instance and provider metadata to manifest generation.
// Boundary OUT: daemon routes and CLI file output, owned by their transport suites.
package bridges

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type slackSchemaRegexp struct {
	source                 string
	compiled               *regexp.Regexp
	rejectLeadingDotOrDash bool
}

func (re *slackSchemaRegexp) MatchString(value string) bool {
	if re.rejectLeadingDotOrDash && (strings.HasPrefix(value, ".") || strings.HasPrefix(value, "-")) {
		return false
	}
	return re.compiled.MatchString(value)
}

func (re *slackSchemaRegexp) String() string {
	return re.source
}

func TestSlackAppManifestContract(t *testing.T) {
	t.Run("Should validate against the vendored Slack app manifest schema", func(t *testing.T) {
		t.Parallel()

		manifest := mustBuildSlackAppManifest(t)
		payload, err := json.Marshal(manifest)
		if err != nil {
			t.Fatalf("json.Marshal(manifest) error = %v", err)
		}
		validateSlackManifestSchema(t, payload)
	})

	t.Run("Should keep event subscriptions and required scopes in one relation", func(t *testing.T) {
		t.Parallel()

		manifest := mustBuildSlackAppManifest(t)
		if !slices.Contains(manifest.OAuthConfig.Scopes.Bot, "chat:write") {
			t.Fatalf("bot scopes = %v, want chat:write", manifest.OAuthConfig.Scopes.Bot)
		}
		if !slices.Contains(manifest.OAuthConfig.Scopes.Bot, "reactions:write") {
			t.Fatalf("bot scopes = %v, want reactions:write", manifest.OAuthConfig.Scopes.Bot)
		}

		for _, event := range manifest.Settings.EventSubscriptions.BotEvents {
			required := SlackRequiredScopesForEvent(event)
			if len(required) == 0 {
				t.Fatalf("SlackRequiredScopesForEvent(%q) = nil, want at least one read scope", event)
			}
			for _, scope := range required {
				if !slices.Contains(manifest.OAuthConfig.Scopes.Bot, scope) {
					t.Fatalf(
						"event %q requires missing scope %q; scopes=%v",
						event,
						scope,
						manifest.OAuthConfig.Scopes.Bot,
					)
				}
			}
		}

		for _, event := range []string{"reaction_added", "reaction_removed"} {
			if !slices.Contains(manifest.Settings.EventSubscriptions.BotEvents, event) {
				t.Fatalf("bot events = %v, want %q", manifest.Settings.EventSubscriptions.BotEvents, event)
			}
			required := SlackRequiredScopesForEvent(event)
			if !slices.Contains(required, "reactions:read") {
				t.Fatalf(
					"SlackRequiredScopesForEvent(%q) = %v, want reactions:read",
					event,
					required,
				)
			}
		}
	})

	t.Run("Should template every callback URL and keep Socket Mode disabled", func(t *testing.T) {
		t.Parallel()

		manifest := mustBuildSlackAppManifest(t)
		const publicURL = "https://bridge.example.com/slack/support"
		if got := manifest.Settings.EventSubscriptions.RequestURL; got != publicURL {
			t.Fatalf("event request URL = %q, want %q", got, publicURL)
		}
		if got := manifest.Settings.Interactivity.RequestURL; got != publicURL {
			t.Fatalf("interactivity request URL = %q, want %q", got, publicURL)
		}
		if len(manifest.Features.SlashCommands) != 1 || manifest.Features.SlashCommands[0].URL != publicURL {
			t.Fatalf("slash commands = %#v, want one command using %q", manifest.Features.SlashCommands, publicURL)
		}
		if manifest.Settings.SocketModeEnabled {
			t.Fatal("socket_mode_enabled = true, want false for webhook ingress")
		}
	})

	t.Run("Should reject a non-Slack instance and missing public webhook URL", func(t *testing.T) {
		t.Parallel()

		provider := testSlackManifestProvider()
		instance := testSlackManifestInstance()
		instance.Platform = "telegram"
		if _, err := BuildSlackAppManifest(instance, provider); err == nil {
			t.Fatal("BuildSlackAppManifest(non-Slack) error = nil, want validation error")
		}

		instance = testSlackManifestInstance()
		instance.ProviderConfig = json.RawMessage(`{"webhook":{"path":"/slack/support"}}`)
		if _, err := BuildSlackAppManifest(instance, provider); err == nil {
			t.Fatal("BuildSlackAppManifest(missing public_url) error = nil, want validation error")
		}
	})

	t.Run("Should return defensive event and relation copies", func(t *testing.T) {
		t.Parallel()

		events := SlackBotEvents()
		required := SlackRequiredScopesForEvent("message.mpim")
		events[0] = "mutated"
		required[0] = "mutated"

		if slices.Contains(SlackBotEvents(), "mutated") {
			t.Fatal("SlackBotEvents() retained caller mutation")
		}
		if slices.Contains(SlackRequiredScopesForEvent("message.mpim"), "mutated") {
			t.Fatal("SlackRequiredScopesForEvent() retained caller mutation")
		}
		if got := SlackRequiredScopesForEvent("unknown"); got != nil {
			t.Fatalf("SlackRequiredScopesForEvent(unknown) = %v, want nil", got)
		}
	})
}

func mustBuildSlackAppManifest(t *testing.T) SlackAppManifest {
	t.Helper()

	manifest, err := BuildSlackAppManifest(testSlackManifestInstance(), testSlackManifestProvider())
	if err != nil {
		t.Fatalf("BuildSlackAppManifest() error = %v", err)
	}
	return manifest
}

func testSlackManifestInstance() BridgeInstance {
	return BridgeInstance{
		ID:            "brg-slack",
		Scope:         ScopeGlobal,
		Platform:      "slack",
		ExtensionName: "slack-reference",
		DisplayName:   "Support agent",
		ProviderConfig: json.RawMessage(
			`{"webhook":{"public_url":"https://bridge.example.com/slack/support"}}`,
		),
	}
}

func testSlackManifestProvider() BridgeProvider {
	return BridgeProvider{
		Platform:      "slack",
		ExtensionName: "slack-reference",
		DisplayName:   "Slack",
		Description:   "Connect Slack messages, commands, actions, and reactions to AGH.",
	}
}

func validateSlackManifestSchema(t *testing.T, payload []byte) {
	t.Helper()

	schemaPath := filepath.Join(
		"testdata",
		"slack-manifest-schema-v0.5.0",
		"manifest.schema.1.0.0.json",
	)
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", schemaPath, err)
	}
	const pinnedSchemaSHA256 = "f2adf9f9800230e2fe44bab0762796492871edaab817e9160fdcdf0eb4642e58"
	if got := fmt.Sprintf("%x", sha256.Sum256(schemaData)); got != pinnedSchemaSHA256 {
		t.Fatalf("Slack schema SHA-256 = %s, want pinned %s", got, pinnedSchemaSHA256)
	}
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
	if err != nil {
		t.Fatalf("jsonschema.UnmarshalJSON(schema) error = %v", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("jsonschema.UnmarshalJSON(manifest) error = %v", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseRegexpEngine(compileSlackSchemaRegexp)
	if err := compiler.AddResource("slack-manifest.json", schemaValue); err != nil {
		t.Fatalf("compiler.AddResource() error = %v", err)
	}
	compiled, err := compiler.Compile("slack-manifest.json")
	if err != nil {
		t.Fatalf("compiler.Compile() error = %v", err)
	}
	if err := compiled.Validate(instance); err != nil {
		t.Fatalf("Slack manifest schema validation error = %v\nmanifest=%s", err, payload)
	}
}

func compileSlackSchemaRegexp(pattern string) (jsonschema.Regexp, error) {
	// Slack's schema uses one ECMAScript negative lookahead that Go's RE2 does not implement.
	const unsupportedPrefix = `^(?![\.\-])`
	goPattern := pattern
	rejectLeadingDotOrDash := strings.HasPrefix(pattern, unsupportedPrefix)
	if rejectLeadingDotOrDash {
		goPattern = "^" + strings.TrimPrefix(pattern, unsupportedPrefix)
	}
	compiled, err := regexp.Compile(goPattern)
	if err != nil {
		return nil, err
	}
	return &slackSchemaRegexp{
		source:                 pattern,
		compiled:               compiled,
		rejectLeadingDotOrDash: rejectLeadingDotOrDash,
	}, nil
}
