# CLAUDE.md — Vearch Project Navigation

> Vearch is a cloud-native distributed vector database. Control plane is written in Go; the vector index engine (Gamma) is implemented in C++.
>
> **Purpose of this document**: help Claude quickly locate modules, understand data flow, and respect project conventions when reading or modifying Vearch source code.

---

## 1. Project Quick Reference

| Item | Value |
|---|---|
| Version | 3.5.9 |
| Language | Go 1.22+ (control plane) + C++ (vector engine, in `internal/engine/`) |
| Module path | `github.com/vearch/vearch/v3` |
| Build entry | `cmd/vearch/startup.go` |
| Sample config | `config/config.toml` (standalone), `config/config_cluster.toml` (cluster) |
| Integration tests | `test/` (Python pytest) |
| Deployment | Single binary, multi-role; launched by tag: `master` / `ps` / `router` / `all` |
| Metadata store | etcd (embedded inside master process by default; or self-managed via config) |
| Consensus protocol | Raft (based on `cubefs/depends/tiglabs/raft`) |
| Default ports | master HTTP `8817` / etcd `2378–2390` ｜ router HTTP `9001` ｜ ps RPC `8081` / raft `8898/8899` |
| Pprof ports | master `6062` ｜ router `6061` ｜ ps `6060` |

---

## 2. Three-Tier Architecture

```
                    ┌─────────────┐
                    │   Client    │  (HTTP / gRPC / SDK)
                    └──────┬──────┘
                           ▼
                    ┌─────────────┐
                    │   Router    │  Stateless routing layer
                    │ (gin HTTP)  │  - Metadata cache
                    └──┬───┬──┬───┘  - Partition routing (murmur3 hash)
                       │   │  │     - Replica routing (5 strategies)
            ┌──────────┘   │  └──────────┐
            ▼              ▼             ▼
        ┌────────┐    ┌────────┐    ┌────────┐
        │  PS    │    │  PS    │    │  PS    │  Partition Server
        │ raft + │    │ raft + │    │ raft + │  - One raft group per partition
        │ gamma  │    │ gamma  │    │ gamma  │  - Gamma C++ index engine
        └────┬───┘    └────┬───┘    └────┬───┘
             │             │              │
             └─────────────┼──────────────┘
                           ▼
                    ┌─────────────┐
                    │   Master    │  Cluster management (embedded etcd)
                    │ (gin HTTP)  │  - DB / Space / Partition metadata
                    │  + etcd     │  - PS registration / heartbeat / recovery
                    └─────────────┘
```

**Key facts**:
- Master embeds etcd (`go.etcd.io/etcd/server/v3/embed`); no need to deploy etcd separately unless `self_manage_etcd = true`.
- Each **partition is an independent raft group**; replicas synchronize through raft.
- Router is fully stateless — failover loses no data.
- The PS index engine is Gamma (C++), called via cgo bindings.

---

## 3. Directory Layout & Responsibilities

