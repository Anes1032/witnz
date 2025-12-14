# Witnz Configuration Guide

This document explains how to configure Witnz for different deployment scenarios.

## Configuration File Location

Witnz automatically searches for configuration files in the following locations (in order):

1. `config/witnz.yaml` (recommended for Docker/production)
2. `witnz.yaml` (for local development)
3. `/etc/witnz/witnz.yaml` (system-wide installation)

**Example usage without --config flag:**
```bash
witnz init
witnz start
witnz status
```

You can override the default search with the `--config` flag:

```bash
./witnz start --config /path/to/custom/witnz.yaml
```

## Configuration Structure

### Single Node Configuration

For development or testing with a single node:

```yaml
database:
  host: "localhost"
  port: 5432
  database: "mydb"
  user: "witnz"
  password: "witnz_password"

hash:
  algorithm: sha256

node:
  id: "node1"
  bind_addr: "0.0.0.0:7000"
  data_dir: "/data/witnz"
  bootstrap: false
  peer_addrs: {}

protected_tables:
  - name: "audit_log"
    verify_interval: "10s"

alerts:
  enabled: true
  slack_webhook: ${SLACK_WEBHOOK_URL}
```

### Three-Node Cluster Configuration

For production deployment with high availability:

#### Node 1 (Bootstrap Node)

```yaml
database:
  host: "postgres"
  port: 5432
  database: "witnzdb"
  user: "witnz"
  password: "witnz_password"

hash:
  algorithm: sha256

node:
  id: "node1"
  bind_addr: "0.0.0.0:7000"
  data_dir: "/data/witnz"
  bootstrap: true
  peer_addrs:
    node2: "node2:7000"
    node3: "node3:7000"

protected_tables:
  - name: "audit_log"
    verify_interval: "30s"

alerts:
  enabled: true
  slack_webhook: ${SLACK_WEBHOOK_URL}
```

#### Node 2 (Follower)

```yaml
database:
  host: "postgres"
  port: 5432
  database: "witnzdb"
  user: "witnz"
  password: "witnz_password"

hash:
  algorithm: sha256

node:
  id: "node2"
  bind_addr: "0.0.0.0:7000"
  data_dir: "/data/witnz"
  bootstrap: false
  peer_addrs:
    node1: "node1:7000"
    node3: "node3:7000"

protected_tables:
  - name: "audit_log"
    verify_interval: "30s"

alerts:
  enabled: true
  slack_webhook: ${SLACK_WEBHOOK_URL}
```

#### Node 3 (Follower)

```yaml
database:
  host: "postgres"
  port: 5432
  database: "witnzdb"
  user: "witnz"
  password: "witnz_password"

hash:
  algorithm: sha256

node:
  id: "node3"
  bind_addr: "0.0.0.0:7000"
  data_dir: "/data/witnz"
  bootstrap: false
  peer_addrs:
    node1: "node1:7000"
    node2: "node2:7000"

protected_tables:
  - name: "audit_log"
    verify_interval: "30s"

alerts:
  enabled: true
  slack_webhook: ${SLACK_WEBHOOK_URL}
```

## Configuration Parameters

### Node Section

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `id` | string | Unique identifier for this node | Yes |
| `bind_addr` | string | Address and port for Raft communication | Yes |
| `data_dir` | string | Directory for storing Raft logs and hash chain data | Yes |
| `bootstrap` | boolean | Whether this node bootstraps the cluster (only one node should be true) | Yes |
| `peer_addrs` | map | Map of peer node IDs to their addresses | Yes (can be empty for single node) |

### Database Section

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `host` | string | PostgreSQL host address | Yes |
| `port` | integer | PostgreSQL port | Yes |
| `database` | string | PostgreSQL database name | Yes |
| `user` | string | PostgreSQL username | Yes |
| `password` | string | PostgreSQL password | Yes |

### Hash Section

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `algorithm` | string | Hash algorithm for Merkle tree construction | Yes |

**Supported Hash Algorithms:**

| Algorithm | Security | Performance | Hash Size | Use Case |
|-----------|----------|-------------|-----------|----------|
| `xxhash64` | Non-cryptographic | Fastest | 64-bit | Development/Testing |
| `xxhash128` | Non-cryptographic | Very Fast | 128-bit | High-throughput scenarios |
| `sha256` | Cryptographic | Moderate | 256-bit | **Recommended for production** |
| `blake2b_256` | Cryptographic | Fast | 256-bit | High-performance production |
| `blake3` | Cryptographic | Very Fast | 256-bit | Best performance with security |

**Recommendations:**
- **Production**: Use `sha256` (industry standard), `blake2b_256`, or `blake3` for cryptographic security
- **Development**: Use `xxhash64` or `xxhash128` for faster testing
- **High-throughput**: Use `blake3` for best balance of speed and security

Example:
```yaml
hash:
  algorithm: sha256  # Default: SHA-256
```

