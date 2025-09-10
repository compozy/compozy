package webhook

import (
	"context"
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockVerifier implements Verifier for testing
type MockVerifier struct {
	mock.Mock
}

func (m *MockVerifier) Verify(ctx context.Context, r *http.Request, body []byte) error {
	args := m.Called(ctx, r, body)
	return args.Error(0)
}

// MockLookup implements Lookup for testing
type MockLookup struct {
	mock.Mock
}

func (m *MockLookup) Get(slug string) (RegistryEntry, bool) {
	args := m.Called(slug)
	entry, ok := args.Get(0).(RegistryEntry)
	if !ok {
		var zero RegistryEntry
		return zero, false
	}
	return entry, args.Bool(1)
}

// MockRedisClient implements RedisClient for testing
type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	args := m.Called(ctx, key, value, expiration)
	return args.Bool(0), args.Error(1)
}

// MockRedisService implements Service for testing
type MockRedisService struct {
	mock.Mock
}

func (m *MockRedisService) CheckAndSet(ctx context.Context, key string, ttl time.Duration) error {
	args := m.Called(ctx, key, ttl)
	return args.Error(0)
}
