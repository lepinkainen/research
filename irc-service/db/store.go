package db

import (
	"context"
	"database/sql"
	"time"
)

// Now returns the current time formatted for storage (UTC, ISO-8601 with ms).
func Now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// FormatTime formats t for storage.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return Now()
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// Network is the minimal network row used at startup.
type Network struct {
	ID       int64
	Name     string
	Host     string
	Port     int
	TLS      bool
	Nick     string
	Realname string
	SASLUser string
	SASLPass string
}

// UpsertNetwork inserts the network if missing (keyed by name) and returns
// the stored row. Existing rows are not modified — mutation goes through a
// future API, not the seed.
func UpsertNetwork(ctx context.Context, d *sql.DB, n Network) (Network, error) {
	var id int64
	err := d.QueryRowContext(ctx,
		`SELECT id FROM networks WHERE name = ?`, n.Name).Scan(&id)
	if err == nil {
		n.ID = id
		return n, nil
	}
	if err != sql.ErrNoRows {
		return Network{}, err
	}
	tls := 0
	if n.TLS {
		tls = 1
	}
	var saslUser, saslPass any
	if n.SASLUser != "" {
		saslUser = n.SASLUser
		saslPass = n.SASLPass
	}
	res, err := d.ExecContext(ctx,
		`INSERT INTO networks(name, host, port, tls, nick, realname, sasl_user, sasl_pass, autoconnect, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		n.Name, n.Host, n.Port, tls, n.Nick, n.Realname, saslUser, saslPass, Now())
	if err != nil {
		return Network{}, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return Network{}, err
	}
	n.ID = id
	return n, nil
}

// BufferKind enumerates the rows allowed in buffers.kind.
const (
	BufferChannel = "channel"
	BufferQuery   = "query"
	BufferStatus  = "status"
)

// UpsertBuffer returns the id of (network, name), creating the row if
// needed. name is the channel name or peer nick; the special name "" maps
// to the network's status buffer.
func UpsertBuffer(ctx context.Context, d *sql.DB, networkID int64, name, kind string) (int64, error) {
	if name == "" {
		name = "*status*"
		kind = BufferStatus
	}
	var id int64
	err := d.QueryRowContext(ctx,
		`SELECT id FROM buffers WHERE network_id = ? AND name = ?`,
		networkID, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	res, err := d.ExecContext(ctx,
		`INSERT INTO buffers(network_id, name, kind, created_at) VALUES (?, ?, ?, ?)`,
		networkID, name, kind, Now())
	if err != nil {
		// Racy insert from another goroutine: re-query.
		if err2 := d.QueryRowContext(ctx,
			`SELECT id FROM buffers WHERE network_id = ? AND name = ?`,
			networkID, name).Scan(&id); err2 == nil {
			return id, nil
		}
		return 0, err
	}
	return res.LastInsertId()
}

// Message is an inbound IRC event prepared for storage.
type Message struct {
	NetworkID int64
	BufferID  int64
	MsgID     string // may be empty
	Timestamp time.Time
	Sender    string
	Account   string
	Kind      string
	Target    string
	Content   string
	Raw       string
}

// InsertMessage writes a message row. msgid uniqueness is enforced by a
// partial unique index; duplicates are silently ignored.
func InsertMessage(ctx context.Context, d *sql.DB, m Message) error {
	ts := FormatTime(m.Timestamp)
	var msgid sql.NullString
	if m.MsgID != "" {
		msgid = sql.NullString{String: m.MsgID, Valid: true}
	}
	var account sql.NullString
	if m.Account != "" {
		account = sql.NullString{String: m.Account, Valid: true}
	}
	var target sql.NullString
	if m.Target != "" {
		target = sql.NullString{String: m.Target, Valid: true}
	}
	_, err := d.ExecContext(ctx,
		`INSERT OR IGNORE INTO messages
		   (network_id, buffer_id, msgid, ts, sender, account, kind, target, content, raw)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.NetworkID, m.BufferID, msgid, ts, m.Sender, account, m.Kind, target, m.Content, m.Raw)
	return err
}
