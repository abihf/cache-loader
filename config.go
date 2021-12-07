package loader

import (
	"context"
	"time"
)

// Fetcher loads the value based on key
type Fetcher[Key, Value any] func(ctx context.Context, key Key) (Value, error)

// ContextFactory creates context to be used by LoadFunc
type ContextFactory func() context.Context

type config struct {
	cf     ContextFactory
	driver CacheDriver

	ttl    time.Duration
	errTtl time.Duration
}

type Option func(cfg *config)

func WithDriver(driver CacheDriver) Option {
	return func(cfg *config) {
		cfg.driver = driver
	}
}

func WithErrorTTL(ttl time.Duration) Option {
	return func(cfg *config) {
		cfg.errTtl = ttl
	}
}

func WithContextFactory(cf ContextFactory) Option {
	return func(cfg *config) {
		cfg.cf = cf
	}
}
