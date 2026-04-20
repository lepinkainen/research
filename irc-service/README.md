# irc-service

Personal IRCCloud-style bouncer + client backend. See [PLAN.md](PLAN.md) for
the full architecture.

## Status

Milestone **M3 — IRCv3 caps**: SASL PLAIN, auto-negotiated server-time /
message-tags / msgid / batch / account-tag / extended-join / multi-prefix /
away-notify / chghost / invite-notify, and opt-in echo-message +
labeled-response. Inbound messages are stored with the server-supplied
timestamp when available and deduplicated by `(network_id, msgid)` via a
partial unique index. Self-sent echoes are routed through the same
persistence path so M4's outbound API lands history rows automatically.
No client API yet (M4).

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
| `IRC_SASL_USER`| *(empty)*       | If set, authenticate with SASL PLAIN                    |
| `IRC_SASL_PASS`| *(empty)*       | SASL PLAIN password                                     |
| `IRC_DEBUG`    | *(empty)*       | If set, dump raw IRC traffic to stderr                  |

A single network via env vars is a temporary seed. Network CRUD through
the API arrives in M4; existing rows are never modified by the seed.
