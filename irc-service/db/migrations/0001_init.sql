CREATE TABLE networks (
  id          INTEGER PRIMARY KEY,
  name        TEXT    NOT NULL UNIQUE,
  host        TEXT    NOT NULL,
  port        INTEGER NOT NULL,
  tls         INTEGER NOT NULL DEFAULT 1,
  nick        TEXT    NOT NULL,
  realname    TEXT,
  sasl_user   TEXT,
  sasl_pass   TEXT,
  autoconnect INTEGER NOT NULL DEFAULT 1,
  created_at  TEXT    NOT NULL
);

CREATE TABLE buffers (
  id           INTEGER PRIMARY KEY,
  network_id   INTEGER NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
  name         TEXT    NOT NULL,
  kind         TEXT    NOT NULL CHECK (kind IN ('channel','query','status')),
  topic        TEXT,
  joined       INTEGER NOT NULL DEFAULT 0,
  last_seen_id INTEGER,
  created_at   TEXT    NOT NULL,
  UNIQUE (network_id, name)
);

CREATE TABLE messages (
  id         INTEGER PRIMARY KEY,
  network_id INTEGER NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
  buffer_id  INTEGER NOT NULL REFERENCES buffers(id)  ON DELETE CASCADE,
  msgid      TEXT,
  ts         TEXT    NOT NULL,
  sender     TEXT    NOT NULL,
  account    TEXT,
  kind       TEXT    NOT NULL,
  target     TEXT,
  content    TEXT    NOT NULL DEFAULT '',
  raw        TEXT    NOT NULL
);

CREATE INDEX messages_buffer_ts ON messages(buffer_id, ts);
CREATE UNIQUE INDEX messages_network_msgid
  ON messages(network_id, msgid) WHERE msgid IS NOT NULL;

CREATE VIRTUAL TABLE messages_fts USING fts5(
  content, sender,
  content='messages', content_rowid='id',
  tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER messages_ai AFTER INSERT ON messages BEGIN
  INSERT INTO messages_fts(rowid, content, sender)
    VALUES (new.id, new.content, new.sender);
END;

CREATE TRIGGER messages_ad AFTER DELETE ON messages BEGIN
  INSERT INTO messages_fts(messages_fts, rowid, content, sender)
    VALUES('delete', old.id, old.content, old.sender);
END;

CREATE TRIGGER messages_au AFTER UPDATE ON messages BEGIN
  INSERT INTO messages_fts(messages_fts, rowid, content, sender)
    VALUES('delete', old.id, old.content, old.sender);
  INSERT INTO messages_fts(rowid, content, sender)
    VALUES (new.id, new.content, new.sender);
END;
