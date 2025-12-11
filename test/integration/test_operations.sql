-- Test operations for witnz integration testing

-- Test 1: Normal INSERT operations (should succeed)
INSERT INTO audit_log (user_id, action, details) VALUES
    (3, 'UPDATE', 'Updated user profile');

-- Test 2: Normal INSERT operations (should succeed)
INSERT INTO audit_log (user_id, action, details) VALUES
    (4, 'DELETE', 'Deleted old records');

-- Test 3: UPDATE operation on append-only table (should trigger alert)
-- Uncomment to test tampering detection:
-- UPDATE audit_log SET action = 'MODIFIED' WHERE id = 1;

-- Test 4: DELETE operation on append-only table (should trigger alert)
-- Uncomment to test tampering detection:
-- DELETE FROM audit_log WHERE id = 2;

-- Test 5: Normal operations on state_integrity table
UPDATE permissions SET access_level = 'admin' WHERE user_id = 1;

INSERT INTO permissions (user_id, resource, access_level) VALUES
    (3, '/api/reports', 'read');