### Alerts Section

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `enabled` | boolean | Enable/disable alert notifications | Yes |
| `slack_webhook` | string | Slack webhook URL for notifications | No |

**Alert Triggers:**
- Real-time `UPDATE`/`DELETE` operations on protected tables
- Merkle root verification failures
- Detailed verification showing tampered records

**Environment Variables:**

Use environment variable substitution for sensitive values:

```yaml
alerts:
  enabled: true
  slack_webhook: ${SLACK_WEBHOOK_URL}
```

**Slack Setup:**

1. Create a Slack incoming webhook: https://api.slack.com/messaging/webhooks
2. Set the webhook URL as an environment variable:
   ```bash
   export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
   ```

**Alert Examples:**

When tampering is detected, Witnz sends alerts like:
```
🚨 TAMPERING DETECTED: audit_log
- Verification failed at 2025-12-15 10:30:45
- Expected Merkle root: abc123...
- Actual Merkle root: def456...
- Action required: Check database integrity
```

### Protected Tables Section

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `name` | string | Table name to protect | Yes |
| `verify_interval` | string | Interval for periodic Merkle verification (e.g., "30s", "1m", "5m") | No (default: no periodic verification) |

## Verification Intervals

The `verify_interval` parameter controls how often Witnz performs Merkle tree verification:

- **Real-time verification**: Always enabled via CDC (Change Data Capture)
- **Periodic verification**: Optional, configured via `verify_interval`
  - Recommended for production: `30s` to `5m`
  - Recommended for development: `10s` to `30s`
  - Set to empty string or omit to disable periodic verification

## Cluster Deployment Best Practices

### 1. Bootstrap Process

Only **one node** should have `bootstrap: true`. This node initializes the Raft cluster. All other nodes should have `bootstrap: false`.

### 2. Peer Configuration

Each node must list **all other nodes** in `peer_addrs`. For a 3-node cluster:
- Node1 lists: node2, node3
- Node2 lists: node1, node3
- Node3 lists: node1, node2

### 3. Network Addresses

- Use hostname-based addressing for Docker/Kubernetes: `node2:7000`
- Use IP-based addressing for bare metal: `192.168.1.102:7000`
- Ensure all nodes can reach each other on the specified ports

### 4. Data Directory

- Use persistent volumes for `data_dir` to survive container restarts
- Each node must have its own separate data directory
- Example Docker volume mapping: `./node1_data:/data/witnz`

### 5. Database Access

All nodes should connect to the **same PostgreSQL database** with:
- Identical database credentials
- Same protected table configuration
- Logical replication enabled (configured via `witnz init`)

## Environment-Specific Configuration

### Development (Single Node)

```yaml
database:
  host: "localhost"
  port: 5432
  database: "mydb"
  user: "witnz"
  password: "witnz_password"

hash:
  algorithm: xxhash64  # Fast non-cryptographic for development

node:
  id: "node1"
  bind_addr: "0.0.0.0:7000"
  data_dir: "/data/witnz"
  bootstrap: false
  peer_addrs: {}  # Empty for single node

protected_tables:
  - name: "audit_log"
    verify_interval: "10s"  # Fast verification for testing

alerts:
  enabled: false  # Disable alerts in development
```

### Production (3-Node Cluster)

```yaml
database:
  host: "postgres"
  port: 5432
  database: "witnzdb"
  user: "witnz"
  password: "witnz_password"

hash:
  algorithm: sha256  # Cryptographic hash for production

node:
  id: "node1"
  bind_addr: "0.0.0.0:7000"
  data_dir: "/data/witnz"
  bootstrap: true  # Only on one node
  peer_addrs:
    node2: "node2:7000"
    node3: "node3:7000"

protected_tables:
  - name: "audit_log"
    verify_interval: "60m"
  - name: "transactions"
    verify_interval: "10s" (critical table)

alerts:
  enabled: true
  slack_webhook: ${SLACK_WEBHOOK_URL}
```

## Checkpoint Sharing

As of the latest version, Witnz automatically shares Merkle checkpoints across the cluster via Raft consensus:

- **Leader**: Creates checkpoints and replicates them to all followers
- **Followers**: Receive and apply checkpoints automatically
- **Leader Rotation**: New leaders have access to the latest checkpoint without recalculation

This ensures optimal performance during leader rotation and consistent verification state across all nodes.

## Troubleshooting

### Leader Election Issues

If no leader is elected:
1. Verify all nodes can reach each other on the bind address
2. Check that exactly one node has `bootstrap: true`
3. Ensure peer addresses are correct and resolvable

### Verification Warnings

If you see verification warnings:
1. Check that all nodes connect to the same PostgreSQL database
2. Verify that logical replication is properly configured
3. Ensure no manual database modifications outside of Witnz monitoring

### Checkpoint Replication

To verify checkpoint replication is working:
1. Check logs for "Created and replicated Merkle checkpoint" (leader)
2. Check logs for "Applied checkpoint from Raft" (followers)
3. Ensure Raft logs show successful consensus
