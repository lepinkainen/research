# irc-service — Personal IRCCloud-style IRC client

A self-hosted, single-user IRC bouncer + client suite. The backend stays
connected 24/7, logs everything to SQLite, and exposes a streaming API that
multiple clients (web, TUI, macOS) connect to over Tailscale.

## Goals

- Keep IRC connections alive when no client is attached (bouncer behaviour).
- Persist every message in a SQLite database that Datasette can read directly,
  with FTS5 search across history.
- Speak modern IRC: IRCv3 capabilities, SASL, server-time, message tags,
  chathistory.
- Clients are dumb terminals — they render whatever the backend streams and
  ask the backend for backlog. No IRC parsing on the client side.
- First client is a Web UI. TUI and SwiftUI macOS clients come later but the
  API is designed for them from day one.

## Non-goals (v1)

- No authentication. Backend binds to the Tailnet only, single user.
- No multi-user / per-user state.
- No file uploads, no pasting service, no integrations (link previews, etc.).
- No mobile client, no push notifications.

## Architecture

```
                   ┌─────────────────────────┐
                   │    irc-service (Go)     │
                   │   Docker, restart=always│
                   │                         │
  IRC servers ◄──► │  IRC client pool        │
                   │  (girc, IRCv3 caps)     │
                   │         │               │
                   │         ▼               │
                   │  Event bus (in-proc)    │
                   │     │           │       │
                   │     ▼           ▼       │
                   │  SQLite      WebSocket  │
                   │  writer      hub        │
                   │  (FTS5)         │       │
                   └─────────┼───────┼───────┘
                             │       │
                       /data │       │ :8080  (Tailnet only)
                             ▼       ▼
                    ./data/irc.db    │
                       ▲             │
                       │             │
                  Datasette       Web UI
                  (read-only)    TUI / SwiftUI
```

### Components

1. **IRC connection manager** — one persistent goroutine per network.
   Auto-reconnects with exponential backoff. Negotiates IRCv3 caps on
   connect. Emits typed events on the in-process bus.
2. **SQLite writer** — single goroutine owns the write connection, drains
   the event bus, inserts rows. WAL mode so Datasette and the web UI can
   read concurrently.
3. **WebSocket hub** — fans events out to connected clients. Each client
   declares which networks/channels it's interested in (or "all"). Also
   handles RPC-style commands (send message, join, part, history).
4. **HTTP server** — serves the web UI static files and a small REST surface
   for things that don't fit a streaming model (initial state snapshot,
   history pagination, search).

## IRC library choice

`github.com/ergochat/irc-go` — modern, IRCv3-aware, written by the Ergo
maintainers. Alternative: `github.com/lrstanley/girc`. Decision deferred
until we prototype both for an hour. Both support the caps we need; pick on
ergonomics.

### IRCv3 capabilities to negotiate

Required:
- `server-time` — every stored message gets the server's authoritative
  timestamp instead of "when we wrote it to disk".
- `message-tags` — needed for `msgid`, `+typing`, `+draft/reply`.
- `account-tag`, `account-notify`, `extended-join` — track services account
  per nick so we can group `nick!user@host` changes.
- `away-notify`, `chghost`, `multi-prefix`, `userhost-in-names`.
- `batch` — required for chathistory and for grouping NAMES bursts.
- `echo-message` + `labeled-response` — so when we send a message we wait
  for the server's echo (with its msgid + server-time) before persisting.
  This avoids dupes and gives the message a stable id.
- `sasl` (PLAIN, EXTERNAL) — credentials in the network config.

Nice to have:
- `draft/chathistory` — pull missed messages from servers that support it
  on (re)connect, so the local log is gap-free even after backend downtime.
- `standard-replies`, `setname`, `invite-notify`.

## Data model

SQLite, single file at `/data/irc.db`. WAL mode. Datasette-friendly: no
blobs, ISO-8601 timestamps, integer primary keys, all FKs declared.

```sql
CREATE TABLE networks (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL UNIQUE,        -- "libera", "oftc"
  host         TEXT NOT NULL,
  port         INTEGER NOT NULL,
  tls          INTEGER NOT NULL DEFAULT 1,
  nick         TEXT NOT NULL,
  realname     TEXT,
  sasl_user    TEXT,
  sasl_pass    TEXT,
  autoconnect  INTEGER NOT NULL DEFAULT 1,
  created_at   TEXT NOT NULL                -- ISO-8601 UTC
);

CREATE TABLE buffers (                       -- channel or PM
  id           INTEGER PRIMARY KEY,
  network_id   INTEGER NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,                -- "#go-nuts", "alice"
  kind         TEXT NOT NULL,                -- 'channel' | 'query' | 'status'
  topic        TEXT,
  joined       INTEGER NOT NULL DEFAULT 0,
  last_seen_id INTEGER,                      -- for unread tracking
  created_at   TEXT NOT NULL,
  UNIQUE (network_id, name)
);

CREATE TABLE messages (
  id          INTEGER PRIMARY KEY,
  network_id  INTEGER NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
  buffer_id   INTEGER NOT NULL REFERENCES buffers(id)  ON DELETE CASCADE,
  msgid       TEXT,                          -- IRCv3 msgid tag, may be NULL
  ts          TEXT NOT NULL,                 -- ISO-8601 UTC, from server-time
  sender      TEXT NOT NULL,                 -- nick at send time
  account     TEXT,                          -- services account if known
  kind        TEXT NOT NULL,                 -- privmsg|notice|action|join|
                                             --   part|quit|nick|mode|topic|
                                             --   kick|self
  target      TEXT,                          -- e.g. new nick for NICK
  content     TEXT NOT NULL DEFAULT '',
  raw         TEXT NOT NULL,                 -- full IRC line for forensics
  UNIQUE (network_id, msgid)                 -- partial: msgid IS NOT NULL
);

CREATE INDEX messages_buffer_ts ON messages(buffer_id, ts);

-- FTS5 over the searchable bits. external-content table to avoid duplication.
CREATE VIRTUAL TABLE messages_fts USING fts5(
  content, sender,
  content='messages', content_rowid='id',
  tokenize = 'unicode61 remove_diacritics 2'
);

-- Triggers to keep FTS in sync
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
```

