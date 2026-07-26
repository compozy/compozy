package extensionpkg

import (
	"context"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

type hostAPIDeliveryBroker interface {
	RegisterPromptDelivery(
		ctx context.Context,
		reg bridgepkg.PromptDeliveryRegistration,
	) (*bridgepkg.DeliverySnapshot, error)
	ProjectEvent(ctx context.Context, sessionID string, event bridgepkg.DeliveryProjectionEvent) error
}

type hostAPIDeliveryAdmissionChecker interface {
	CheckPromptDeliveryAdmission(ctx context.Context) error
}
