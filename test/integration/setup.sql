-- Integration test setup for witnz

-- ========================================
-- Table: audit_log (protected table)
-- ========================================
CREATE TABLE IF NOT EXISTS audit_log (
    id SERIAL PRIMARY KEY,
    action VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Set replica identity for tables (required for logical replication)
ALTER TABLE audit_log REPLICA IDENTITY FULL;
