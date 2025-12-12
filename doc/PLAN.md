# Witnz - PostgreSQL Tamper Detection System

## Project Overview

Witnz is a distributed database tampering detection system for PostgreSQL that provides lightweight, blockchain-inspired tamper detection capabilities. The system is designed to detect internal fraud by database administrators and tampering during direct attacks on RDS, while being lighter-weight than solutions like Hyperledger and meeting audit requirements (SOC2, ISO27001) with minimal overhead.

## Architecture

### Core Components

1. **Lightweight Node** (Go binary)
   - Deployed as sidecar on each application server
   - Maintains hash chains and performs mutual verification
   - Single binary with no external dependencies

2. **Dashboard** (React + Go API)
   - Management UI with monitoring capabilities
   - Participates in consensus
   - Alert management and visualization

3. **PostgreSQL Integration**
   - Works with existing databases (RDS, Aurora, Cloud SQL, Supabase)
   - No schema changes required
   - Uses PostgreSQL Logical Replication for change detection

### Network Configuration

- VPN or private network deployment
- P2P mutual verification between nodes
- All nodes connect to the same PostgreSQL database

## Key Features

### Protection Modes

#### Append-only Mode
For audit/history tables where past records must remain immutable.

**Use Cases:**
- Change logs and audit trails
- Contract history
- Consent records
- Transaction logs
- Financial transaction logs
- Healthcare access logs

**Guarantees:** Past records have not been tampered with

**Behavior:**
- Calculates hash on INSERT (Chain Hash + Data Hash)
- Alerts on UPDATE/DELETE operations immediately
- Maintains hash chain across all nodes via Raft consensus
- Periodically creates Merkle Root checkpoints
- Verifies integrity using fast O(1) Merkle Root comparison
- Identifies specific tampered records via Merkle Tree traversal when needed

## Technology Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Lightweight Node | Go | Single binary, easy cross-compilation |
| DB Change Detection | PostgreSQL Logical Replication | Standard feature, low overhead |
| Consensus | Raft | Lightweight, easy implementation, sufficient fault tolerance |
| Local Storage | BoltDB | Embeddable, no additional infrastructure |
| Hash Structure | Merkle Tree | Efficient integrity verification, diff identification |
| Inter-node Communication | gRPC | High performance, type-safe |
| Dashboard | React + Go API | Modern UI, integrates with node binary |

### Key Dependencies

- `hashicorp/raft` - Distributed consensus
- `jackc/pglogrepl` - PostgreSQL Logical Replication
- `jackc/pgx/v5` - PostgreSQL driver
- `etcd-io/bbolt` - Local KV store (Raft log + hash storage)
- `grpc/grpc-go` - Inter-node communication
- `spf13/cobra` + `viper` - CLI + configuration management

## Project Structure

```
witnz/
├── cmd/witnz/main.go           # CLI entry point
├── internal/
│   ├── cdc/                    # PostgreSQL CDC integration
│   ├── consensus/              # Raft consensus implementation
│   ├── hash/                   # HashChain, MerkleTree algorithms
│   ├── storage/                # BoltDB storage layer
│   ├── verify/                 # Verification logic
│   └── config/                 # Configuration management
├── proto/                      # gRPC definitions
├── deploy/                     # Docker, Kubernetes manifests
└── doc/                        # Documentation
```

## Data Flow

### Write Flow

1. Application performs INSERT/UPDATE/DELETE to PostgreSQL
2. Changes propagate to local node via Logical Replication
3. Node calculates hashes:
   - Chain Hash: SHA256(previous_hash + data) for sequential integrity
   - Data Hash: SHA256(data) for Merkle Tree construction
4. Propagates and reaches consensus with other nodes via Raft
5. Each node saves hash chain to local storage
6. Periodically creates Merkle Root checkpoints (every 24 hours)

### Verification Flow

