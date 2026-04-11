package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy"
	"github.com/compozy/compozy/internal/charmtheme"
	"github.com/compozy/compozy/internal/update"
	"github.com/compozy/compozy/internal/version"
)

const updateResultWaitTimeout = 250 * time.Millisecond

func main() {
	os.Exit(run())
}

func run() int {
	cmd := compozy.NewCommand()
	cmd.Version = version.String()

	updateResult, cancelUpdateCheck, updateDone := startUpdateCheck(context.Background(), version.Version)
	err := cmd.Execute()
	cancelUpdateCheck()

	if release := waitForUpdateResult(updateResult); release != nil {
		if writeErr := writeUpdateNotification(
			cmd.ErrOrStderr(),
			version.Version,
			release,
		); writeErr != nil &&
			err == nil {
			err = fmt.Errorf("write update notification: %w", writeErr)
		}
	}
	<-updateDone

	if err != nil {
		return compozy.ExitCode(err)
	}
	return 0
}

func startUpdateCheck(
	parent context.Context,
	currentVersion string,
) (<-chan *update.ReleaseInfo, context.CancelFunc, <-chan struct{}) {
	ctx, cancel := context.WithCancel(parent)
	result := make(chan *update.ReleaseInfo, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
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

	return result, cancel, done
}

func waitForUpdateResult(result <-chan *update.ReleaseInfo) *update.ReleaseInfo {
	if result == nil {
		return nil
	}
	select {
	case release, ok := <-result:
		if !ok {
			return nil
		}
		return release
	case <-time.After(updateResultWaitTimeout):
		return nil
	}
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

func writeUpdateNotification(w io.Writer, currentVersion string, release *update.ReleaseInfo) error {
	if release == nil {
		return nil
	}
	_, err := fmt.Fprintln(w, renderUpdateNotification(currentVersion, release))
	return err
}

type updateNotificationStyles struct {
	header  lipgloss.Style
	current lipgloss.Style
	arrow   lipgloss.Style
	latest  lipgloss.Style
	body    lipgloss.Style
}
