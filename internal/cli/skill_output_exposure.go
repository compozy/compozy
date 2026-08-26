package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type skillCreateExposureOutput struct {
	Created  skillCreateItem                             `json:"created"`
	Exposure []contract.SkillExposureTargetResultPayload `json:"results"`
}

func skillExposureSuccessBundle(success skillExposureSuccess) outputBundle {
	jsonValue := success.JSONValue
	if jsonValue == nil {
		jsonValue = success.Results
	}
	return outputBundle{
		jsonValue: jsonValue,
		human: func() (string, error) {
			if success.Action != skillExposeAction && success.Action != skillUnexposeAction {
				return "", fmt.Errorf("cli: unsupported skill exposure action %q", success.Action)
			}
			lines := make([]string, 0, len(success.Results))
			for _, result := range success.Results {
				path := ""
				if result.Exposure != nil {
					path = result.Exposure.Path
				}
				switch success.Action {
				case skillExposeAction:
					if success.Preexisting[result.Target] {
						lines = append(lines, fmt.Sprintf("already exposed: %s → %s (no change)", success.Name, path))
					} else {
						lines = append(lines, fmt.Sprintf("exposed %s → %s", success.Name, path))
					}
				case skillUnexposeAction:
					lines = append(lines, fmt.Sprintf("unexposed %s ← %s", success.Name, path))
				}
			}
			return strings.Join(lines, "\n"), nil
		},
		toon: func() (string, error) {
			return renderToonArray(
				"results",
				[]string{automationTargetKey, "ok", cliPathKey},
				skillExposureToonRows(success),
			), nil
		},
	}
}

func skillCreateExposureBundle(
	created skillCreateItem,
	relativeFile string,
	exposure skillExposureSuccess,
) outputBundle {
	bundle := skillExposureSuccessBundle(exposure)
	bundle.jsonValue = skillCreateExposureOutput{Created: created, Exposure: exposure.Results}
	exposureHuman := bundle.human
	bundle.human = func() (string, error) {
		rendered, err := exposureHuman()
		if err != nil {
			return "", err
		}
		return "created " + relativeFile + "\n" + rendered, nil
	}
	exposureToon := bundle.toon
	bundle.toon = func() (string, error) {
		rendered, err := exposureToon()
		if err != nil {
			return "", err
		}
		return renderHumanBlocks(
			renderToonObject("created", []string{
				automationNameKey, cliPathKey, cliFileKey, cliSourceKey, automationStatusKey,
			}, []string{
				created.Name, created.Path, created.File, created.Source, created.Status,
			}),
			rendered,
		), nil
	}
	return bundle
}

func skillExposureToonRows(success skillExposureSuccess) [][]string {
	rows := make([][]string, 0, len(success.Results))
	for _, result := range success.Results {
		path := ""
		if result.Exposure != nil {
			path = result.Exposure.Path
		}
		rows = append(rows, []string{result.Target, fmt.Sprintf("%t", result.OK), path})
	}
	return rows
}