Migrations live in `migrations/` as numbered SQL files; applied at startup
inside a transaction.

## Backend API

Single port (default 8080). Tailnet-only bind: `tailscale0` interface or
the user's tailnet IP, never `0.0.0.0` outside the container. Inside the
container we bind `0.0.0.0:8080` and rely on Docker port-mapping
(`127.0.0.1:8080:8080` plus a Tailscale serve / sidecar) to keep it private.

### REST

- `GET  /api/state` — full snapshot: networks, buffers, last N messages per
  buffer, connection state. Used by clients on first load.
- `GET  /api/buffers/:id/history?before=<id>&limit=200` — backlog paging.
- `GET  /api/search?q=...&network=...&buffer=...` — FTS5 query.
- `POST /api/networks`, `PATCH`, `DELETE` — network CRUD.
- `POST /api/networks/:id/connect`, `/disconnect`.

### WebSocket  `/api/stream`

Bidirectional JSON, one message per frame.

Server → client events:
```json
{"type":"message","buffer_id":12,"id":98765,"ts":"...","sender":"alice",
 "kind":"privmsg","content":"hi"}
{"type":"buffer_update","buffer_id":12,"topic":"...","joined":true}
{"type":"network_state","network_id":2,"state":"connected"}
{"type":"presence","buffer_id":12,"nick":"bob","action":"join"}
{"type":"typing","buffer_id":12,"nick":"alice"}        // +typing tag
```

Client → server commands:
```json
{"type":"send","buffer_id":12,"content":"hello"}
{"type":"join","network_id":2,"channel":"#go-nuts"}
{"type":"part","buffer_id":12,"reason":"bbl"}
{"type":"mark_read","buffer_id":12,"message_id":98765}
{"type":"history","buffer_id":12,"before":98765,"limit":200}
```

Echoes / errors come back tagged with a client-supplied `req_id`.

## Web UI (v1)

Plain HTML + vanilla JS modules + a tiny CSS file. No build step. Goal is
something maintainable forever, not pretty. Layout:

- **Left pane**: networks → channels/queries, with unread badges.
- **Centre pane**: scrollback for the active buffer, infinite-scroll up to
  load history. Input box with `/`-command parsing at the bottom.
- **Right pane**: nick list for channels.

State held in a single store object, updated by WebSocket events. On open:
fetch `/api/state`, then attach the WebSocket and apply deltas. Unread is
client-side from `last_seen_id`.

Files:
- `web/index.html`
- `web/app.js` (store, WebSocket, render)
- `web/style.css`

Served by the Go binary from an embedded `embed.FS`.

## Docker / deployment

```
irc-service/
├── Dockerfile          # multi-stage; final image alpine + binary + web/
├── compose.yaml        # bind ./data to /data, ports 127.0.0.1:8080:8080
├── Makefile            # build, dev, test, docker, up, down
├── go.mod
├── main.go
├── config.go           # YAML/env config
├── db/                 # schema, migrations, queries
├── irc/                # connection manager, event types
├── api/                # http + websocket handlers
├── web/                # static client
└── migrations/
    └── 0001_init.sql
```

`./data/irc.db` is the only state. Backup = copy that file (use
`sqlite3 .backup` to be safe with WAL).

## Tailscale story

Out of scope for the binary itself. Two viable deploys:

1. Run the container on a host already on the tailnet, publish to
   `127.0.0.1:8080`, expose via `tailscale serve --bg http://localhost:8080`.
2. Run a Tailscale sidecar container sharing the network namespace.

Document option 1 in the README; it's simpler and the user asked for
zero-auth.

## Future clients (design notes only)

- **TUI**: Go + `bubbletea`. Same WebSocket protocol. Lives in
  `clients/tui/`.
- **macOS SwiftUI**: `URLSessionWebSocketTask`, `Codable` structs mirroring
  the JSON event types. Lives in `clients/macos/`. Could share the JSON
  schema via a generated `events.json` if it gets unwieldy.

## Milestones

1. **M1 — skeleton**: Go module, Dockerfile, schema + migrations, can
   start and exit cleanly. Datasette can open the empty DB.
2. **M2 — IRC connect**: connect to one network, log raw lines into
   `messages`, no API yet. Verify FTS5 search via Datasette.
3. **M3 — IRCv3 caps**: server-time, message-tags, msgid dedup,
   echo-message + labeled-response on outbound.
4. **M4 — API**: REST snapshot + WebSocket stream + send/join/part.
5. **M5 — Web UI**: minimal but usable; can read scrollback, send
   messages, switch buffers.
6. **M6 — chathistory**: pull missed messages on reconnect from servers
   that support it.

Stop after M5 for v1; M6 is a nice-to-have.

## Open questions

- Library: `ergochat/irc-go` vs `lrstanley/girc`. Spike both before M2.
- Config format: YAML on disk vs network rows in SQLite. Likely
  SQLite-only (mutate via API), with a `--seed` flag for first run.
- Do we ever want to run a TLS reverse proxy in front? Probably not while
  the only access path is Tailscale.
- How long do we keep raw lines? Forever for now; revisit if the DB
  blows past a few GB.
