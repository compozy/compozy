package core

import (
	"context"
)

type Store interface {
	Get(ctx context.Context, key []byte) ([]byte, error)
	Save(ctx context.Context, key, value []byte) error
	SaveJSON(ctx context.Context, key []byte, obj any) error
	Update(ctx context.Context, key, value []byte) error
	UpdateJSON(ctx context.Context, key []byte, obj any) error
	Upsert(ctx context.Context, key, value []byte) error
	UpsertJSON(ctx context.Context, key []byte, obj any) error
	Delete(ctx context.Context, key []byte) error
	Close() error
	CloseWithContext(ctx context.Context) error
	DataDir() string
}
