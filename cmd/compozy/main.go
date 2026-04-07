package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/command"
	"github.com/compozy/compozy/internal/charmtheme"
	"github.com/compozy/compozy/internal/update"
	"github.com/compozy/compozy/internal/version"
)

func main() {
	os.Exit(run())
}

func run() int {
	cmd := command.New()
	cmd.Version = version.String()

	updateResult, cancel := startUpdateCheck(version.Version)
	err := cmd.Execute()
	cancel()

	if release := waitForUpdateResult(updateResult); release != nil {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), renderUpdateNotification(version.Version, release))
	}

	if err != nil {
		return command.ExitCode(err)
	}
	return 0
}

func startUpdateCheck(currentVersion string) (<-chan *update.ReleaseInfo, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan *update.ReleaseInfo, 1)

	go func() {
		defer close(result)

		statePath, err := update.StateFilePath()
		if err != nil {
			return
		}

		release, err := update.CheckForUpdate(ctx, currentVersion, statePath)
		if err != nil || release == nil {
			return
		}

		result <- release
	}()

	return result, cancel
}

func waitForUpdateResult(result <-chan *update.ReleaseInfo) *update.ReleaseInfo {
	if result == nil {
		return nil
	}
	return <-result
}

func renderUpdateNotification(currentVersion string, release *update.ReleaseInfo) string {
	styles := updateNotificationStyles{
		header:  lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorWarning),
		current: lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorMuted),
		arrow:   lipgloss.NewStyle().Foreground(charmtheme.ColorMuted),
		latest:  lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorBrand),
		body:    lipgloss.NewStyle().Foreground(charmtheme.ColorMuted),
	}

	lineOne := fmt.Sprintf(
		"%s %s %s %s",
		styles.header.Render("Update available:"),
		styles.current.Render(strings.TrimSpace(currentVersion)),
		styles.arrow.Render("->"),
		styles.latest.Render(release.Version),
	)
	lineTwo := styles.body.Render("Run 'compozy upgrade' to update")

	return lipgloss.JoinVertical(lipgloss.Left, lineOne, lineTwo)
}

type updateNotificationStyles struct {
	header  lipgloss.Style
	current lipgloss.Style
	arrow   lipgloss.Style
	latest  lipgloss.Style
	body    lipgloss.Style
}
