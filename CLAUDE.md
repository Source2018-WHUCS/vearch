# CLAUDE.md

## Project Overview

Vearch is a distributed vector search system for AI applications. It provides scalable, real-time vector similarity search with support for multiple index types (HNSW, IVF-Flat, IVF-PQ, DiskANN, etc.).

**Version:** 3.5.9  
**Language:** Go 1.22+ (control plane) + C++ (vector engine)  
**Module:** `github.com/vearch/vearch/v3`

## Architecture

Vearch has three node roles, all built into a single binary (`vearch`):

- **Master** — cluster coordination via embedded etcd, metadata management (port 8817)
- **Router** — HTTP API gateway, query routing, result merging (port 9001)
- **PS (Partition Server)** — stores data partitions, runs vector engine, Raft replication (RPC port 8081)

Entry point: `cmd/vearch/startup.go`  
Core packages: `internal/master/`, `internal/router/`, `internal/ps/`  
Vector engine (C++): `internal/engine/` (built via CMake, exposed through `internal/engine/c_api/`)

## Build

### Prerequisites

- Go 1.22+
- CMake 3.17+
- GCC/G++ with C++17 support
- DiskANN library (libdiskann.so)
- Intel oneAPI MKL (for DiskANN optimized builds)
- Boost 1.78 (auto-downloaded by build script)

### Build Commands

```bash
# Full build (engine + Go binary), uses 4 threads by default
make

# Build with N parallel threads
make j=8

# Build Go binary only (skip engine rebuild)
cd build && ./build.sh -g OFF

# Debug build
cd build && ./build.sh -d

# Build tools
make tools
```

Output: `build/bin/vearch`

### Build Options (build/build.sh)

- `-n <num>` — compile thread count
- `-g ON|OFF` — build gamma engine (default ON)
- `-t` — build engine tests
- `-d` — debug build
- `-o generic|avx2|avx512` — SIMD optimization level (default avx512)

## Running

```bash
# Standalone (all roles in one process)
./build/bin/vearch -conf config/config.toml all

# Individual roles
./build/bin/vearch -conf config/config.toml master
./build/bin/vearch -conf config/config.toml router
./build/bin/vearch -conf config/config.toml ps
```

### Docker

```bash
# Standalone
cd cloud && docker compose --profile standalone up

# Cluster (3 masters, 2 routers, 3 PS)
cd cloud && docker compose --profile cluster up
```

## Testing

### Integration Tests (Python, pytest)

Tests require a running Vearch instance (standalone mode on localhost).

```bash
# Full test suite
make test

# Individual test categories
cd test
pytest test_vearch.py -x --log-cli-level=INFO
pytest test_document_* -k "not test_vearch_document_upsert_benchmark" -x --log-cli-level=INFO
pytest test_module_* -x --log-cli-level=INFO
```

### Go SDK Tests

```bash
cd sdk/go/test && go test -v
```

### Engine Unit Tests (C++)

```bash
cd build && ./build.sh -t   # builds with test enabled
cd build/gamma_build && ctest
```

## Project Layout

```
cmd/vearch/          Single binary entry point
internal/
  master/            Cluster management, metadata, etcd
  router/            HTTP API (gin), request routing
  ps/                Partition server, raft, document handling
  engine/            C++ vector engine (CMake project)
    c_api/           CGo bridge
    index/           Vector index implementations
    storage/         On-disk storage
  client/            Internal RPC client
  config/            Configuration parsing
  entity/            Shared data structures
  pkg/               Utilities (logging, metrics, etc.)
  proto/             Protobuf definitions
api/openapi/         OpenAPI specification
sdk/                 Client SDKs (Go, Python, Java, Rust)
test/                Integration tests (pytest)
config/              Example configuration files
cloud/               Docker build and compose files
scripts/benchmarks/  Benchmark scripts
```

## Configuration

Config format: TOML (`config/config.toml` for standalone, `config/config_cluster.toml` for cluster).

Key sections: `[global]`, `[etcd]`, `[[masters]]`, `[router]`, `[ps]`

## Code Conventions

- Apache 2.0 license header on all source files
- Go standard formatting (`gofmt`)
- HTTP framework: gin-gonic/gin
- RPC framework: rpcx (smallnest/rpcx)
- Serialization: flatbuffers, msgpack, sonic (JSON)
- Logging: custom logger at `internal/pkg/log`
- CGo tags: build with `-tags="vector"` to link the C++ engine

## CI

GitHub Actions runs on push/PR to master. Workflows:
- `CI.yml` — main build + pytest suite + SDK tests (amd64 + arm64)
- `CI_cluster.yml`, `CI_document.yml`, `CI_index.yml` — focused test suites
- `docker-image.yml` — Docker image builds
