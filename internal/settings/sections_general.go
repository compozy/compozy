package settings

import (
	"context"
	"fmt"

	aghconfig "github.com/compozy/agh/internal/config"
)

func (s *service) buildGeneralSection(ctx context.Context, cfg *aghconfig.Config) (GeneralSection, error) {
	runtime := DaemonRuntimeStatus{}
	if s.generalRuntime != nil {
		status, err := s.generalRuntime.GeneralRuntimeStatus(ctx)
		if err != nil {
			return GeneralSection{}, fmt.Errorf("settings: general runtime status: %w", err)
		}
		runtime = status
	}

	return GeneralSection{
		Runtime: runtime,
		ConfigPaths: ConfigPaths{
			HomeDir:          s.homePaths.HomeDir,
			GlobalConfig:     s.homePaths.ConfigFile,
			GlobalMCPSidecar: globalMCPSidecarPath(s.homePaths),
			LogFile:          s.homePaths.LogFile,
			DaemonInfo:       s.homePaths.DaemonInfo,
		},
		Settings: generalSettingsFromConfig(cfg),
		Actions: GeneralActions{
			Restart: ActionMetadata{
				Name:      sectionsRestartKey,
				Available: s.restartActionAvailable,
				Behavior:  MutationBehaviorActionTrigger,
			},
		},
	}, nil
}
