package daemon

import (
	"strings"

	"github.com/compozy/compozy/internal/resources"
)

const extensionResourceOwnerKind resources.ResourceOwnerKind = "extension"

func extensionOwner(name string) *resources.ResourceOwner {
	return &resources.ResourceOwner{
		Kind: extensionResourceOwnerKind,
		ID:   strings.TrimSpace(name),
	}
}

func managedDraftOwner(actor resources.MutationActor, explicit *resources.ResourceOwner) resources.ResourceOwner {
	if explicit != nil {
		return explicit.Normalize()
	}
	owner := actor.Owner.Normalize()
	if owner != (resources.ResourceOwner{}) {
		return owner
	}
	return resources.ResourceOwner{Kind: resources.ResourceOwnerKind(actor.Kind), ID: strings.TrimSpace(actor.ID)}
}
