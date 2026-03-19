-- shift:up
CREATE TABLE e2e_check (id INTEGER PRIMARY KEY, msg TEXT NOT NULL);

-- shift:down
DROP TABLE e2e_check;
