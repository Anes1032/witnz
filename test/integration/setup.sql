-- Integration test setup for witnz

-- ========================================
-- Table: audit_log (for append-only mode)
-- ========================================
CREATE TABLE IF NOT EXISTS audit_log (
    id SERIAL PRIMARY KEY,
    action VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

-- ========================================
-- Table: permissions (for state-integrity mode)
-- ========================================
CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER,
    role VARCHAR(50),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Set replica identity for tables (required for logical replication)
ALTER TABLE audit_log REPLICA IDENTITY FULL;
ALTER TABLE permissions REPLICA IDENTITY FULL;

-- Insert initial data for state-integrity test
INSERT INTO permissions (user_id, role) VALUES (1, 'admin'), (2, 'user'), (3, 'guest');
