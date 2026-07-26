package modelcatalog

import (
	"encoding/json"
	"strings"
	"testing"

	redactpkg "github.com/compozy/agh/internal/redact"
	"github.com/compozy/agh/internal/toolmeta"
)

func TestCanonicalRedactionConsumersStaySafe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		leaks []string
	}{
		{
			name: "Should redact provider token shapes identically",
			input: strings.Join([]string{
				"Bearer bearer-token-value",
				"sk-openai-secret-value",
				"xoxb-slack-secret-value",
				"xapp-slack-app-secret",
				"ghp_githubsecretvalue",
			}, " "),
			want: strings.Join([]string{
				"Bearer [REDACTED]",
				"[REDACTED]",
				"[REDACTED]",
				"[REDACTED]",
				"[REDACTED]",
			}, " "),
			leaks: []string{
				"bearer-token-value",
				"sk-openai-secret-value",
				"xoxb-slack-secret-value",
				"xapp-slack-app-secret",
				"ghp_githubsecretvalue",
			},
		},
		{
			name: "Should redact AGH MCP OAuth PKCE and binding secrets identically",
			input: strings.Join([]string{
				"agh_claim_raw-claim-value",
				"mcp_auth_token=mcp-token-value",
				"authorization_code=oauth-code-value",
				"code_verifier=pkce-verifier-value",
				"secret_ref=vault:bridges/slack/token",
			}, " "),
			want: strings.Join([]string{
				"agh_claim_[REDACTED]",
				"mcp_auth_token=[REDACTED]",
				"authorization_code=[REDACTED]",
				"code_verifier=[REDACTED]",
				"secret_ref=[REDACTED]",
			}, " "),
			leaks: []string{
				"raw-claim-value",
				"mcp-token-value",
				"oauth-code-value",
				"pkce-verifier-value",
				"vault:bridges/slack/token",
			},
		},
		{
			name: "Should redact secret suffixes and refs identically",
			input: strings.Join([]string{
				"workspace_secret=workspace-secret-value",
				"client_secret_ref=env:CLIENT_SECRET",
				"webhook_secret_ref=vault:webhook-secret",
			}, " "),
			want: strings.Join([]string{
				"workspace_secret=[REDACTED]",
				"client_secret_ref=[REDACTED]",
				"webhook_secret_ref=[REDACTED]",
			}, " "),
			leaks: []string{
				"workspace-secret-value",
				"env:CLIENT_SECRET",
				"vault:webhook-secret",
			},
		},
		{
			name: "Should redact generic quoted secret fields identically",
			input: `{"api_key":"api-value","auth_token":"auth-value",` +
				`"oauth_token":"oauth-value","access_token":"access-value",` +
				`"refresh_token":"refresh-value","id_token":"id-value",` +
				`"secret":"secret-value","password":"pass-value",` +
				`"credential":"cred-value","private_key":"private-value","safe":"visible"}`,
			want: `{"api_key":"[REDACTED]","auth_token":"[REDACTED]",` +
				`"oauth_token":"[REDACTED]","access_token":"[REDACTED]",` +
				`"refresh_token":"[REDACTED]","id_token":"[REDACTED]",` +
				`"secret":"[REDACTED]","password":"[REDACTED]",` +
				`"credential":"[REDACTED]","private_key":"[REDACTED]","safe":"visible"}`,
			leaks: []string{
				"api-value",
				"auth-value",
				"oauth-value",
				"access-value",
				"refresh-value",
				"id-value",
				"secret-value",
				"pass-value",
				"cred-value",
				"private-value",
			},
		},
		{
			name:  "Should redact complete authorization headers identically",
			input: "Proxy-Authorization: Basic dXNlcjpwYXNz",
			want:  "Proxy-Authorization: [REDACTED]",
			leaks: []string{"Basic dXNlcjpwYXNz"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			canonical := redactpkg.String(tc.input)
			if canonical != tc.want {
				t.Fatalf("redact.String() = %q, want %q", canonical, tc.want)
			}
			modelCatalog := RedactString(tc.input)
			if modelCatalog != tc.want {
				t.Fatalf("modelcatalog.RedactString() = %q, want %q", modelCatalog, tc.want)
			}
			rawInput, err := json.Marshal(map[string]string{"command": tc.input})
			if err != nil {
				t.Fatalf("marshal tool input: %v", err)
			}
			presentation := toolmeta.Render(
				"terminal.command",
				rawInput,
				toolmeta.RenderOptions{Verbose: true},
			)
			for _, leak := range tc.leaks {
				for consumer, value := range map[string]string{
					"redact":       canonical,
					"modelcatalog": modelCatalog,
					"toolmeta":     presentation.Preview,
				} {
					if strings.Contains(value, leak) {
						t.Fatalf("%s output %q leaked %q", consumer, value, leak)
					}
				}
			}
		})
	}
}
