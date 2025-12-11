-- Integration test setup for witnz

-- Create test tables
CREATE TABLE IF NOT EXISTS audit_log (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    action VARCHAR(255) NOT NULL,
    details TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    resource VARCHAR(255) NOT NULL,
    access_level VARCHAR(50) NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Set replica identity for tables (required for logical replication)
ALTER TABLE audit_log REPLICA IDENTITY FULL;
ALTER TABLE permissions REPLICA IDENTITY FULL;

-- Insert some initial data
INSERT INTO audit_log (user_id, action, details) VALUES
    (1, 'LOGIN', 'User logged in'),
    (2, 'CREATE', 'Created new resource');

INSERT INTO permissions (user_id, resource, access_level) VALUES
    (1, '/api/users', 'read'),
    (2, '/api/admin', 'write');
