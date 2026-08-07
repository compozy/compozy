package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/gateway"
)

func defaultGatewayTierServerFactory(
	_ context.Context,
	deps *RuntimeDeps,
	tier gateway.Tier,
	surfaces []gateway.Surface,
) (Server, error) {
	surfaceSet, err := gatewaySurfaceSet(tier, surfaces)
	if err != nil {
		return nil, err
	}
	port := deps.Config.Gateway.PrivatePort
	if tier == gateway.TierPublic {
		port = deps.Config.Gateway.PublicPort
	}
	options := httpServerOptions(deps)
	options = append(options,
		httpapi.WithHost("127.0.0.1"),
		httpapi.WithPort(port),
		httpapi.WithSurfaceSet(surfaceSet),
		httpapi.WithGatewayService(deps.Gateway),
		httpapi.WithGatewayChallenge(tier, deps.GatewayChallenges),
		httpapi.WithDeviceAuth(deps.Gateway),
		httpapi.WithGatewayAuthLimiter(gateway.NewAuthFailureLimiter(
			deps.Config.Gateway.Auth.RateLimit.MaxFails,
			deps.Config.Gateway.Auth.RateLimit.Window,
			func() time.Time { return time.Now().UTC() },
		)),
	)
	return httpapi.New(options...)
}

func gatewaySurfaceSet(tier gateway.Tier, surfaces []gateway.Surface) (httpapi.SurfaceSet, error) {
	switch tier {
	case gateway.TierPrivate:
		if !slices.Contains(surfaces, gateway.SurfaceOperatorUI) {
			return "", fmt.Errorf("%w: private tier requires the operator UI surface", gateway.ErrExposureRefused)
		}
		return httpapi.SurfaceSetPrivate, nil
	case gateway.TierPublic:
		hasOperator := slices.Contains(surfaces, gateway.SurfaceOperatorUI)
		hasIngress := slices.Contains(surfaces, gateway.SurfaceWebhookIngress)
		switch {
		case hasOperator && hasIngress:
			return httpapi.SurfaceSetPublicCombined, nil
		case hasOperator:
			return httpapi.SurfaceSetPublicOperator, nil
		case hasIngress:
			return httpapi.SurfaceSetPublicIngress, nil
		default:
			return "", fmt.Errorf("%w: public tier has no supported surface", gateway.ErrExposureRefused)
		}
	default:
		return "", errors.New("daemon: gateway tier must be private or public")
	}
}