```
vearch/
├── cmd/vearch/                 # Main entry (startup.go: main + role bootstrap)
├── internal/
│   ├── master/                 (★) Cluster control plane
│   │   ├── server.go           - Embedded etcd + Gin HTTP server startup
│   │   ├── cluster_api.go      - Cluster admin HTTP routes (User/DB/Space/PS CRUD)
│   │   ├── cluster_service.go  - Aggregator entry for all sub-services
│   │   ├── monitor_service.go  - Prometheus metrics
│   │   ├── schedule_job.go     - Background cron (orphan cleanup / etcd watch)
│   │   ├── services/           - Sub-services (one per entity)
│   │   │   ├── space_service.go        (★) Space creation + PS selection (core scheduling)
│   │   │   ├── partition_service.go    - Partition metadata
│   │   │   ├── server_service.go       - PS registration / failure recovery
│   │   │   ├── member_service.go       (★) Raft member changes (add/remove node)
│   │   │   ├── db_service.go           - Database CRUD
│   │   │   ├── alias_service.go        - Aliases
│   │   │   ├── user_service.go         - User management (RBAC)
│   │   │   ├── role_service.go         - Roles / permissions
│   │   │   ├── config_service.go       - Space-level config
│   │   │   └── backup_service.go       - Backup / restore
│   │   └── store/              - etcd client wrapper
│   │       ├── store.go                - Interface
│   │       ├── etcdstore.go            - etcd implementation
│   │       └── distlock.go             - Distributed lock
│   │
│   ├── ps/                     (★) Partition Server (data node)
│   │   ├── server.go           - PS RPC server + master registration
│   │   ├── handler_admin.go    - Admin RPC (create/delete partition, replica config)
│   │   ├── handler_document.go - Document RPC (write / query / search)
│   │   ├── partition_service.go- Local partition lifecycle
│   │   ├── schedule_job.go     - Heartbeat + resource monitoring
│   │   ├── psutil/             - PS utilities
│   │   ├── backup/             - Backup implementation
│   │   ├── storage/            (★) Storage layer
│   │   │   ├── storebase.go            - Common base
│   │   │   └── raftstore/              - Raft storage implementation
│   │   │       ├── store.go            - Entry
│   │   │       ├── raft_state_machine.go - Raft state machine (apply log)
│   │   │       ├── store_writer.go     - Write path (Apply → engine write)
│   │   │       ├── store_read.go       - Read path (engine direct)
│   │   │       ├── store_raft_job.go   - Raft background tasks
│   │   │       └── store_raft_snapshot.go - Snapshots
│   │   └── engine/             (★) Index engine abstraction
│   │       ├── engine.go               - Reader / Writer interfaces
│   │       ├── gammacb/                - Gamma C++ engine Go bindings (default impl)
│   │       │   ├── gamma.go            - Engine entry
│   │       │   ├── reader.go           - Query / search
│   │       │   ├── writer.go           - Document writes
│   │       │   └── snapshot.go         - Snapshot
│   │       ├── mapping/                - Field type mapping
│   │       └── sortorder/              - Sort utilities
│   │
│   ├── router/                 (★) Router (routing layer)
│   │   ├── server.go           - Startup + master registration
│   │   ├── schedule_job.go     - Heartbeat keepalive (10s etcd lease)
│   │   └── document/           - HTTP/gRPC document API
│   │       ├── doc_http.go             - Gin HTTP routes + middleware
│   │       ├── doc_service.go          - Business logic (calls client.RouterRequest)
│   │       ├── doc_rpc.go               - gRPC service
│   │       ├── doc_query.go            - Query DSL parsing (1500+ lines)
│   │       ├── doc_parse.go            - Request parsing
│   │       ├── doc_resp.go             - Response packaging
│   │       └── gctuner/                - GC adaptive tuning
│   │
│   ├── client/                 (★) Cross-component client
│   │   ├── client.go           (★) RouterRequest routing core (1933 lines)
│   │   │                         - PartitionDocs: hash partition by primary key
│   │   │                         - SearchByPartitions: broadcast to all partitions
│   │   │                         - GetNodeIdsByClientType: 5 replica selection strategies
│   │   ├── master.go           - Master HTTP client
│   │   ├── master_cache.go     - Metadata cache (go-cache)
│   │   ├── ps.go               - PS RPC client + faulty node management
│   │   └── ps_admin_service.go - PS admin operations client
│   │
│   ├── entity/                 (★) Data model (mapped to etcd keys)
│   │   ├── meta.go             - etcd key-space constants (PrefixSpace etc.)
│   │   ├── space.go            - Space (table) + PartitionId mapping logic
│   │   ├── partition.go        - Partition (shard)
│   │   ├── server.go           - Server (PS node)
│   │   ├── db.go               - Database
│   │   ├── alias.go / user.go / config.go
│   │   ├── raft.go             - Raft data structures
│   │   ├── request/            - Request structs (JSON unmarshal)
│   │   ├── response/           - Response structs
│   │   └── errors/             - Error definitions
│   │
│   ├── engine/                 (★) C++ Gamma engine (independent project)
│   │   ├── CMakeLists.txt      - C++ build
│   │   ├── c_api/              - CGo bridge
│   │   ├── index/              - Vector index (HNSW/IVF/PQ/RaBitQ etc.)
│   │   ├── vector/             - Vector management
│   │   ├── search/             - Search algorithms
│   │   ├── storage/ memory/ io/- Storage & IO
│   │   ├── sdk/go/gamma/       - Go bindings (cgo)
│   │   └── benchs/             - Benchmarks
│   │
│   ├── proto/vearchpb/         - Protobuf definitions (main RPC types)
│   │   ├── data_model.pb.go    - Document / Space / Partition etc.
│   │   ├── errors.pb.go        - Error code enum
│   │   ├── raftcmd.pb.go       - Raft commands
│   │   └── vearch_err.go       - Error helpers
│   │
│   ├── config/                 - Config loading (toml)
│   ├── monitor/                - Metric exporters
│   ├── debugutil/pprofui/      - Pprof visualizer
│   └── pkg/                    - Common utilities
│       ├── log/                - Logging (vearchlog)
│       ├── atomic/             - Atomic operations
│       ├── metrics/            - Metric collection (mserver/sysstat)
│       ├── server/rpc/         - rpcx wrapper
│       ├── netutil/ fileutil/ vjson/ cbbytes/ number/
│       └── routine/ signals/ runtime/
│
├── cmd/vearch/startup.go       - main entry
├── config/config.toml          - Sample config
├── api/openapi/                - OpenAPI spec
├── sdk/                        - Multi-language SDKs (go/python/java/rust/integrations)
├── test/                       - Python integration tests (pytest)
├── docs/                       - Design documents (Architecture.md / Quickstart.md / etc.)
├── build/build.sh              - C++ + Go build script
├── cloud/                      - Cloud deployment (k8s / docker compose)
├── tools/                      - CLI tools
├── examples/                   - SDK usage examples
└── scripts/benchmarks/         - Benchmark scripts
```

