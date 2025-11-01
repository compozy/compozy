package basic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/template"
)

func TestTemplateGetFilesWithOptions(t *testing.T) {
	t.Parallel()
	tmpl := &Template{}
	tests := []struct {
		name        string
		mode        string
		dockerSetup bool
		wantDocker  bool
	}{
		{name: "memory skips docker", mode: "memory", dockerSetup: true, wantDocker: false},
		{name: "persistent skips docker", mode: "persistent", dockerSetup: true, wantDocker: false},
		{name: "distributed includes docker", mode: "distributed", dockerSetup: true, wantDocker: true},
		{name: "docker disabled ignored", mode: "distributed", dockerSetup: false, wantDocker: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := &template.GenerateOptions{
				Mode:        tt.mode,
				DockerSetup: tt.dockerSetup,
			}
			files := tmpl.GetFilesWithOptions(opts)
			hasDocker := fileExists(files, "docker-compose.yaml")
			require.Equalf(t, tt.wantDocker, hasDocker, "unexpected docker-compose inclusion for mode %s", tt.mode)
		})
	}
}

func TestTemplateGenerateProducesModeSpecificArtifacts(t *testing.T) {
	t.Parallel()
	svc := template.GetService()
	templateName := fmt.Sprintf("basic-%s-%d", strings.ToLower(t.Name()), time.Now().UnixNano())
	require.NoError(t, svc.Register(templateName, &Template{}))
	cases := []struct {
		name              string
		mode              string
		wantDocker        bool
		compozyContains   []string
		compozyNotContain []string
		envContains       []string
		readmeContains    []string
		gitignoreContains []string
		gitignoreAbsent   []string
	}{
		{
			name:       "memory mode",
			mode:       "memory",
			wantDocker: false,
			compozyContains: []string{
				"mode: memory",
				`driver: sqlite`,
				`url: ":memory:"`,
				"temporal:\n  mode: memory",
			},
			compozyNotContain: []string{"${COMPOZY_DATABASE_URL}"},
			envContains: []string{
				"COMPOZY_MODE=memory",
				"# Memory Mode - zero dependencies",
			},
			readmeContains: []string{
				"Memory Mode (Zero Dependencies)",
				"The runtime boots in under a second.",
			},
			gitignoreContains: []string{".compozy/"},
		},
		{
			name:       "persistent mode",
			mode:       "persistent",
			wantDocker: false,
			compozyContains: []string{
				"mode: persistent",
				`driver: sqlite`,
				`url: ./.compozy/{{APP_KEBAB}}.db`,
				"redis:\n  mode: persistent",
			},
			compozyNotContain: []string{"${COMPOZY_DATABASE_URL}"},
			envContains: []string{
				"COMPOZY_MODE=persistent",
				"# Persistent Mode - override paths",
				"# COMPOZY_DATABASE_URL=./.compozy/{{APP_KEBAB}}.db",
			},
			readmeContains: []string{
				"Persistent Mode (Local Development)",
				"Compozy creates the `./.compozy/` directory automatically",
			},
			gitignoreContains: []string{".compozy/"},
		},
		{
			name:       "distributed mode",
			mode:       "distributed",
			wantDocker: true,
			compozyContains: []string{
				"mode: distributed",
				"driver: postgres",
				"url: ${COMPOZY_DATABASE_URL}",
				"redis:\n  mode: distributed",
			},
			envContains: []string{
				"COMPOZY_MODE=distributed",
				"COMPOZY_DATABASE_URL=postgresql://user:password@localhost:5432/{{APP_KEBAB}}",
				"TEMPORAL_NAMESPACE={{APP_KEBAB}}-prod",
			},
			readmeContains: []string{
				"Distributed Mode (Production-Ready)",
				"docker-compose up -d",
			},
			gitignoreAbsent: []string{".compozy/"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			output := t.TempDir()
			ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
			appName := fmt.Sprintf("%s-app", tc.mode)
			kebabName := toKebab(appName)
			replacements := map[string]string{
				"{{APP}}":       appName,
				"{{APP_KEBAB}}": kebabName,
			}
			opts := &template.GenerateOptions{
				Context:     ctx,
				Path:        output,
				Name:        appName,
				Description: fmt.Sprintf("%s project", tc.mode),
				Version:     "0.1.0",
				Mode:        tc.mode,
				DockerSetup: true,
			}
			require.NoError(t, svc.Generate(templateName, opts))
			verifyDockerPresence(t, output, tc.wantDocker)
			compozy := readFile(t, filepath.Join(output, "compozy.yaml"))
			assertContainsAll(t, compozy, formatExpectations(tc.compozyContains, replacements))
			assertNotContainsAny(t, compozy, formatExpectations(tc.compozyNotContain, replacements))
			env := readFile(t, filepath.Join(output, "env.example"))
			assertContainsAll(t, env, formatExpectations(tc.envContains, replacements))
			readme := readFile(t, filepath.Join(output, "README.md"))
			assertContainsAll(t, readme, formatExpectations(tc.readmeContains, replacements))
			gitignore := readFile(t, filepath.Join(output, ".gitignore"))
			assertContainsAll(t, gitignore, formatExpectations(tc.gitignoreContains, replacements))
			assertNotContainsAny(t, gitignore, formatExpectations(tc.gitignoreAbsent, replacements))
		})
	}
}

func fileExists(files []template.File, name string) bool {
	for _, file := range files {
		if file.Name == name {
			return true
		}
	}
	return false
}

func verifyDockerPresence(t *testing.T, dir string, want bool) {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, "docker-compose.yaml"))
	if want {
		require.NoError(t, err)
		return
	}
	require.ErrorIs(t, err, os.ErrNotExist)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func formatExpectations(values []string, replacements map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	formatted := make([]string, 0, len(values))
	for _, v := range values {
		replaced := v
		for key, val := range replacements {
			replaced = strings.ReplaceAll(replaced, key, val)
		}
		formatted = append(formatted, replaced)
	}
	return formatted
}

func toKebab(value string) string {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return ""
	}
	clean = strings.ReplaceAll(clean, "_", "-")
	clean = strings.ReplaceAll(clean, " ", "-")
	for strings.Contains(clean, "--") {
		clean = strings.ReplaceAll(clean, "--", "-")
	}
	return strings.ToLower(clean)
}

func assertContainsAll(t *testing.T, content string, expected []string) {
	t.Helper()
	for _, piece := range expected {
		if piece == "" {
			continue
		}
		require.Contains(t, content, piece)
	}
}

func assertNotContainsAny(t *testing.T, content string, forbidden []string) {
	t.Helper()
	for _, piece := range forbidden {
		if piece == "" {
			continue
		}
		require.NotContains(t, content, piece)
	}
}
