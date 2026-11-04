package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/profiling"
	"github.com/spf13/cobra"
)

type daemonProfileClient interface {
	CaptureDaemonProfile(context.Context, string, int, io.Writer) error
}

var _ daemonProfileClient = (*daemonClient)(nil)

func newDaemonProfileCommand(deps commandDeps) *cobra.Command {
	var file string
	var seconds int
	cmd := &cobra.Command{
		Use:   "profile <kind>",
		Short: "Capture a runtime profile over the local daemon socket (experimental)",
		Long: `Capture cpu, heap, allocs, goroutine, mutex, block or threadcreate as a raw .pprof file.
Experimental; local UDS only. CPU, mutex and block captures sample for 1–120 seconds.
Mutex/block sampling is enabled only during the capture. Profiles contain runtime stacks,
counts and ID labels, without prompt or file contents. Concurrent CPU captures return
profile_busy. Snapshot profiles reject --seconds. Inspect with go tool pprof; use -tags
to list labels and -tagfocus session_id=<id> to select one active session.`,
		Example: `  compozy daemon profile cpu --seconds 30 --file cpu.pprof
  compozy daemon profile heap --file heap.pprof
  go tool pprof -top cpu.pprof
  go tool pprof -http :0 cpu.pprof
  go tool pprof -tagfocus session_id=sess-example cpu.pprof`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"cpu", "heap", "allocs", "goroutine", "mutex", "block", "threadcreate"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := profiling.Validate(args[0], seconds); err != nil {
				return err
			}
			if cmd.Flags().Changed("seconds") && !profiling.Windowed(args[0]) {
				return errors.New("cli: --seconds applies only to cpu, mutex and block")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			profiler, ok := client.(daemonProfileClient)
			if !ok {
				return errors.New("cli: daemon client does not support profiling")
			}
			if err := writeDaemonProfile(cmd.Context(), profiler, args[0], seconds, file); err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{
				human:     func() (string, error) { return "Profile saved to " + file + "\n", nil },
				jsonValue: map[string]string{"kind": args[0], "path": file},
			})
		},
	}
	cmd.Flags().
		IntVar(&seconds, "seconds", profiling.DefaultSeconds, "Capture duration for cpu, mutex and block (1–120 seconds)")
	cmd.Flags().StringVar(&file, "file", "", "Destination .pprof file (required)")
	mustMarkFlagRequired(cmd, "file")
	return cmd
}

func writeDaemonProfile(
	ctx context.Context,
	client daemonProfileClient,
	kind string,
	seconds int,
	path string,
) (err error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".profile-*.pprof")
	if err != nil {
		return fmt.Errorf("cli: create profile file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			err = errors.Join(err, closeErr)
		}
		if removeErr := os.Remove(file.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	if err := client.CaptureDaemonProfile(ctx, kind, seconds, file); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("cli: save profile: %w", err)
	}
	return nil
}

func (c *daemonClient) CaptureDaemonProfile(ctx context.Context, kind string, seconds int, dst io.Writer) (err error) {
	if c.target.kind != clientTargetLocal {
		return errors.New("cli: profiling requires a local Unix socket connection")
	}
	if err := profiling.Validate(kind, seconds); err != nil {
		return err
	}
	query := url.Values{}
	if profiling.Windowed(kind) {
		query.Set("seconds", strconv.Itoa(seconds))
	}
	client := *c.httpClient
	client.Timeout = time.Duration(profiling.MaxSeconds+10) * time.Second
	response, err := c.doRequestWithReaderAndClient(ctx, clientReaderRequest{
		Method: http.MethodGet, Path: "/api/daemon/profile/" + url.PathEscape(kind), Query: query, Client: &client,
	})
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(
			err,
			drainResponseBody(http.MethodGet, "/api/daemon/profile", response.Body),
			response.Body.Close(),
		)
	}()
	if response.StatusCode != http.StatusOK {
		return readAPIError(response)
	}
	_, err = io.Copy(dst, response.Body)
	return err
}