---

## 4. Core Data Flows

### 4.1 Write Path (Upsert)

```
Client → POST /document/upsert
   ↓
Router (doc_http.go → doc_service.go → client.RouterRequest)
   ↓
[1] Look up Space in metadata cache → master_cache.go
[2] PartitionDocs(): partitionID = murmur3(PKey) → binary-search partition slot range
[3] Group docs by partitionID → sendMap
[4] Find partition leader: GetNodeIdsByClientType(Leader)
   ↓
PS RPC (handler_document.go::Bulk)
   ↓
[5] raft Apply (store_writer.go)
[6] After raft commit, write to engine (gamma writer)
[7] Replicate to followers
   ↓
Response → Router → Client
```

### 4.2 Search Path (Vector Search)

```
Client → POST /document/search
   ↓
Router (doc_service.go::search)
   ↓
[1] Look up Space + Partitions in metadata cache
[2] SearchByPartitions(): broadcast to all partitions
[3] Per partition GetNodeIdsByClientType (default: Random)
[4] Concurrent RPCs
   ↓
PS RPC (handler_document.go::Search) — does NOT go through raft
   ↓
[5] gamma reader Search (HNSW/IVF/...)
[6] Return local topK
   ↓
[7] Router merges results from all partitions (mergeSortedArrays)
   ↓
Response → Client
```

### 4.3 Cluster Scheduling (Space Creation / PS Selection)

```
POST /space/create
   ↓
Master cluster_api.go → space_service.CreateSpace
   ↓
[1] Validate schema + generate partition IDs
[2] filterAndSortServer: sort PS by partition count ascending
[3] selectServersForPartition: pick ReplicaNum servers per partition
    (with anti-affinity zone/rack/host constraints)
[4] Notify each PS to create the partition
[5] waitForPartitionsReady (poll until replicas ready)
[6] Persist Space metadata to etcd
```

---

## 5. Key Conventions

### 5.1 etcd Key Space (`internal/entity/meta.go`)

| Prefix | Purpose |
|---|---|
| `/server/<id>` | PS node registration (with TTL) |
| `/router/<name>/<addr>` | Router heartbeat (10s TTL) |
| `/db/id/<id>` | DB ID → metadata |
| `/db/name/<name>` | DB Name → ID |
| `/space/<dbid>/<spaceid>` | Space metadata |
| `/partition/<id>` | Partition metadata |
| `/lock/...` | Distributed lock (distlock.go) |
| `/cluster/clean_job` | Background STM timestamp gating |

