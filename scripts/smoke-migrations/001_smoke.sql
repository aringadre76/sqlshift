-- shift:up
CREATE TABLE sqlshift_smoke_harness (id INTEGER PRIMARY KEY, note TEXT);
INSERT INTO sqlshift_smoke_harness (id, note) VALUES (1, 'sqlshift-smoke-ok');

-- shift:down
DROP TABLE sqlshift_smoke_harness;
