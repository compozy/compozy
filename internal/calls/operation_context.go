package calls

import "context"

func (s *Service) detachedOperationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(ctx), s.operationTimeout)
}
