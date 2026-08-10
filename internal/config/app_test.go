package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppConfigLifecycleMatchesSharedCorpus(t *testing.T) {
	t.Parallel()

	type fixture struct {
		Defaults struct {
			UpdateCheck         bool   `json:"update_check"`
			UpdateCheckInterval string `json:"update_check_interval"`
		} `json:"defaults"`
		ValidIntervals   []string `json:"valid_intervals"`
		InvalidIntervals []string `json:"invalid_intervals"`
		InvalidDocuments []struct {
			Name string `json:"name"`
			TOML string `json:"toml"`
		} `json:"invalid_documents"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "desktop", "schema", "fixtures", "app-config.json"))
	if err != nil {
		t.Fatalf("read shared app config corpus: %v", err)
	}
	var corpus fixture
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode shared app config corpus: %v", err)
	}

	t.Run("Should apply defaults from the shared corpus", func(t *testing.T) {
		t.Parallel()
		wantInterval, err := time.ParseDuration(corpus.Defaults.UpdateCheckInterval)
		if err != nil {
			t.Fatalf("ParseDuration(default) error = %v", err)
		}
		defaults := DefaultWithHome(HomePaths{})
		if defaults.App.UpdateCheck != corpus.Defaults.UpdateCheck ||
			defaults.App.UpdateCheckInterval != wantInterval {
			t.Fatalf("DefaultWithHome().App = %#v, want corpus defaults %#v", defaults.App, corpus.Defaults)
		}
	})

	for _, interval := range corpus.ValidIntervals {
		t.Run("Should accept shared valid interval "+interval, func(t *testing.T) {
			t.Parallel()
			overlay, decodeErr := loadConfigOverlayBytes(
				[]byte("[app]\nupdate_check_interval = \""+interval+"\"\n"),
				"shared-app-config.toml",
			)
			if decodeErr != nil {
				t.Fatalf("loadConfigOverlayBytes(%q) error = %v", interval, decodeErr)
			}
			cfg := DefaultWithHome(HomePaths{})
			if err := overlay.Apply(&cfg); err != nil {
				t.Fatalf("configOverlay.Apply() error = %v", err)
			}
			if err := cfg.App.Validate(); err != nil {
				t.Fatalf("AppConfig.Validate() error = %v", err)
			}
		})
	}
	for _, interval := range corpus.InvalidIntervals {
		t.Run("Should reject shared invalid interval "+interval, func(t *testing.T) {
			t.Parallel()
			overlay, decodeErr := loadConfigOverlayBytes(
				[]byte("[app]\nupdate_check_interval = \""+interval+"\"\n"),
				"shared-app-config.toml",
			)
			if decodeErr != nil {
				return
			}
			cfg := DefaultWithHome(HomePaths{})
			if err := overlay.Apply(&cfg); err != nil {
				t.Fatalf("configOverlay.Apply() error = %v", err)
			}
			err := cfg.App.Validate()
			var validationErr ValidationError
			if !errors.As(err, &validationErr) || validationErr.Path != appUpdateCheckIntervalPath {
				t.Fatalf("AppConfig.Validate() error = %#v, want path %q", err, appUpdateCheckIntervalPath)
			}
		})
	}
	for _, document := range corpus.InvalidDocuments {
		t.Run("Should reject shared invalid document "+document.Name, func(t *testing.T) {
			t.Parallel()
			if _, err := loadConfigOverlayBytes([]byte(document.TOML), "shared-app-config.toml"); err == nil {
				t.Fatalf("loadConfigOverlayBytes(%q) error = nil, want rejection", document.Name)
			}
		})
	}

	t.Run("Should preserve configured app values through TOML load", func(t *testing.T) {
		homePaths, resolveErr := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if resolveErr != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", resolveErr)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, "[app]\nupdate_check = false\nupdate_check_interval = \"12h\"\n")
		loaded, loadErr := LoadForHome(homePaths)
		if loadErr != nil {
			t.Fatalf("LoadForHome() error = %v", loadErr)
		}
		if loaded.App.UpdateCheck || loaded.App.UpdateCheckInterval != 12*time.Hour {
			t.Fatalf("LoadForHome().App = %#v, want disabled 12h", loaded.App)
		}
	})
}