1. Periodic or on-demand verification triggered
2. Fast path: Compares current Merkle Root with stored checkpoint (O(1))
3. If Merkle Root matches: All records verified in milliseconds
4. If Merkle Root mismatch:
   - Retrieves target data from PostgreSQL
   - Traverses Merkle Tree to identify tampered records (O(log n))
   - Identifies specific modified, deleted, or phantom inserted records
5. Triggers alerts for any tampering detected

## CLI Commands

```bash
witnz init                              # Initialize, create Publication/Slot
witnz start                             # Start node
witnz status                            # Display cluster status
witnz verify [table]                    # Execute immediate Merkle Root verification
```

## Configuration

Configuration is managed via YAML file (`witnz-node1.yaml` for bootstrap, `witnz-node2.yaml`, `witnz-node3.yaml` for followers):

**Bootstrap Node (node1):**
```yaml
database:
  host: localhost
  port: 5432
  database: mydb
  user: witnz
  password: secret

node:
  id: node1
  bind_addr: node1:7000        # Use hostname, not 0.0.0.0 (Raft requirement)
  grpc_addr: 0.0.0.0:8000
  data_dir: /data
  bootstrap: true              # Only one node should be bootstrap
  peer_addrs:                  # Map format: {node_id: address}
    node2: node2:7000
    node3: node3:7000

protected_tables:
  - name: audit_log
    verify_interval: 30m
  - name: financial_transactions
    verify_interval: 10m

alerts:
  enabled: true
  slack_webhook: https://hooks.slack.com/...
```

**Follower Nodes (node2, node3):**
```yaml
database:
  host: localhost
  port: 5432
  database: mydb
  user: witnz
  password: secret

node:
  id: node2                    # Change to node3 for third node
  bind_addr: node2:7000        # Change to node3:7000 for third node
  grpc_addr: 0.0.0.0:8000
  data_dir: /data
  bootstrap: false             # Followers are NOT bootstrap
  peer_addrs:
    node1: node1:7000
    node3: node3:7000          # Adjust peer list for each node

protected_tables:
  - name: audit_log
    verify_interval: 30m
  - name: financial_transactions
    verify_interval: 10m

alerts:
  enabled: true
  slack_webhook: https://hooks.slack.com/...
```

## Development Guidelines

### Code Style

- **Language:** All code, comments, and documentation must be in English
- **Comments:** Keep code comments to an absolute minimum
  - Code should be self-documenting with clear variable and function names
  - Only add comments for complex algorithms or non-obvious business logic
  - Prefer extracting complex logic into well-named functions over adding explanatory comments
- **Go Conventions:** Follow standard Go idioms and best practices
- **Error Handling:** Use explicit error returns, avoid panics in library code
- **Testing:** Write table-driven tests, aim for high coverage on critical paths

### Architecture Principles

- **Single Binary:** The entire node must compile to a single executable
- **Zero Schema Changes:** No modifications to user's PostgreSQL schema
- **Minimal Dependencies:** Prefer standard library, carefully evaluate external dependencies
- **Cloud Agnostic:** Support all major PostgreSQL hosting platforms
- **Low Overhead:** Minimize performance impact on application database

### Security Considerations

- External anchor support (S3 Object Lock, public blockchain) for enhanced proof
- HSM usage and key rotation for secret key protection
- Multi-node consensus prevents single point of compromise
- All inter-node communication must be authenticated and encrypted

## Development Phases

### ✅ Phase 1: MVP (COMPLETED - v0.1.*)

#### Core Infrastructure ✅
- ✅ Single binary Go implementation (~17MB)
- ✅ Configuration management (YAML + environment variables)
- ✅ BoltDB embedded storage for hash chains
- ✅ SHA256 hash algorithms (HashChain, MerkleTree)

#### Database Integration ✅
- ✅ PostgreSQL CDC via Logical Replication (pglogrepl)
- ✅ Automatic publication/slot creation and management
- ✅ Real-time change event processing
- ✅ Support for RDS, Aurora, Cloud SQL, Supabase