### 5.2 Service Pattern

All `services/*` in master are `XxxService` structs holding `client *client.Client`:

```go
type SpaceService struct { client *client.Client }
func NewSpaceService(c *client.Client) *SpaceService { ... }
func (s *SpaceService) CreateSpace(ctx ...) error { ... }
```

When modifying master business logic, **look first in `services/` for the matching service file**.

### 5.3 Routing Request Pattern (Builder Style)

`client.RouterRequest` is the core builder pattern on the routing side:

```go
request := client.NewRouterRequest(ctx, c)
request.SetMsgID(...).SetMethod(...).SetHead(...).SetSpace().SetDocs(...).PartitionDocs()
items := request.Execute()
```

**For routing-logic changes, edit `client/client.go`** — it is the 1933-line core file.

### 5.4 Raft Writes, Local Reads

- **All writes must go through raft**: `storage/raftstore/store_writer.go`
- **Reads go directly to engine**: `storage/raftstore/store_read.go`
- Read replica selection is controlled at Router (via `request.ClientType`), see `client.go:1353` `GetNodeIdsByClientType`

### 5.5 Error Codes

- All error definitions are in `internal/proto/vearchpb/errors.pb.go` (protobuf enum)
- Wrap business errors with `vearchpb.NewError(ErrorEnum_XXX, err)`
- HTTP layer maps codes to HTTP status in `cluster_api.go::handleError`

### 5.6 Protobuf

- Main types in `internal/proto/vearchpb/data_model.pb.go`
- Regenerate from `.proto` sources (if present in `internal/proto/vearchpb/`)
- **Do NOT hand-edit `.pb.go` files** — modify the `.proto` and regenerate

### 5.7 Code Style

- Apache 2.0 license header on every source file
- Go standard formatting (`gofmt`)
- HTTP framework: `gin-gonic/gin`
- RPC framework: `smallnest/rpcx`
- Serialization: flatbuffers, msgpack, sonic (JSON)
- Logger: custom logger at `internal/pkg/log`
- CGo build tag: `-tags="vector"` to link the C++ engine

---

## 6. Common Commands

### 6.1 Build

#### Prerequisites

- Go 1.22+
- CMake 3.17+
- GCC/G++ with C++17 support
- DiskANN library (`libdiskann.so`)
- Intel oneAPI MKL (for DiskANN optimized builds)
- Boost 1.78 (auto-downloaded by build script)

#### Commands

```bash
# Full build (C++ Gamma + Go), 4 threads by default
make all                           # equivalent to: cd build && ./build.sh -n 1
make all j=8                       # 8 parallel jobs

# Build Go binary only (skip engine rebuild)
cd build && ./build.sh -g OFF

# Debug build
cd build && ./build.sh -d

# Build CLI tools only (tools/)
make tools

# See all build options
cd build && ./build.sh -h
```

#### build.sh Options

- `-n <num>` — compile thread count
- `-g ON|OFF` — build gamma engine (default ON)
- `-t` — build engine tests
- `-d` — debug build
- `-o generic|avx2|avx512` — SIMD optimization level (default avx512)

#### Build Output

- `build/bin/vearch` — main binary
- `build/lib/` — C++ shared libraries

### 6.2 Run

```bash
# Standalone (all roles in one process, for local dev)
./build/bin/vearch -conf=config/config.toml all

# Individual roles
./build/bin/vearch -conf=config/config.toml master
./build/bin/vearch -conf=config/config.toml ps
./build/bin/vearch -conf=config/config.toml router

# Multi-master: specify identity
./build/bin/vearch -conf=config/config_cluster.toml -master=m1 master
```

#### Docker

```bash
# Standalone
cd cloud && docker compose --profile standalone up

# Cluster (3 masters, 2 routers, 3 PS)
cd cloud && docker compose --profile cluster up
```

### 6.3 Test

#### Integration Tests (Python pytest)

