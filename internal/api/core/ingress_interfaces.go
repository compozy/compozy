package core

import (
	"context"

	"github.com/compozy/compozy/internal/ingress"
)

// IngressService manages confirmed webhook bindings and their public projections.
type IngressService interface {
	Bind(context.Context, ingress.BindRequest) (ingress.Binding, error)
	Unbind(context.Context, ingress.UnbindRequest) (ingress.UnbindResult, error)
	Project(context.Context, ingress.SubjectRef) (ingress.Projection, error)
}