#### Protection Modes ✅
- ✅ **Append-only Mode**: Immediate UPDATE/DELETE detection with alerts
- ✅ **Merkle Root Verification**: Periodic O(1) verification with tampering detection and specific record identification

#### Distributed Consensus ✅
- ✅ Raft consensus implementation (hashicorp/raft)
- ✅ Multi-node hash chain replication
- ✅ Leader election and automatic failover
- ✅ Bootstrap-based cluster formation
- ✅ Snapshot persistence and restore
- ✅ 3-node cluster tested and verified

#### Alert System ✅
- ✅ Slack webhook integration
- ✅ Tampering detection alerts (append-only)
- ✅ Merkle Root mismatch alerts
- ✅ Hash chain integrity violation alerts
- ✅ Phantom insert detection alerts

#### Testing ✅
- ✅ Unit tests (40%+ coverage)
- ✅ Integration tests (append-only mode)
- ✅ Integration tests (Merkle Root verification)
- ✅ Multi-node cluster tests with Docker Compose
- ✅ Phantom insert detection tests
- ✅ Offline tampering detection tests

#### CLI & Operations ✅
- ✅ `witnz init` - Initialize replication slot and publication
- ✅ `witnz start` - Start node with cluster support
- ✅ `witnz status` - Display node and cluster status
- ✅ `witnz verify` - Manual verification trigger
- ✅ Graceful shutdown with cleanup

### 📋 Phase 1.5: MVP Remaining Items (HIGH PRIORITY)

**Goal**: Complete critical Raft cluster tampering detection and recovery features before moving to Witnz architecture.

#### Raft Node Tampering Detection & Recovery
- [ ] **Raft Node Tampering Detection**
  - Detect when a Raft Node's hash chain becomes inconsistent with cluster consensus
  - Alert when tampering is detected on any Raft Node
  - Log tampering events with node ID, table name, sequence number

- [ ] **Automatic Raft Node Shutdown on Tampering**
  - Automatically stop compromised Raft Node to prevent further damage
  - Implement graceful shutdown with alert notification
  - Prevent compromised node from participating in consensus

- [ ] **Leader Rotation via Voting**
  - Implement periodic leader rotation mechanism
  - Force leader step-down after configurable interval (e.g., 24 hours)
  - Trigger new leader election via Raft voting
  - Configuration: `raft.leader_rotation_interval`

- [ ] **Leader Change on Leader Node Tampering**
  - Detect tampering on current leader node
  - Immediately trigger leader step-down
  - Force follower nodes to elect new leader
  - Ensure cluster continues operating with new leader

### 📋 Phase 2: Witnz Architecture Core Implementation (CURRENT FOCUS)

**Goal**: Implement the revolutionary Witnz Node architecture that establishes absolute technical superiority. Focus ONLY on the core Zero-Trust architecture without SaaS features.

**Scope**: Single-region Witnz Node PoC + External Anchoring. Multi-region deployment and auto-rotation deferred to Phase 4.

**Terminology**:
- **Raft Node**: Customer-operated node in their VPC (forms Raft cluster, has voting rights)
- **Witnz Node**: External monitoring node operated by Witnz Cloud (observer-only, no voting rights)

#### Priority 1: Witnz Node Architecture (Observer-only, No Voting Rights) 🔥 (REVOLUTIONARY)

**This is what sets Witnz apart from ALL competitors. No other solution has external monitoring nodes with mutual distrust.**

##### Witnz Node Core Implementation
- [ ] **Witnz Node Role Implementation**
  - Witnz Nodes do NOT have Raft voting rights (Observer role only)
  - Raft Nodes (customer): Form 3-5 node Raft cluster, vote, achieve consensus in customer VPC
  - Witnz Nodes (external): Receive hashes from Raft Nodes, store, monitor, alert
  - No cross-region Raft consensus needed (Raft cluster stays in customer VPC)
  - Configuration: `node.role: raft | witnz`

