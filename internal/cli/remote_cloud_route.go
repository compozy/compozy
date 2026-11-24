package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/internal/api/contract"
)

func newRemoteCloudRouteCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "cloud-route", Short: "Manage the hosted route for this computer"}
	cmd.AddCommand(
		remoteVerb(deps, "enable", "Turn on the hosted route", cobra.NoArgs, changeHostedRoute(true)),
		remoteVerb(deps, "disable", "Turn off the hosted route", cobra.NoArgs, changeHostedRoute(false)),
	)
	return cmd
}

func changeHostedRoute(enabled bool) func(*cobra.Command, remoteCommandClient, []string) error {
	return func(cmd *cobra.Command, client remoteCommandClient, _ []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
		defer cancel()
		status, err := client.PatchRemoteStatus(ctx, contract.RemotePatchRequest{CloudRoute: new(enabled)})
		if err != nil {
			return err
		}
		if enabled && !hostedRouteSettled(status) {
			initial := status
			status, err = pollUntil(ctx, time.Minute, 250*time.Millisecond, nil, "Waiting for the hosted route",
				func(ctx context.Context, _ pollEvent) (contract.RemoteStatusResponse, bool, error) {
					current, err := client.RemoteStatus(ctx)
					return current, hostedRouteSettled(current), err
				})
			if err != nil {
				if ctx.Err() == nil || cmd.Context().Err() != nil {
					return err
				}
				status = initial
			}
		}
		return writeCommandOutput(cmd, outputBundle{jsonValue: status, human: func() (string, error) {
			switch {
			case !status.CloudRoute.Enabled:
				return "Hosted route is off.", nil
			case status.CloudRoute.State == "ready" && status.CloudRoute.Hostname != nil:
				return fmt.Sprintf("Hosted route ready: %s", *status.CloudRoute.Hostname), nil
			case status.CloudRoute.Diagnosis != "":
				return status.CloudRoute.Diagnosis, nil
			default:
				return "Hosted route is starting.", nil
			}
		}})
	}
}

func hostedRouteSettled(status contract.RemoteStatusResponse) bool {
	switch status.CloudRoute.State {
	case "ready", "entitlement_only", "off":
		return true
	default:
		return status.CloudRoute.Diagnosis != ""
	}
}
