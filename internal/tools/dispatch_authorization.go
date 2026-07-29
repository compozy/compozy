package tools

import "context"

func (r *RuntimeRegistry) authorizeBoundCallInput(
	ctx context.Context,
	scope Scope,
	descriptor Descriptor,
	request CallRequest,
) error {
	if r.callInputAuthorizer == nil {
		return nil
	}
	return r.callInputAuthorizer.AuthorizeCallInput(ctx, scope, descriptor, request)
}