- [ ] **Hash Submission Protocol (gRPC)**
  - Raft Nodes: After achieving Raft consensus, submit `(record_id, chain_hash, data_hash, merkle_root)` to configured Witnz Node
  - Witnz Node: Verify Ed25519 signature, store in local BoltDB, detect inconsistencies
  - gRPC endpoint: `WitnzService.SubmitCheckpoint()`
  - Authentication: Ed25519 signature per customer to prevent tampering
  - Configuration: Customer Raft Nodes specify `witnz_node: witnz-node-1:9000`

- [ ] **Single Witnz Node PoC**
  - Deploy single Witnz Node for proof of concept
  - Test hash-only submission from customer Raft Nodes
  - Verify inconsistency detection works
  - Multi-region deployment deferred to Phase 4

##### Data Masking for Witnz Nodes (Hash-only Mode)
- [ ] **Hash-only Submission Protocol**
  - Raft Nodes: Calculate `chain_hash` and `data_hash` from raw database records
  - Witnz Node: Receives only hashes, never sees raw data or connects to customer database
  - Privacy-preserving: Customer data never leaves customer VPC
  - Witnz Node can still detect tampering via hash verification

- [ ] **Inconsistency Detection**
  - Witnz Node receives checkpoints from multiple customer Raft Nodes
  - If same `(table, sequence_num)` has different `merkle_root` from different Raft Nodes → Alert tampering
  - Alert channel: Slack, PagerDuty, or customer webhook
  - Configuration: `witnz.inconsistency_alert_threshold`

#### Priority 2: External Anchoring (Tamper-proof External Proof) 🔥 (CRITICAL)

**Defeats "all nodes compromised" scenario. This + Witnz Nodes = unbreakable.**

##### S3 Object Lock Integration
- [ ] **S3 Anchor Implementation**
  - Create `S3Anchor` struct with AWS SDK v2
  - Upload Merkle Root checkpoints to S3 with Object Lock (WORM)
  - Set retention period (e.g., 10 years) for compliance
  - Configuration: `anchoring.s3.bucket`, `anchoring.s3.interval` (default: 24h)

- [ ] **Automatic Periodic Anchoring**
  - Background goroutine uploads checkpoints every 24 hours
  - Batch multiple tables into single S3 object (cost optimization)
  - Include metadata: timestamp, table name, record count, node IDs
  - Retry logic with exponential backoff on S3 errors

- [ ] **Verification Against S3 Anchors**
  - `witnz verify --check-anchor` command
  - Fetch latest S3 checkpoint and compare with local Merkle Root
  - Alert if mismatch detected (all-node tampering scenario)
  - Generate compliance report with S3 proof

##### Blockchain Anchoring (Optional)
- [ ] **Ethereum Smart Contract**
  - Deploy simple contract: `function anchorHash(bytes32 merkleRoot, uint256 timestamp)`
  - Batch multiple checkpoints into single transaction (gas optimization)
  - Store transaction hash in BoltDB for verification
  - Configuration: `anchoring.blockchain.enabled`, `anchoring.blockchain.network`

- [ ] **Public Verification**
  - Generate Etherscan link for each anchored checkpoint
  - Allow anyone to verify Merkle Root on blockchain
  - Create compliance report generator for auditors
  - Cost estimation tool (gas price × frequency)

### 📋 Phase 3: Performance Optimization & Basic Operations

**Goal**: Optimize performance for production workloads and add minimal operational capabilities.

#### Performance Optimizations 🚀

##### Incremental Merkle Tree
- [ ] **Avoid Full Table Scan on Every Verification**
  - Cache Merkle Tree structure in BoltDB
  - On INSERT: Update only affected branch (O(log n) instead of O(n))
  - Store intermediate nodes: `merkle_tree_{table}` bucket
  - Rebuild full tree periodically (e.g., every 1000 inserts) to prevent drift

- [ ] **Billion-Record Support**
  - Test with 1 billion record table
  - Target: <10 seconds for verification (currently ~20 seconds per million)
  - Memory-efficient streaming: Process records in 10,000 record batches
  - Benchmark and document performance characteristics

