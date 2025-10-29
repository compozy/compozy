# Edge Deployment Example

Optimized configuration for resource-constrained environments (ARM64/x86_64).

## Features
- Minimal agent workflow
- Small runtime surface (native tools off)
- Example `Dockerfile.edge` for tiny image

## Build (Docker)
```bash
cd examples/standalone/edge-deployment
docker build -f Dockerfile.edge -t compozy-edge:local ../../..
```

## Run (binary only)
```bash
COMPOZY_DEBUG=false ./compozy-edge-binary --config.from=env
```

