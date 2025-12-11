-- Test operations for witnz integration testing

-- 1. Insert initial legal record (builds hash chain)
INSERT INTO audit_log (user_id, action, details) VALUES
    (3, 'UPDATE', 'Updated user profile');

-- 2. Attempt tampering (UPDATE on Append-Only Table)
-- This should trigger "TAMPERING DETECTED" log and break the hash chain
UPDATE audit_log SET action = 'TAMPERED' WHERE id = 1;
