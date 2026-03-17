-- shift:up
CREATE TABLE posts (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL,
  title VARCHAR(255) NOT NULL
);

-- shift:down
DROP TABLE posts;
