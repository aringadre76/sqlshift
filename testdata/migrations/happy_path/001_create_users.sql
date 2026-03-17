-- shift:up
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE
);

-- shift:down
DROP TABLE users;