##### CDC Batch Processing
- [ ] **Buffer CDC Events**
  - Accumulate up to 100 INSERT events before submitting to Raft
  - Configurable: `cdc.batch_size`, `cdc.batch_timeout` (default: 100ms)
  - Single Raft log entry for batch (reduce consensus overhead)
  - Trade-off: Slightly delayed detection (100ms) for 10x throughput

#### Basic Operational Essentials 🟡

##### Basic Reliability
- [ ] **CDC Reconnection**
  - Exponential backoff retry on PostgreSQL disconnect
  - Persist LSN for resume after restart

- [ ] **Raft Snapshot Rotation**
  - Keep last 3 snapshots, delete older ones
  - Automatic snapshot every 10,000 entries

##### Basic Observability
- [ ] **Health Check Endpoint**
  - `GET /healthz` returns 200 if process running
  - `GET /readyz` returns 200 if Raft + CDC connected

- [ ] **Basic Logging**
  - Replace `fmt.Printf` with `slog` for structured logs
  - Configurable log level (debug, info, warn, error)

### 📋 Phase 4: SaaS Platform & Enterprise Features

**Goal**: Build Witnz-as-a-Service (WaaS) platform for managed Witnz nodes, multi-tenant support, and enterprise compliance features.

#### Multi-region Witnz Nodes & Advanced Features

##### Multi-region Witnz Node Deployment
- [ ] **Geographic Distribution**
  - Deploy Witnz Nodes across 3+ regions (US-East, EU-West, AP-Tokyo)
  - Each Witnz Node is independent (no Raft between them)
  - Geographic diversity prevents single-region attacks
  - Test with AWS/GCP/Azure multi-region setup

##### Witnz Node Rotation & Long-term Attack Prevention
- [ ] **Witnz Pool Management**
  - Create pool of N Witnz Nodes (e.g., 10 nodes across 3 regions: US, EU, AP)
  - Maintain minimum M active Witnz Nodes (e.g., 4) per customer
  - Implement `WitnzRotator` for periodic node replacement
  - Configuration: `witnz.pool_size`, `witnz.active_count`, `witnz.rotation_interval`

- [ ] **Automated Rolling Rotation**
  - Every 7 days, replace 1 Witnz Node with fresh node from pool
  - No Raft membership change needed (Witnz Nodes are not Raft voters)
  - Update customer's `witnz_nodes` configuration via dashboard
  - Old Witnz Node archives data and shuts down gracefully
  - Log rotation events for audit trail

- [ ] **Attack Resistance Testing**
  - Simulate scenario: Attacker compromises Witnz Node on Day 1
  - Verify: Node is automatically rotated out by Day 7
  - Test: New Witnz Node can still verify integrity from S3/Blockchain anchors
  - Document rotation strategy in security whitepaper

#### Witnz-as-a-Service (WaaS) Platform
- [ ] **Managed Witnz Node Infrastructure**
  - Multi-region Witnz node deployment (US, EU, AP)
  - Automated Witnz node provisioning and lifecycle management
  - Customer Raft node registration API
  - Witnz node health monitoring and auto-recovery

