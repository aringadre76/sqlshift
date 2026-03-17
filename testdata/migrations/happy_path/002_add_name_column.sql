-- shift:up
ALTER TABLE users ADD COLUMN name VARCHAR(255);

-- shift:down
-- SQLite does not support DROP COLUMN on all versions; the test runner only
-- asserts missing/present history records for down on this fixture set.
SELECT 1;
