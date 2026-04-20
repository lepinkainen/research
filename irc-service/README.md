# irc-service

Personal IRCCloud-style bouncer + client backend. See [PLAN.md](PLAN.md) for
the full architecture.

## Status

Milestone **M2 — IRC connect**: a single IRC connection driven by
[lrstanley/girc](https://github.com/lrstanley/girc), auto-reconnect with
exponential backoff, persistent logging of PRIVMSG/NOTICE/ACTION, JOIN,
PART, KICK, TOPIC, MODE, QUIT, NICK into SQLite. Status buffer per
network for connect/disconnect and network-level events. No IRCv3 caps
yet (M3) and no client API yet (M4).

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

| var            | default         | meaning                                                 |
|----------------|-----------------|---------------------------------------------------------|
| `DB_PATH`      | `./data/irc.db` | SQLite file path                                        |
| `ADDR`         | `:8080`         | HTTP listen address                                     |
| `IRC_NETWORK`  | *(empty)*       | Friendly name; if empty, no IRC connection is started   |
| `IRC_SERVER`   | *(empty)*       | IRC server hostname                                     |
| `IRC_PORT`     | `6697`          | IRC server port                                         |
| `IRC_TLS`      | `true`          | Use TLS                                                 |
| `IRC_NICK`     | `ircsvc`        | Nickname                                                |
| `IRC_USER`     | = nick          | Ident                                                   |
| `IRC_NAME`     | = nick          | Realname / gecos                                        |
| `IRC_CHANNELS` | *(empty)*       | Comma-separated channels to autojoin                    |
| `IRC_DEBUG`    | *(empty)*       | If set, dump raw IRC traffic to stderr                  |

A single network via env vars is a temporary seed. Network CRUD through
the API arrives in M4; existing rows are never modified by the seed.
