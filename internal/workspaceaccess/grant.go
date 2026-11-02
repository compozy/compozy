package workspaceaccess

import "context"

const Unrestricted = "unrestricted"

// ActorApp marks work created through an app token. App tokens never carry the unrestricted grant
// (USW Extensibility, Authority Matrix), so app-originated work gets no ceiling.
const ActorApp ActorKind = "app"

type Origin struct {
	ID      string
	Kind    ActorKind
	Ceiling string
}

func (o Origin) Effective(definition string) string {
	if definition == Unrestricted && (o.Kind == ActorHuman || o.Ceiling == Unrestricted) {
		return Unrestricted
	}
	return ""
}

func NarrowGrant(existing, ceiling string) string {
	if existing == Unrestricted && ceiling == Unrestricted {
		return Unrestricted
	}
	return ""
}

type originKey struct{}

func WithOrigin(ctx context.Context, origin Origin) context.Context {
	return context.WithValue(ctx, originKey{}, origin)
}

func OriginFromContext(ctx context.Context) Origin {
	origin, ok := ctx.Value(originKey{}).(Origin)
	if !ok {
		return Origin{}
	}
	return origin
}

func AuthoringCeiling(ctx context.Context, operator bool) string {
	if origin, ok := ctx.Value(originKey{}).(Origin); ok {
		return origin.Effective(Unrestricted)
	}
	if operator {
		return Unrestricted
	}
	return ""
}
