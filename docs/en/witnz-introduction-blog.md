# Built an Audit System in a Single 15MB Binary That Even DBAs Can't Fool

When auditors ask you to prove your data hasn't been tampered with, what do you show them?

Access logs? Backups? pgaudit output?

**But what if the DBA who generated those logs is the one committing fraud? How would you detect that?**

DBAs are gods (Superusers). They have the power to modify data and erase the evidence. "We just have to trust the admins" — is that really acceptable?

I built an OSS called **Witnz** to answer this question: **No Kafka, no dedicated DB, no additional servers, no complex configuration — just a single 15MB binary.**

🔗 **https://github.com/Anes1032/witnz**

---

## Witnz in 5 Seconds

Here's what happens when an attacker tries to tamper with data that should never change:

![Witnz detecting tampering](https://storage.googleapis.com/zenn-user-upload/af906d7a5345-20251212.gif)

Witnz monitors PostgreSQL's transaction log (WAL) externally and **instantly detects** unauthorized changes — regardless of who made them.

---

## Comparison with Other Solutions

| Solution | Migration Required | Deployment | Trust Model |
|----------|-------------------|------------|-------------|
| **Witnz** | No | Sidecar binary (~15MB) | Distributed Raft nodes |
| **pgaudit** | No | PostgreSQL extension | Single server logs |
| **immudb** | Yes (full DB replacement) | Dedicated database | immudb server |
| **Amazon QLDB** | Yes (full DB replacement) | AWS managed service | AWS infrastructure |
| **Hyperledger Fabric** | Yes (new infrastructure) | Multi-GB blockchain | Consortium nodes |

### vs pgaudit
- pgaudit only logs queries; Witnz actively verifies data integrity
- pgaudit logs can be tampered; Witnz uses distributed hash chains
- Witnz provides real-time alerts on tampering

### vs immudb / Amazon QLDB
- No migration required - works with existing PostgreSQL
- Same trust model (see Security Considerations below)
- Significantly lower deployment cost

### vs Hyperledger Fabric
- 1000x lighter (~15MB vs multi-GB infrastructure)
- Hours to deploy vs weeks/months
- No blockchain complexity

Witnz delivers a **blockchain-like trust model** with the simplicity of **a sidecar you can drop next to your app servers**.

---

## Why Can It Detect DBA Fraud?

The key is **monitoring from outside the DB and locking evidence via distributed consensus**.


![Image description](https://dev-to-uploads.s3.amazonaws.com/uploads/articles/h5xxtbkz3nf0bwnqf8kd.png)



### Two Layers of Defense

#### Layer 1: Real-time WAL Monitoring (Instant)
- Receives change events via PostgreSQL Logical Replication
- **Instantly detects** `UPDATE` / `DELETE` and alerts
- Even if the DBA deletes logs, Witnz has already captured the WAL

#### Layer 2: Merkle Root Verification (Periodic, Fast)
- Periodically fetches all records in **a single query** and computes Merkle Root
- Compares against stored Merkle Root Checkpoint **instantly**
- Catches tampering that bypasses Logical Replication:
  - Direct DB file manipulation
  - Manual SQL during node downtime
  - Restore from tampered backups
  - Phantom inserts via unmonitored methods

### Distributed Consensus for Tamper Resistance

- **Raft consensus** (3+ nodes recommended, works with 1)
- Nodes share "the correct DB state" (Hash Chain + Merkle Root)
- **BoltDB embedded**: Evidence stored locally, zero external DB dependency

**Even if a DBA tampers with the DB, it won't match the "ground truth" held by the Witnz cluster — and gets caught immediately.**

### Issues
Raft consensus operates on a feudal-style system with the Leader as sovereign, lacking mechanisms to prevent tampering by the Leader node.
→ This will be addressed through monitoring by Witness nodes, as described below.

---

## Tech Stack: Simplicity First

```
- Language: Go (easy cross-compilation)
- DB Integration: PostgreSQL Logical Replication (jackc/pglogrepl)
- Consensus: Raft (hashicorp/raft)
- Storage: BoltDB (etcd-io/bbolt)
- Hashing: SHA256 + Merkle Tree
- Binary Size: ~15MB
```

**Zero additional infrastructure.** No Kafka, no dedicated DB, no Java VM.

---

## Protection Mode: For Append-Only Tables

Witnz is designed for **append-only tables** like audit logs and transaction histories.

```yaml
protected_tables:
  - name: audit_logs
    verify_interval: 30m  # Merkle Root verification every 30 min

  - name: financial_transactions
    verify_interval: 10m  # Higher frequency (still seconds for 1M records)
```

### What Attacks Can It Detect?

| Attack Scenario | Detection Method | Timing | Performance |
|-----------------|------------------|--------|-------------|
| `UPDATE` / `DELETE` via SQL | Logical Replication | **Instant** | Real-time |
| Direct DB file manipulation | Merkle Root verification | **Next check** | Fast (seconds) |
| Tampering during node downtime | Merkle Root verification | **On startup** | Fast (seconds) |
| Phantom Insert | Merkle Root verification | **Next check** | Fast (seconds) |
| Restore Tampered DB backup | Merkle Root verification | **Next check** | Fast (seconds) |

---

## Getting Started (Single Node)

### 1. Enable Logical Replication in PostgreSQL

```sql
SHOW wal_level;  -- Should be 'logical'
```

### 2. Download Witnz

```bash
# Linux (amd64)
curl -sSL https://github.com/Anes1032/witnz/releases/latest/download/witnz-linux-amd64 \
  -o /usr/local/bin/witnz
chmod +x /usr/local/bin/witnz
```

### 3. Create Config

```yaml
# witnz.yaml
database:
  host: ${DB_HOST}           # e.g., "postgres" or "prod-db.example.com"
  port: ${DB_PORT}           # e.g., 5432
  database: ${DB_NAME}       # e.g., "witnzdb"
  user: ${DB_USER}           # e.g., "witnz"
  password: ${DB_PASSWORD}   # Use environment variable

hash:
  algorithm: sha256

node:
  id: node
  data_dir: /data
  bootstrap: true
  peer_addrs: []

protected_tables:
  - name: audit_log
    verify_interval: 30s

alerts:
  enabled: true
```

### 4. Run

```bash
witnz init --config witnz.yaml
witnz start --config witnz.yaml
```

**That's it.** A scalable audit system running from a single 15MB binary.

---

## Try It with Docker

```bash
git clone https://github.com/Anes1032/witnz.git
cd witnz
docker-compose up
```

Three Witnz nodes spin up and start monitoring PostgreSQL.

---

## Current Status

### Implemented Features
- Append-only mode with real-time UPDATE/DELETE detection
- Merkle Root verification with specific tampered record identification
- 3-node Raft cluster with automatic failover
- PostgreSQL Logical Replication integration
- Slack webhook alerts
- Multi-platform support (Linux, macOS, Windows)

## Security Considerations

### Raft Leader Compromise

Witnz has a fundamental limitation: if a Raft leader node is compromised with **root access**, it can submit false hash values that followers will accept.

**However, this requires:**
- Root access to the leader node's server
- Ability to modify the running binary or restart with a tampered version

### The Same Applies to Other Solutions

| Solution | Server Root Compromise |
|----------|----------------------|
| **Witnz** | Attacker can submit false hashes |
| **immudb** | Attacker can submit false data and proofs |
| **Amazon QLDB** | Attacker with AWS access can manipulate |
| **Any software** | Root access = full control |

**No software-only solution can protect against server root compromise.** This is a fundamental limitation shared by all database integrity tools, including immudb.

The only theoretical protection is hardware-based root of trust (TPM, AWS Nitro Enclave, Intel SGX), which requires trusting the hardware vendor.

### What Witnz Protects Against

- Database administrator misconduct (without server root access)
- SQL injection attacks modifying audit records
- Direct database file tampering (detected via Merkle Root)
- Application-level bugs causing unauthorized modifications

---

🔗 **https://github.com/Anes1032/witnz**
