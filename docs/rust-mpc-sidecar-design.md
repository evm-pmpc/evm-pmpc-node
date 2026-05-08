# Rust MPC Sidecar Design

## Scope

This document is specifically about how to design the Rust sidecar that will perform the private MPC work for this repository.

It is intentionally narrow:

- what the Rust workspace should look like
- which crates and files to create
- what each module owns
- how the sidecar talks to the Go node
- how secret state is stored
- how MPC sessions are executed inside the Rust process

It is not a full redesign of the Go node.

## Current Go Boundary

The current Go worker:

- creates the libp2p host and identity: [cmd/node/main.go](/Users/jnyandeepsingh/Programming/Github/evm-pmpc-node/cmd/node/main.go:83)
- bootstraps peer discovery and rendezvous: [cmd/node/main.go](/Users/jnyandeepsingh/Programming/Github/evm-pmpc-node/cmd/node/main.go:98)
- joins one generic pubsub topic and logs messages: [cmd/node/main.go](/Users/jnyandeepsingh/Programming/Github/evm-pmpc-node/cmd/node/main.go:126)

That means the Rust sidecar should assume:

- Go owns libp2p networking
- Go owns peer discovery and session coordination
- Rust receives already-routed MPC frames from Go
- Rust never opens public network ports in phase 1

## Sidecar Responsibilities

The Rust sidecar should own all secret MPC state and all cryptographic protocol execution.

Specifically:

- auxiliary info generation
- distributed key generation
- presignature generation
- threshold ECDSA signing
- key share storage
- aux info storage
- presignature storage and single-use consumption
- cryptographic validation of session inputs
- producing Ethereum-compatible signatures

The sidecar should not own:

- peer discovery
- peer selection
- threshold policy
- digest construction from transactions
- public HTTP APIs

## Backend Choice

Use `cggmp21` as the first MPC backend.

Why:

- threshold ECDSA over secp256k1
- transport-agnostic API
- supports aux info, keygen, presign, and sign flows
- fits a sidecar that gets messages from Go over a local RPC stream

Do not bind the whole sidecar directly to `cggmp21` types. Put a thin backend trait in front of it so the rest of the sidecar is stable if the backend changes.

## Workspace Layout

Create a Rust workspace at `sidecar-rs/`.

```text
sidecar-rs/
├── Cargo.toml
├── rust-toolchain.toml
├── .cargo/
│   └── config.toml
├── proto/
│   └── mpc_sidecar.proto
├── crates/
│   ├── sidecar-bin/
│   │   ├── Cargo.toml
│   │   └── src/
│   │       └── main.rs
│   ├── sidecar-app/
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── app.rs
│   │       ├── config.rs
│   │       ├── server.rs
│   │       ├── shutdown.rs
│   │       └── health.rs
│   ├── sidecar-api/
│   │   ├── Cargo.toml
│   │   ├── build.rs
│   │   └── src/
│   │       └── lib.rs
│   ├── sidecar-types/
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── ids.rs
│   │       ├── session.rs
│   │       ├── key.rs
│   │       ├── presign.rs
│   │       └── error.rs
│   ├── sidecar-store/
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── fs.rs
│   │       ├── crypto.rs
│   │       ├── models.rs
│   │       ├── atomic.rs
│   │       └── lock.rs
│   ├── sidecar-backend/
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── traits.rs
│   │       ├── cggmp21/
│   │       │   ├── mod.rs
│   │       │   ├── aux_info.rs
│   │       │   ├── keygen.rs
│   │       │   ├── presign.rs
│   │       │   ├── sign.rs
│   │       │   ├── mapping.rs
│   │       │   └── codec.rs
│   │       └── testkit.rs
│   ├── sidecar-session/
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── manager.rs
│   │       ├── runner.rs
│   │       ├── inbox.rs
│   │       ├── outbox.rs
│   │       ├── registry.rs
│   │       ├── context.rs
│   │       └── events.rs
│   ├── sidecar-keystore/
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── keys.rs
│   │       ├── aux.rs
│   │       ├── presigs.rs
│   │       └── metadata.rs
│   └── sidecar-observability/
│       ├── Cargo.toml
│       └── src/
│           ├── lib.rs
│           ├── logging.rs
│           ├── metrics.rs
│           └── tracing.rs
└── tests/
    ├── integration_keygen.rs
    ├── integration_sign.rs
    ├── integration_restart.rs
    └── fixtures/
```

## Why This Split

This workspace split keeps responsibilities clean.

- `sidecar-bin`: process entrypoint only
- `sidecar-app`: wiring and lifecycle
- `sidecar-api`: protobuf and gRPC service definitions
- `sidecar-types`: internal domain types shared across crates
- `sidecar-store`: generic filesystem and encryption primitives
- `sidecar-keystore`: MPC-specific secret persistence
- `sidecar-backend`: crypto backend abstraction and `cggmp21` adapter
- `sidecar-session`: active session execution engine
- `sidecar-observability`: tracing, logs, metrics