- [ ] **Public Audit Log**
  - Public HTTP endpoint for Merkle Root checkpoints
  - Customer-specific audit log URLs (https://audit.witnz.io/{customer}/table)
  - JSON API for programmatic verification
  - Timestamped proof generation for compliance

- [ ] **Witnz Node Marketplace** (Optional)
  - Allow third-party organizations to run Witnz nodes
  - Trust scoring for Witnz node providers
  - Decentralized Witnz network

#### SaaS Multi-tenant Platform
- [ ] **Multi-tenant Architecture**
  - Customer isolation and resource quotas
  - Per-customer Raft cluster management
  - Centralized control plane for customer management
  - Customer-specific configuration and policies

- [ ] **Billing & Subscription Management**
  - Stripe integration for payment processing
  - Tiered pricing (Free, Standard, Enterprise)
  - Usage-based billing (tables, verification frequency, witness nodes)
  - Invoice generation and tax handling

- [ ] **Customer Dashboard (Web UI)**
  - Customer portal for cluster management
  - Real-time cluster topology visualization
  - Hash chain explorer with search
  - Verification history and alerts
  - Witness node configuration
  - Billing and subscription management

- [ ] **gRPC Management API**
  - Remote cluster configuration
  - Centralized verification triggers
  - Customer-specific alert routing
  - Witnz node management API

#### Security & Compliance
- [ ] **TLS/mTLS for Inter-node Communication**
  - Certificate-based authentication
  - Automatic certificate rotation
  - mTLS for Raft node-to-Witnz node communication

- [ ] **Encryption at Rest**
  - BoltDB encryption (AES-256)
  - Key management with HSM support
  - Customer-managed encryption keys (CMEK)

- [ ] **RBAC (Role-Based Access Control)**
  - User roles (Admin, Operator, Viewer)
  - Fine-grained permissions for API/UI
  - Audit logging (WHO did WHAT, WHEN)

- [ ] **Compliance & Certifications**
  - SOC2 Type II compliance documentation
  - ISO27001 compliance preparation
  - GDPR compliance (data residency, right to deletion)
  - HIPAA compliance features (BAA support)

#### Management & Tooling
- [ ] **Infrastructure as Code**
  - Kubernetes Operator for automated deployment
  - Helm charts for easy installation
  - Terraform Provider for infrastructure provisioning

- [ ] **Backup & Disaster Recovery**
  - Automated BoltDB backup to S3
  - Cross-region backup replication
  - Point-in-time recovery
  - Disaster recovery runbooks

- [ ] **Migration & Upgrade Tools**
  - Zero-downtime rolling upgrades
  - Version compatibility matrix
  - Automated migration scripts
  - Rollback procedures

#### Enterprise Features
- [ ] **Private Witnz Nodes**
  - Customer-dedicated Witnz nodes (VPC peering)
  - On-premise Witnz node support
  - Custom Witnz rotation policies

- [ ] **Advanced Alerting**
  - Integration with enterprise monitoring (DataDog, New Relic)
  - Custom alert rules and thresholds
  - Alert aggregation and reporting
  - SLA monitoring and reporting

- [ ] **Professional Services**
  - Architecture consulting
  - Custom integration development
  - Training and documentation
  - 24/7 support (Enterprise tier)

## Competitive Differentiation

- **vs Hyperledger Fabric:** Much lighter weight, single binary deployment
- **vs immudb:** Uses existing PostgreSQL, no migration needed
- **vs Amazon QLDB:** Cloud-agnostic with distributed verification
- **vs pgaudit + S3:** Includes distributed verification and consensus
- **vs ScalarDL:** Open source and startup-friendly

## Testing Strategy

- Unit tests for hash algorithms, storage layer, configuration parsing
- Integration tests for PostgreSQL replication, Raft consensus
- End-to-end tests for tampering detection scenarios
- Performance benchmarks for hash calculation and verification overhead

## Common Patterns

- Use context.Context for cancellation and timeouts
- Log errors with structured logging (consider `slog`)
- Configuration via `viper`, CLI via `cobra`
- gRPC for all inter-node communication
- BoltDB transactions for atomic state updates

## Development Environment

### Docker-based Development Setup

Use Docker Compose to create a local development environment with networked containers:

**Components:**
- PostgreSQL container with Logical Replication enabled
- Multiple witnz node containers (node1, node2, node3) for testing distributed consensus
- Shared Docker network for inter-node communication

**Setup:**
```bash
docker-compose up -d          # Start development environment
docker-compose logs -f node1  # View node logs
docker-compose down           # Stop environment
```

**Key Configuration:**
- PostgreSQL with `wal_level=logical` for replication
- Each node has its own volume for BoltDB storage
- Nodes communicate via Docker network (e.g., node1:7000)
- Expose PostgreSQL (5432) and node ports for local access
