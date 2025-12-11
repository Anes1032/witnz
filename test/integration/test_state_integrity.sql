-- Test operations for state integrity mode
-- This simulates tampering on the permissions table (state_integrity mode)

-- 1. Tamper with permissions table (UPDATE on state_integrity table)
UPDATE permissions SET access_level = 'hacked' WHERE user_id = 1;