Do not collapse everything into one crate. The session engine, keystore, and backend adapter need separate test surfaces.

## File-Level Design

### `crates/sidecar-bin/src/main.rs`

Only:

- parse config/env
- initialize tracing/logging
- create `App`
- run until shutdown

Keep `main.rs` thin. It should not contain MPC logic.

### `crates/sidecar-app/src/app.rs`

Owns top-level assembly:

- load config
- open keystore
- create backend instance
- create session manager
- start gRPC server on UDS
- expose health state

Suggested shape:

```rust
pub struct App {
    config: SidecarConfig,
    keystore: Arc<Keystore>,
    backend: Arc<dyn MpcBackend>,
    sessions: Arc<SessionManager>,
    health: Arc<HealthState>,
}
```

### `crates/sidecar-app/src/server.rs`

Implements the gRPC service exposed to Go.

Responsibilities:

- authenticate local caller if needed
- validate envelope shape
- map RPC stream events into `SessionManager`
- stream outbound network frames and progress updates back to Go

This file should contain protocol-server glue, not cryptography.

### `crates/sidecar-api`

This crate is generated API surface only.

- `proto/mpc_sidecar.proto`: source of truth
- `build.rs`: generate tonic/prost types
- `src/lib.rs`: exports generated modules

Keep generated API types out of the backend and keystore crates. Translate them at the boundary.

### `crates/sidecar-types`

This crate holds internal types that should not depend on gRPC codegen.

Core files:

- `ids.rs`
  - `SessionId`
  - `ExecutionId`
  - `KeyId`
  - `PresignId`
- `session.rs`
  - `Operation`
  - `Participant`
  - `SessionConfig`
  - `InboundFrame`
  - `OutboundFrame`
- `key.rs`
  - `KeyMetadata`
  - `PublicKeyRecord`
  - `EthereumAddress`
- `presign.rs`
  - `PresignMetadata`
  - `PresignStatus`
- `error.rs`
  - top-level typed error enums

This crate should be `serde`-friendly and backend-agnostic.

### `crates/sidecar-store`

This is low-level storage machinery.

Files:

- `fs.rs`
  - directory creation
  - path normalization
  - secure permission checks
- `crypto.rs`
  - local envelope encryption
  - master-key load/unseal helpers
- `models.rs`
  - generic stored blob wrappers
- `atomic.rs`
  - temp-file write + fsync + rename
- `lock.rs`
  - process-level file locks to avoid dual writers

This crate should know nothing about MPC semantics.

### `crates/sidecar-keystore`

This is MPC-aware persistence.

Files:

- `keys.rs`
  - store/load key shares
  - store/load public key metadata
- `aux.rs`
  - store/load auxiliary info by signer group
- `presigs.rs`
  - reserve/consume presignatures
  - enforce single-use state transitions
- `metadata.rs`
  - helper records and indexes

Suggested public API:

```rust
pub trait Keystore {
    fn put_key_share(&self, key_id: &KeyId, share: StoredKeyShare) -> Result<()>;
    fn get_key_share(&self, key_id: &KeyId) -> Result<StoredKeyShare>;
    fn put_aux_info(&self, group_id: &str, aux: StoredAuxInfo) -> Result<()>;
    fn get_aux_info(&self, group_id: &str) -> Result<Option<StoredAuxInfo>>;
    fn reserve_presign(&self, key_id: &KeyId) -> Result<Option<ReservedPresign>>;
    fn commit_consumed_presign(&self, reservation: ReservedPresign) -> Result<()>;
    fn release_presign(&self, reservation: ReservedPresign) -> Result<()>;
}
```

The presign reservation API matters. Consumption must be durable and explicit.

### `crates/sidecar-backend`

This crate adapts MPC library details into a stable interface the rest of the sidecar can use.

`traits.rs` should define the main abstraction:

```rust
#[async_trait]
pub trait MpcBackend: Send + Sync {
    async fn run_aux_info(
        &self,
        ctx: BackendSessionContext,
        io: BackendSessionIo,
    ) -> Result<AuxInfoResult>;

    async fn run_keygen(
        &self,
        ctx: BackendSessionContext,
        io: BackendSessionIo,
    ) -> Result<KeygenResult>;

    async fn run_presign(
        &self,
        ctx: BackendSessionContext,
        io: BackendSessionIo,
    ) -> Result<PresignResult>;

    async fn run_sign(
        &self,
        ctx: BackendSessionContext,
        io: BackendSessionIo,
    ) -> Result<SignResult>;
}
```

`cggmp21/` files:

- `mapping.rs`
  - convert `Participant` list and indices into backend-specific party mappings
