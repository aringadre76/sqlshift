-- shift:up
CREATE TABLE comments (
  id INTEGER PRIMARY KEY,
  body VARCHAR(255) NOT NULL
);

-- shift:down
DROP TABLE comments;
