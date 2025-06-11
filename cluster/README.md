# Compozy Infrastructure Setup

This directory contains the Docker Compose configuration for running Compozy's infrastructure components including PostgreSQL, Temporal, and Redis.

## Services Overview

| Service                   | Purpose                          | Port | Container Name        |
| ------------------------- | -------------------------------- | ---- | --------------------- |
| **Redis**                 | Caching, Config Storage, Pub/Sub | 6379 | `redis`               |
| **PostgreSQL (App)**      | Application data persistence     | 5432 | `app-postgresql`      |
| **PostgreSQL (Temporal)** | Temporal workflow state          | 5433 | `temporal-postgresql` |
| **PostgreSQL (Test)**     | Test database (isolated)         | 5434 | `test-postgresql`     |
| **Temporal**              | Workflow orchestration           | 7233 | `temporal`            |
| **Temporal UI**           | Workflow monitoring dashboard    | 8080 | `temporal-ui`         |

## Quick Start

1. **Copy environment configuration**:

    ```bash
    cp .env.example .env
    # Edit .env with your desired configuration
    ```

2. **Start all services**:

    ```bash
    make start-docker
    ```

3. **Verify Redis is running**:

    ```bash
    make test-redis
    ```

4. **Run database migrations**:
    ```bash
    make migrate-up
    ```

## Redis Configuration

### Development Setup

Redis is configured with:

- **Password protection**: `redis_secret` (configurable via `REDIS_PASSWORD`)
- **Memory limit**: 512MB with LRU eviction policy
- **Persistence**: RDB snapshots for development data retention
- **Pub/Sub notifications**: Enabled for keyspace events

### Environment Variables

| Variable          | Default        | Description                |
| ----------------- | -------------- | -------------------------- |
| `REDIS_HOST`      | `localhost`    | Redis host                 |
| `REDIS_PORT`      | `6379`         | Redis port                 |
| `REDIS_PASSWORD`  | `redis_secret` | Redis password             |
| `REDIS_VERSION`   | `7.2-alpine`   | Redis Docker image version |
| `REDIS_MAXMEMORY` | `512mb`        | Memory limit               |

### Redis Commands

```bash
# Interactive Redis CLI
make redis-cli

# Get Redis info
make redis-info

# Monitor Redis commands (dev only)
make redis-monitor

# Flush all Redis data
make redis-flush

# Test Redis connection
make test-redis
```

## Security Considerations

### Development

- Redis is password-protected
- Dangerous commands (FLUSHDB, FLUSHALL) are disabled
- Protected mode is disabled for local development

### Production

For production deployments:

- Enable TLS encryption
- Use Redis Sentinel or Cluster for high availability
- Implement proper network isolation
- Use strong passwords and consider ACL authentication
- Enable audit logging

## Data Persistence

### Redis

- RDB snapshots are saved to `redis_data` volume
- Snapshots occur at: 900s (1 change), 300s (10 changes), 60s (10000 changes)
- Data persists across container restarts

### PostgreSQL

- Each database has its own persistent volume
- Test database can be safely reset without affecting development data

## Monitoring & Debugging

### Health Checks

All services include health checks:

- Redis: `ping` command
- PostgreSQL: `pg_isready`
- Temporal: workflow list command

### Logging

- Use `docker logs <container_name>` to view service logs
- Redis logs are set to `notice` level for development

### Troubleshooting

**Redis connection issues**:

```bash
# Check if Redis is running
docker ps | grep redis

# Check Redis logs
docker logs redis

# Test connection manually
docker exec redis redis-cli -a redis_secret ping
```

**Database connection issues**:

```bash
# Check if PostgreSQL is running
docker ps | grep postgresql

# Test application database connection
docker exec app-postgresql pg_isready -U postgres
```

## Configuration Files

- `docker-compose.yml`: Main infrastructure definition
- `.env.example`: Environment variables template
- `redis-dev.conf`: Redis development configuration
- `temporal-dev.yaml`: Temporal development settings

## Next Steps

1. ✅ Redis infrastructure setup complete
2. 🔄 Implement Redis client integration in Go application
3. 🔄 Create ConfigStore interface implementation
4. 🔄 Add cache layer for workflow/task states
5. 🔄 Implement distributed locking with Redlock
6. 🔄 Add pub/sub notification system