- `codec.rs`
  - encode/decode backend message blobs for transport through Go
- `aux_info.rs`
  - backend-specific aux flow
- `keygen.rs`
  - backend-specific keygen flow
- `presign.rs`
  - backend-specific presign flow
- `sign.rs`
  - backend-specific sign flow

Keep backend-specific wire encoding isolated here. Do not leak raw `cggmp21` message types into `sidecar-session`.

### `crates/sidecar-session`

This is the core runtime crate.

It should own:

- active session registry
- per-session async tasks
- inbound frame routing
- outbound frame emission
- progress and terminal state
- cancellation and timeouts

Files:

- `manager.rs`
  - public API for creating and driving sessions
- `runner.rs`
  - executes one session from start to finish
- `registry.rs`
  - tracks active sessions by `SessionId`
- `inbox.rs`
  - buffered inbound message handling
- `outbox.rs`
  - fanout of outbound frames to Go RPC stream
- `context.rs`
  - immutable execution inputs
- `events.rs`
  - `SessionEvent`, `ProgressEvent`, `TerminalEvent`

Suggested public API:

```rust
pub struct SessionManager { ... }

impl SessionManager {
    pub async fn start_session(&self, cfg: SessionConfig) -> Result<SessionHandle>;
    pub async fn push_inbound(&self, frame: InboundFrame) -> Result<()>;
    pub async fn cancel(&self, session_id: SessionId) -> Result<()>;
    pub async fn get_status(&self, session_id: SessionId) -> Result<SessionStatus>;
}
```

`runner.rs` is where operation dispatch happens:

- load key material if needed
- reserve presign if needed
- build backend context
- call the backend trait
- persist outputs atomically
- emit terminal result

### `crates/sidecar-observability`

Keep observability out of business logic.

Files:

- `logging.rs`
  - field conventions
- `metrics.rs`
  - counters, histograms, gauges
- `tracing.rs`
  - tracing subscriber setup

This keeps session and backend crates testable without logger setup.

## RPC Surface

Use gRPC over a Unix domain socket.

### Socket path

Recommended:

- `/var/run/evm-pmpc/sidecar.sock`

Permissions:

- socket dir created by Go or the entrypoint
- `0700` on directory
- `0600`-equivalent restricted access for socket owner

### Proto shape

Use one long-lived bidirectional stream for each active session:

- `RunSession(stream SessionEnvelope) returns (stream SessionEnvelope)`

Add unary RPCs:

- `Health`
- `ListKeys`
- `GetKey`
- `DeleteKey`

### `SessionEnvelope`

Make this a tagged union with explicit variants:

- `StartSession`
- `InboundFrame`
- `CancelSession`
- `Ack`
- `OutboundFrame`
- `Progress`
- `Completed`
- `Failed`

Do not rely on one flat message with many optional fields. Use `oneof`.

## Session Execution Model

Each MPC execution should map to one Rust session task.

Suggested runtime model:

- one `tokio` task per session
- one bounded `mpsc` inbox per session
- one bounded `mpsc` outbox per session
- one registry entry in `SessionManager`

This avoids global locks around protocol progress.

### Session start

When Go sends `StartSession`:

1. validate `session_id`, `execution_id`, participants, threshold, operation
2. ensure local participant exists and has one stable index
3. ensure required key material exists for sign/presign
4. spawn a runner task
5. return immediate `Progress { state: Started }`

### Inbound frames

When Go sends an `InboundFrame`:

1. route by `session_id`
2. validate sender index is known for this session
3. push into the session inbox
4. reject if session is closed or queue is full

### Outbound frames

When the backend emits a protocol message:

1. encode it into backend-opaque bytes
2. wrap it in `OutboundFrame`
3. include `session_id`, recipient index, sender index, sequence number
4. send back to Go over the RPC stream

### Completion

On success:

- persist result before returning `Completed`
- drop session registry entry

On failure:

- mark reservation cleanup if needed
- emit structured `Failed`
- drop session registry entry

## On-Disk Layout

The sidecar data directory should be independent from Go metadata.

Recommended layout:

```text
/data/mpc/
├── master.key
├── keys/
│   ├── key-001/
│   │   ├── share.enc
│   │   ├── public.json
│   │   └── participants.json
├── aux/
│   ├── group-<hash>.enc
├── presigs/
│   ├── key-001/
│   │   ├── available/
│   │   ├── reserved/
│   │   └── consumed/
├── sessions/
│   ├── active/
│   └── archive/
└── tmp/
```

### File meanings

- `share.enc`
  - encrypted serialized local key share
- `public.json`
  - public key, ethereum address, threshold, created timestamp
- `participants.json`
  - signer set and deterministic index mapping
- `group-<hash>.enc`
  - encrypted auxiliary info blob for exactly one signer group
- `presigs/...`
  - presign material and state transitions

