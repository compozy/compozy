# Suggested Commands for Compozy Development

## Essential Development Commands

### Quick Setup

```bash
make deps && make start-docker && make migrate-up
```

### Development Server

```bash
make dev                 # Start development with hot reload (default: weather example)
make dev EXAMPLE=signals # Start with specific example
```

### Testing

```bash
make test          # Run tests (excludes E2E/slow tests)
make test-coverage # Run tests with coverage
make test-nocache  # Run tests without cache
```

### Code Quality (ALWAYS run before committing)

```bash
make fmt && make lint # Format and lint code
```

### Database Management

```bash
make migrate-up     # Apply migrations
make migrate-down   # Rollback last migration
make migrate-status # Check migration status
make reset-db       # Reset database completely
```

### Docker Management

```bash
make start-docker # Start docker services
make stop-docker  # Stop docker services
make clean-docker # Clean volumes
make reset-docker # Full reset
```

### Build and Release

```bash
make build     # Build binary
make swagger   # Generate API documentation
make schemagen # Generate schemas
```

### Specific Testing

```bash
go test -v ./engine/task -run TestExecutor_Execute
```

## System Commands (Darwin)

- `git`: Version control
- `ls`: List files
- `cd`: Change directory
- `grep`: Search text
- `find`: Find files
- `cat`: Display file content
- `less`: Page through content
- `tail`: Show end of file
- `head`: Show beginning of file