Tests require a running Vearch instance (standalone mode on localhost).

```bash
# Full test suite
make test

# Categories
cd test
pytest test_vearch.py -x --log-cli-level=INFO
pytest test_document_* -k "not test_vearch_document_upsert_benchmark" -x --log-cli-level=INFO
pytest test_module_* -x --log-cli-level=INFO

# Single test file
cd test && pytest test_document_search.py -x --log-cli-level=INFO
```

#### Go Unit Tests

```bash
go test ./internal/entity/... -v
```

#### Go SDK Tests

```bash
cd sdk/go/test && go test -v
```

#### Engine Unit Tests (C++)

```bash
cd build && ./build.sh -t   # build with tests enabled
cd build/gamma_build && ctest
```

### 6.4 Debug Endpoints

| Port (default) | Purpose |
|---|---|
| `:6060` | PS pprof |
| `:6061` | Router pprof |
| `:6062` | Master pprof |
| `:8818` | PS / Master Prometheus metrics |

Access pprof Web UI via `localhost:606x/debug/pprof/` (powered by `debugutil/pprofui`).

---

## 7. Modification Guidelines

### 7.1 Modifying Routing Strategy

- File: `internal/client/client.go`
- Five `ClientType` strategies: `Leader / NotLeader / Random / LeastConnection / NearestConnection`
- Round-robin counters are per-partition (`replicaRoundRobin sync.Map`)
- Faulty nodes have 30s TTL (`ps.go:138 initFaultylist`)

### 7.2 Modifying Master Scheduling

- File: `internal/master/services/space_service.go`
- `selectServersForPartition` (line 1246) — currently sorts PS by partition count ascending
- `filterAndSortServer` (line 458) — accumulates partition counts; **no data-volume dimension**
- Before changing, study `internal/master/services/member_service.go::ChangeMember` — the migration primitive already exists

### 7.3 Adding a New etcd Key

- Add the constant to `internal/entity/meta.go`
- Use `client.Master().Store` (interface in `master/store/store.go`)
- For cross-node coordination, use the distributed lock: `master/store/distlock.go`

### 7.4 Adding a New RPC Handler

- PS side: `internal/ps/handler_document.go` or `handler_admin.go`
- Router side: `internal/router/document/doc_*.go`
- Client side: `internal/client/ps.go` (add Handler constant) + `client.go` (add routing-request method)

### 7.5 Modifying the Gamma Engine (C++)

- Source: `internal/engine/`
- Build: through `build/build.sh`
- Go bindings: `internal/engine/sdk/go/gamma/` (cgo)
- Engine interface contract: `internal/ps/engine/engine.go::Reader/Writer`

### 7.6 Invariants You Must Never Break

1. **Writes always go through raft** — every data-mutation path must go via `storage/raftstore`, never call the engine directly.
2. **Replica count ≥ ReplicaNum** — for member changes, Add first, Remove second.
3. **Anti-affinity** — migration targets must preserve existing zone/rack isolation (`space_service.go:1269`).
4. **No cross-ResourceName** — migration cannot cross resource pools.
5. **etcd keys are not hand-modified** — every etcd modification goes through the service layer (preserves cache consistency).

---

## 8. Call-Chain Quick Reference

> To trace "which functions does action X go through", see this table.

| User action | Call chain |
|---|---|
| Create DB | `cluster_api.createDB` → `services.DBService.CreateDB` → `etcdstore.Create` |
| Create Space | `cluster_api.createSpace` → `services.SpaceService.CreateSpace` → `selectServersForPartition` → `client.CreatePartition` (per PS) |
| Write document | `doc_http.upsertHandler` → `doc_service.bulk` → `client.RouterRequest.UpsertByPartitions` → PS `handler_document.Bulk` → `raftstore.store_writer` → `gamma.Write` |
| Search documents | `doc_http.searchHandler` → `doc_service.search` → `client.RouterRequest.SearchByPartitions` → concurrent PS `handler_document.Search` → `gamma.Search` → merge |
| Get doc by ID | `doc_http.getHandler` → `doc_service.getDocs` → `client.RouterRequest.PartitionDocs` (murmur3) → PS `handler_document.GetDocs` |
| Replica change | `cluster_api.changeMember` → `services.MemberService.ChangeMember` → `proto.ConfAddNode/RemoveNode` → PS raft conf change |
| Failed PS recovery | `cluster_api.recoverFailServer` → `services.ServerService.RecoverFailServer` → `MemberService.ChangeMember` (add then remove) |