Do not store active-session secret material only in memory if losing it would create ambiguous presign consumption state. Persist reservation state.

## Data Models

### Public metadata

`public.json` should contain:

```json
{
  "key_id": "key-001",
  "curve": "secp256k1",
  "scheme": "ecdsa",
  "threshold": 2,
  "participants": [
    {"peer_id": "12D3...", "index": 1},
    {"peer_id": "12D3...", "index": 2},
    {"peer_id": "12D3...", "index": 3}
  ],
  "public_key_hex": "...",
  "ethereum_address": "0x...",
  "created_at": "2026-05-07T12:00:00Z"
}
```

### Secret blob wrapper

Encrypted blobs should use a stable envelope:

```json
{
  "version": 1,
  "algorithm": "xchacha20poly1305",
  "nonce_b64": "...",
  "ciphertext_b64": "..."
}
```

This makes future migrations manageable.

## Config Design

The sidecar should have its own config struct even if Go is the main parent process.

Suggested Rust config:

```rust
pub struct SidecarConfig {
    pub socket_path: PathBuf,
    pub data_dir: PathBuf,
    pub log_format: LogFormat,
    pub max_concurrent_sessions: usize,
    pub session_inbox_capacity: usize,
    pub session_timeout: Duration,
    pub backend: BackendConfig,
    pub storage: StorageConfig,
    pub metrics: MetricsConfig,
}
```

`BackendConfig` should include:

- backend kind
- protocol-specific tuning knobs
- presign pool target

`StorageConfig` should include:

- master key file path
- allow plaintext dev mode flag, default false

## Error Model

Do not use stringly-typed errors across crate boundaries.

Recommended top-level error categories:

- `ConfigError`
- `RpcError`
- `ValidationError`
- `StorageError`
- `BackendError`
- `SessionError`
- `PresignReservationError`

`Completed` and `Failed` session responses should return structured machine-readable codes so Go can decide whether to retry, abort, or mark the peer unhealthy.

## Security Rules

The sidecar should enforce these rules itself, not trust Go blindly.

- reject unknown operations
- reject malformed participant lists
- reject duplicate participant indices
- reject thresholds outside `1..=n`
- reject sign requests whose key metadata does not match the session participants
- reject inbound frames from unknown sender indices
- never expose decrypted share bytes in logs or RPC responses
- never return a completed signature until presign consumption is durable

## Test Layout

Tests should mirror the workspace split.

### Unit tests

- `sidecar-store`
  - atomic write recovery
  - key file permission checks
  - encryption/decryption roundtrip
- `sidecar-keystore`
  - reserve/commit/release presign transitions
  - missing-key behavior
- `sidecar-session`
  - session registry correctness
  - timeout and cancellation
  - out-of-order inbound frame handling
- `sidecar-backend`
  - participant-index mapping
  - backend message codec

### Integration tests

Put these in `sidecar-rs/tests/`:

- `integration_keygen.rs`
  - simulate 3 participants, 2-of-3 keygen
- `integration_sign.rs`
  - sign a digest and verify output
- `integration_restart.rs`
  - persist a key, restart process, sign again

The integration tests should use a fake Go driver that pumps session envelopes into the gRPC server.

## First Implementation Order

Implement the sidecar in this order.

### Step 1

Scaffold:

- `sidecar-bin`
- `sidecar-app`
- `sidecar-api`
- `sidecar-types`

At the end of this step:

- sidecar starts
- sidecar exposes `Health`
- `RunSession` accepts a session and returns mocked progress

### Step 2

Build storage:

- `sidecar-store`
- `sidecar-keystore`

At the end of this step:

- encrypted key share blobs can be written and read
- presign reservation state machine exists

### Step 3

Build session engine:

- `sidecar-session`

At the end of this step:

- multiple concurrent sessions can run
- inbound and outbound frame routing works
- cancellation and timeout work

### Step 4

Integrate backend:

- `sidecar-backend`
- `cggmp21` adapter

At the end of this step:

- aux info
- keygen
- sign

### Step 5

Add presign support and hardening:

- pool refill jobs
- crash-safe reservation semantics
- metrics
- structured failure codes

## Minimum Go Changes Required

The Rust design depends on a small set of Go changes, but only these:

- add a sidecar supervisor in the worker path
- add a local gRPC client over UDS
- add direct MPC stream protocols instead of using the generic pubsub topic
- add worker APIs that request keygen/sign from the local node

The Rust sidecar should not depend on any other large Go refactor.

## Recommendation

Build the sidecar as a small Rust workspace with:

1. one process crate
2. one app/wiring crate
3. one API crate
4. one session engine crate
5. one backend adapter crate
6. one keystore crate
7. one low-level storage crate

That split is enough to keep the sidecar maintainable once real MPC logic lands, and it is concrete enough to scaffold immediately.
