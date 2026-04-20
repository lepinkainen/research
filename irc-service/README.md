# irc-service

Personal IRCCloud-style bouncer + client backend. See [PLAN.md](PLAN.md) for
the full architecture.

## Status

Milestone **M1 — skeleton**: Go module, embedded SQL migrations, SQLite
(WAL) store, HTTP server with `/healthz`, graceful shutdown, Docker image.
No IRC or API yet.

## Run locally

```
make dev
# or
DB_PATH=./data/irc.db ADDR=:8080 go run .
```

## Run in Docker

```
make up       # docker compose up -d --build
make down
```

The database lives at `./data/irc.db` (bind mount, not a named volume) so
Datasette can open it directly:

```
datasette ./data/irc.db
```

## Config

Environment variables:

| var       | default            | meaning                      |
|-----------|--------------------|------------------------------|
| `DB_PATH` | `./data/irc.db`    | SQLite file path             |
| `ADDR`    | `:8080`            | HTTP listen address          |
