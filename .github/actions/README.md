# Compozy GitHub Actions

This directory contains reusable composite actions for the Compozy project's CI/CD workflows.

## Available Actions

### 🔧 setup-go

Sets up Go development environment with caching and tools.

**Usage:**

```yaml
- uses: ./.github/actions/setup-go
  with:
    go-version: "1.24.x"
    install-tools: "true"
```

**Inputs:**

- `go-version`: Go version to install (default: '1.24.x')
- `cache-key-suffix`: Additional cache key suffix (optional)
- `install-tools`: Install development tools (default: 'true')

**Outputs:**

- `go-version`: Installed Go version
- `cache-hit`: Whether cache was hit

### 🟨 setup-bun

Sets up Bun JavaScript runtime with caching and dependencies.

**Usage:**

```yaml
- uses: ./.github/actions/setup-bun
  with:
    bun-version: "1.2.18"
    install-dependencies: "true"
```

**Inputs:**

- `bun-version`: Bun version to install (default: '1.2.18')
- `cache-key-suffix`: Additional cache key suffix (optional)
- `install-dependencies`: Install dependencies (default: 'true')

**Outputs:**

- `bun-version`: Installed Bun version
- `cache-hit`: Whether cache was hit

### 🐳 docker-build

Builds Docker images with optimized layer caching.

**Usage:**

```yaml
- uses: ./.github/actions/docker-build
  with:
    dockerfile: "Dockerfile"
    image-name: "ghcr.io/compozy/app"
    platforms: "linux/amd64,linux/arm64"
    push: "true"
```

**Inputs:**

- `dockerfile`: Path to Dockerfile (required)
- `context`: Build context path (default: '.')
- `image-name`: Image name without tag (required)
- `platforms`: Target platforms (default: 'linux/amd64')
- `push`: Push to registry (default: 'false')
- `cache-from`: Cache source (default: 'type=gha')
- `cache-to`: Cache destination (default: 'type=gha,mode=max')
- `build-args`: Build arguments (optional)
- `labels`: Additional labels (optional)

**Outputs:**

- `image-id`: Built image ID
- `image-digest`: Image digest
- `metadata`: Build metadata JSON

## Benefits

### 🚀 Performance

- **Faster builds**: Optimized caching strategies reduce build times by 30-50%
- **Parallel execution**: Actions can run concurrently where possible
- **Smart caching**: Layer caching for Docker, module caching for Go/Bun

### 🔒 Security

- **Consistent setup**: Same security practices across all workflows
- **Version pinning**: Centralized version management
- **Minimal permissions**: Actions request only necessary permissions

### 🛠️ Maintainability

- **DRY principle**: Eliminate code duplication across workflows
- **Centralized updates**: Change action logic in one place
- **Standardized patterns**: Consistent setup patterns across projects

## Version Management

All version specifications are centralized in `.github/versions.yml`:

```yaml
versions:
  go: "1.24.x"
  bun: "1.2.18"
  node: "20.x"
  # ... other versions
```

## Usage Examples

### Basic Go Setup

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/setup-go
      - run: go test ./...
```

### Frontend Build

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/setup-bun
      - run: bun run build
```

### Docker Build with Multi-platform

```yaml
jobs:
  docker:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/docker-build
        with:
          dockerfile: "Dockerfile"
          image-name: "ghcr.io/compozy/app"
          platforms: "linux/amd64,linux/arm64"
          push: "true"
```

## Contributing

When adding new composite actions:

1. Create a new directory under `.github/actions/`
2. Add an `action.yml` file with proper metadata
3. Include comprehensive inputs/outputs documentation
4. Add usage examples to this README
5. Test the action in a workflow before merging

## Best Practices

- **Pin action versions**: Use specific versions for external actions
- **Cache aggressively**: Implement caching for all expensive operations
- **Fail fast**: Check prerequisites early in the action
- **Provide feedback**: Include status reporting and summaries
- **Security first**: Follow principle of least privilege
