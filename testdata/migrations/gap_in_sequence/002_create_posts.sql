-- shift:up
CREATE TABLE posts (
  id INTEGER PRIMARY KEY,
  title VARCHAR(255) NOT NULL
);

-- shift:down
DROP TABLE posts;
