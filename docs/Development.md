# Vearch Development

This guide collects build, test, debug, and CI commands for developers working on Vearch.

## Build

### Prerequisites

- Go 1.22+
- CMake 3.17+
- GCC/G++ with C++17 support
- DiskANN library (`libdiskann.so`)
- Intel oneAPI MKL for DiskANN optimized builds
- Boost 1.78, auto-downloaded by the build script

### Commands

```bash
# Full build: C++ Gamma engine + Go binary
make all
make all j=8

# Build Go binary only and skip engine rebuild
cd build && ./build.sh -g OFF

# Debug build
cd build && ./build.sh -d

# Build CLI tools only
make tools

# Show all build options
cd build && ./build.sh -h
```

### build.sh Options

| Option | Purpose |
|---|---|
| `-n <num>` | Compile thread count |
| `-g ON\|OFF` | Build Gamma engine; default is `ON` |
| `-t` | Build engine tests |
| `-d` | Debug build |
| `-o generic\|avx2\|avx512` | SIMD optimization level; default is `avx512` |

### Build Output

- `build/bin/vearch` — main binary
- `build/lib/` — C++ shared libraries

## Run

```bash
# Standalone: all roles in one process for local development
./build/bin/vearch -conf=config/config.toml all

# Individual roles
./build/bin/vearch -conf=config/config.toml master
./build/bin/vearch -conf=config/config.toml ps
./build/bin/vearch -conf=config/config.toml router

# Multi-master: specify identity
./build/bin/vearch -conf=config/config_cluster.toml -master=m1 master
```

## Docker

```bash
# Standalone
cd cloud && docker compose --profile standalone up

# Cluster: 3 masters, 2 routers, 3 PS
cd cloud && docker compose --profile cluster up
```

## Tests

Integration tests require a running Vearch instance in standalone mode on localhost.

```bash
# Full test suite
make test

# Integration test categories
cd test
pytest test_vearch.py -x --log-cli-level=INFO
pytest test_document_* -k "not test_vearch_document_upsert_benchmark" -x --log-cli-level=INFO
pytest test_module_* -x --log-cli-level=INFO

# Single integration test file
cd test && pytest test_document_search.py -x --log-cli-level=INFO

# Go unit tests
go test ./internal/entity/... -v

# Go SDK tests
cd sdk/go/test && go test -v

# Engine unit tests
cd build && ./build.sh -t
cd build/gamma_build && ctest
```

## Debug Endpoints

| Port | Purpose |
|---|---|
| `:6060` | PS pprof |
| `:6061` | Router pprof |
| `:6062` | Master pprof |
| `:8818` | PS / Master Prometheus metrics |

Access pprof Web UI through `localhost:606x/debug/pprof/`.

## Debugging Slow PS Writes

1. Check Prometheus metrics in `internal/pkg/metrics/`.
2. Capture PS pprof at `localhost:6060/debug/pprof/profile`.
3. Inspect raft state in `internal/ps/storage/raftstore/store_raft_job.go`.
4. Inspect engine writes in `internal/ps/engine/gammacb/writer.go`.

## CI

GitHub Actions runs on push and pull requests to `master`.

- `CI.yml` — main build, pytest suite, and SDK tests for amd64 and arm64
- `CI_cluster.yml`, `CI_document.yml`, `CI_index.yml` — focused test suites
- `docker-image.yml` — Docker image builds
