package workflow

import (
	"fmt"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
)

func LoadTasksRef(wc *Config) error {
	for i := 0; i < len(wc.Tasks); i++ {
		tc := &wc.Tasks[i]
		cfg, err := tc.LoadFileRef(wc.GetCWD())
		if err != nil {
			return fmt.Errorf("failed to load task reference for task %s: %w", tc.ID, err)
		}
		if cfg != nil {
			wc.Tasks[i] = *cfg
			if err := loadReferencedComponents(wc, cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadReferencedComponents(wc *Config, tc *task.Config) error {
	if err := loadAgentsRefOnTask(wc, tc); err != nil {
		return fmt.Errorf("failed to load agent reference for task %s: %w", tc.ID, err)
	}

	if err := loadToolsRefOnTask(wc, tc); err != nil {
		return fmt.Errorf("failed to load tool reference for task %s: %w", tc.ID, err)
	}

	return nil
}

func loadAgentsRefOnTask(wc *Config, tc *task.Config) error {
	if tc.Use == nil {
		return nil
	}

	ref, err := tc.Use.IntoRef()
	if err != nil {
		return err
	}

	if !ref.Type.IsFile() || !ref.Component.IsAgent() {
		return nil
	}

	cfg, err := agent.Load(tc.GetCWD(), ref.Value())
	if err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid agent configuration: %w", err)
	}

	for i := 0; i < len(wc.Agents); i++ {
		ac := &wc.Agents[i]
		if ac.ID == cfg.ID {
			return nil
		}
	}

	wc.Agents = append(wc.Agents, *cfg)
	return nil
}

func loadToolsRefOnTask(wc *Config, tc *task.Config) error {
	if tc.Use == nil {
		return nil
	}

	ref, err := tc.Use.IntoRef()
	if err != nil {
		return err
	}

	if !ref.Type.IsFile() || !ref.Component.IsTool() {
		return nil
	}

	cfg, err := tool.Load(tc.GetCWD(), ref.Value())
	if err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid tool configuration: %w", err)
	}

	for _, tc := range wc.Tools {
		if tc.ID == cfg.ID {
			return nil
		}
	}

	wc.Tools = append(wc.Tools, *cfg)
	return nil
}
