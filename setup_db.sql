DROP TABLE replies;
DROP TABLE topics;
DROP TABLE sessions;
DROP TABLE logins;
DROP TABLE users;

CREATE TABLE users (
  id         SERIAL PRIMARY KEY,
  uu_id      VARCHAR(255) NOT NULL UNIQUE,
  name       VARCHAR(255) NOT NULL UNIQUE,
  email      VARCHAR(255) NOT NULL UNIQUE,
  password   VARCHAR(255) NOT NULL,
  created_at TIMESTAMP NOT NULL   
);

CREATE TABLE logins (
  id          SERIAL PRIMARY KEY,
  uu_id       VARCHAR(255) NOT NULL UNIQUE,
  user_name   VARCHAR(255),
  user_id     SERIAL REFERENCES users(id),
  state       TEXT,
  last_update TIMESTAMP NOT NULL,
  created_at  TIMESTAMP NOT NULL   
);

CREATE TABLE sessions (
  id           SERIAL PRIMARY KEY,
  uu_id        VARCHAR(255) NOT NULL UNIQUE,
  state        TEXT,
  topic_uu_id TEXT,
  topic_id    SERIAL,
  created_at   TIMESTAMP NOT NULL
);

CREATE TABLE topics (
  id          SERIAL PRIMARY KEY,
  uu_id       VARCHAR(255) NOT NULL UNIQUE,
  topic       TEXT,
  num_replies SERIAL,
  owner       VARCHAR(255),
  user_id     SERIAL REFERENCES users(id),
  last_update TIMESTAMP NOT NULL,
  created_at  TIMESTAMP NOT NULL       
);

CREATE TABLE replies (
  id          SERIAL PRIMARY KEY,
  uu_id       VARCHAR(255) NOT NULL UNIQUE,
  body        TEXT,
  contributor VARCHAR(255),
  user_id     SERIAL REFERENCES users(id),
  topic_id   SERIAL REFERENCES topics(id),
  created_at  TIMESTAMP NOT NULL  
);