---

## 9. External Dependencies (Critical Points)

| Dependency | Purpose | Path |
|---|---|---|
| `go.etcd.io/etcd` | Embedded etcd | `master/server.go` |
| `cubefs/depends/tiglabs/raft` | Raft protocol | `ps/storage/raftstore` |
| `gin-gonic/gin` | HTTP framework | `master`, `router` |
| `smallnest/rpcx` | RPC framework | `ps`, `client/ps.go` |
| `spaolacci/murmur3` | Consistent hashing | `client/client.go:238` |
| `prometheus/client_golang` | Metric collection | `monitor/`, `pkg/metrics/` |
| `patrickmn/go-cache` | In-memory cache | `client/master_cache.go` |
| `BurntSushi/toml` | Config parsing | `config/` |
| `uber/jaeger-client-go` | Distributed tracing | `cmd/vearch/startup.go` |

---

## 10. Common Task Recipes (For Claude)

### Task: Locate "the logic that picks PS for a new Space"

→ `internal/master/services/space_service.go:1246` `selectServersForPartition`

### Task: Understand "how the router selects a replica"

→ `internal/client/client.go:1353` `GetNodeIdsByClientType`

### Task: Add a new HTTP API

1. Master API: register the route in `internal/master/cluster_api.go` + add the business method in `services/xxx_service.go`
2. Router API: register the route in `internal/router/document/doc_http.go` + add the business method in `doc_service.go`

### Task: Debug slow PS writes

1. Check Prometheus metrics in `pkg/metrics/`
2. Capture PS pprof: `localhost:6060/debug/pprof/profile`
3. Inspect raft state: `storage/raftstore/store_raft_job.go`
4. Inspect engine writes: `engine/gammacb/writer.go`

### Task: Change replica count (scale up/down)

→ `internal/master/services/member_service.go::ChangeReplica` (only ±1 at a time)

### Task: Analyze traffic distribution across partitions

→ Prometheus query: `vearch_data_node_request_count` grouped by `partition_id`

---

## 11. File-Size Quick Reference (Reading Order)

| File | Lines | Importance |
|---|---|---|
| `internal/client/client.go` | 1933 | ★★★ Routing core |
| `internal/router/document/doc_query.go` | 1584 | ★★ Query DSL parsing |
| `internal/client/master_cache.go` | 1397 | ★★ Metadata cache |
| `internal/master/services/space_service.go` | 1372 | ★★★ Cluster scheduling core |
| `internal/router/document/doc_http.go` | 997 | ★★ HTTP entry |
| `internal/client/master.go` | 922 | ★★ Master client |

---

## 12. Pitfalls

1. **Do NOT read PS data directly from a master service** — always go through `client.PS()` over RPC.
2. **Do NOT assume partition data volume is balanced** — hash skew or range partitions cause skew.
3. **Do NOT ignore the `RaftConsistent` flag** — when enabled, only ReplicasOK replicas are read; throughput drops but reads are fresh.
4. **When modifying etcd keys**: master's internal cache (`master_cache.go`) uses `watch`, but the router cache uses TTL — there will be a brief inconsistency window.
5. **`ChangeMember` is one step at a time** — Add first, Remove second; replica count must never drop below ReplicaNum.
6. **C++ engine changes require `make all`** — cgo linking must be re-run.

---

## 13. CI

GitHub Actions runs on push/PR to `master`. Workflows:

- `CI.yml` — main build + pytest suite + SDK tests (amd64 + arm64)
- `CI_cluster.yml`, `CI_document.yml`, `CI_index.yml` — focused test suites
- `docker-image.yml` — Docker image builds

---

**Document version**: generated from the Vearch v3 main-branch source tree.
