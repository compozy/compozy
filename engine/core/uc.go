package core

import "context"

type Usecase[T any] interface {
	Execute(ctx context.Context) (T, error)
}
