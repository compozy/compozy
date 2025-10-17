---
title: "Postgres/Redis Pool Configuration Hardcoded"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "performance"
priority: "🟢 LOW"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_PERFORMANCE.md"
issue_index: "5"
sequence: "21"
---

## Postgres/Redis Pool Configuration Hardcoded

**Location:** `engine/infra/postgres/`, `engine/infra/redis/`

**Severity:** 🟢 LOW

**Issue:**
Default connection pool sizes are hardcoded in initialization code, preventing tuning for different deployment scenarios.

**Fix:**

```go
// pkg/config/database.go
type PostgresConfig struct {
    DSN             string        `yaml:"dsn" mapstructure:"dsn"`
    MaxOpenConns    int           `yaml:"max_open_conns" mapstructure:"max_open_conns"`
    MaxIdleConns    int           `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
    ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
    ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time"`
}

type RedisConfig struct {
    Addr         string        `yaml:"addr" mapstructure:"addr"`
    Password     string        `yaml:"password" mapstructure:"password"`
    DB           int           `yaml:"db" mapstructure:"db"`
    PoolSize     int           `yaml:"pool_size" mapstructure:"pool_size"`
    MinIdleConns int           `yaml:"min_idle_conns" mapstructure:"min_idle_conns"`
    MaxRetries   int           `yaml:"max_retries" mapstructure:"max_retries"`
}

func DefaultPostgresConfig() PostgresConfig {
    return PostgresConfig{
        MaxOpenConns:    25,  // Default reasonable for most deployments
        MaxIdleConns:    5,
        ConnMaxLifetime: 5 * time.Minute,
        ConnMaxIdleTime: 1 * time.Minute,
    }
}

func DefaultRedisConfig() RedisConfig {
    numCPU := runtime.NumCPU()
    return RedisConfig{
        PoolSize:     numCPU * 10,  // 10 connections per CPU core
        MinIdleConns: numCPU * 2,   // 2 idle connections per core
        MaxRetries:   3,
    }
}
```

**Apply configuration:**

```go
// engine/infra/postgres/client.go
func NewClient(ctx context.Context, cfg *config.PostgresConfig) (*sql.DB, error) {
    db, err := sql.Open("postgres", cfg.DSN)
    if err != nil {
        return nil, err
    }

    db.SetMaxOpenConns(cfg.MaxOpenConns)
    db.SetMaxIdleConns(cfg.MaxIdleConns)
    db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
    db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

    return db, nil
}

// engine/infra/redis/client.go
func NewClient(ctx context.Context, cfg *config.RedisConfig) (*redis.Client, error) {
    return redis.NewClient(&redis.Options{
        Addr:         cfg.Addr,
        Password:     cfg.Password,
        DB:           cfg.DB,
        PoolSize:     cfg.PoolSize,
        MinIdleConns: cfg.MinIdleConns,
        MaxRetries:   cfg.MaxRetries,
    }), nil
}
```

**Configuration example:**

```yaml
# config.yaml
postgres:
  dsn: "postgres://user:pass@localhost/compozy"
  max_open_conns: 50 # Increase for high-traffic deployments
  max_idle_conns: 10
  conn_max_lifetime: 5m
  conn_max_idle_time: 1m

redis:
  addr: "localhost:6379"
  pool_size: 80 # 8 cores * 10
  min_idle_conns: 16 # 8 cores * 2
  max_retries: 3
```

**Tuning guidelines:**

```go
// Add to documentation
/*
Connection Pool Sizing Guidelines:

Postgres:
- max_open_conns = (num_cores * 2) + disk_io_parallelism
- For 8-core machine with SSD: 25-50 connections
- For 16-core machine: 50-100 connections
- max_idle_conns = max_open_conns / 5

Redis:
- pool_size = num_cores * 10
- For 8-core machine: 80 connections
- min_idle_conns = num_cores * 2
*/
```

**Impact:**

- **Tuning flexibility:** Optimize for deployment size
- **Resource efficiency:** Fewer idle connections in small deployments
- **Scalability:** More connections in large deployments

**Effort:** S (2h)  
**Risk:** None - defaults unchanged
