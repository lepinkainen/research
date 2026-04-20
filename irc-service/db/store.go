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

// Buffer is the DTO returned for newly-created buffers so the API can
// push a buffer_created event without a second query.
type Buffer struct {
	ID        int64
	NetworkID int64
	Name      string
	Kind      string
	Topic     string
	Joined    bool
	CreatedAt string
}

// UpsertBuffer returns the id of (network, name), creating the row if
// needed. If the buffer was freshly created, created=true and buf is
// populated so callers can broadcast it. name is the channel name or
// peer nick; the special name "" maps to the network's status buffer.
func UpsertBuffer(ctx context.Context, d *sql.DB, networkID int64, name, kind string) (id int64, created bool, buf Buffer, err error) {
	if name == "" {
		name = "*status*"
		kind = BufferStatus
	}
	err = d.QueryRowContext(ctx,
		`SELECT id FROM buffers WHERE network_id = ? AND name = ?`,
		networkID, name).Scan(&id)
	if err == nil {
		return id, false, Buffer{}, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, Buffer{}, err
	}
	now := Now()
	res, err := d.ExecContext(ctx,
		`INSERT INTO buffers(network_id, name, kind, created_at) VALUES (?, ?, ?, ?)`,
		networkID, name, kind, now)
	if err != nil {
		// Racy insert from another goroutine: re-query.
		if err2 := d.QueryRowContext(ctx,
			`SELECT id FROM buffers WHERE network_id = ? AND name = ?`,
			networkID, name).Scan(&id); err2 == nil {
			return id, false, Buffer{}, nil
		}
		return 0, false, Buffer{}, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, false, Buffer{}, err
	}
	return id, true, Buffer{
		ID: id, NetworkID: networkID, Name: name, Kind: kind, CreatedAt: now,
	}, nil
}

// ListNetworks returns every network row for /api/state.
func ListNetworks(ctx context.Context, d *sql.DB) ([]Network, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, name, host, port, tls, nick, COALESCE(realname,'')
		 FROM networks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Network
	for rows.Next() {
		var n Network
		var tls int
		if err := rows.Scan(&n.ID, &n.Name, &n.Host, &n.Port, &tls, &n.Nick, &n.Realname); err != nil {
			return nil, err
		}
		n.TLS = tls == 1
		out = append(out, n)
	}
	return out, rows.Err()
}

// ListBuffers returns every buffer row for /api/state.
func ListBuffers(ctx context.Context, d *sql.DB) ([]Buffer, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, network_id, name, kind, COALESCE(topic,''), joined, created_at
		 FROM buffers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Buffer
	for rows.Next() {
		var b Buffer
		var joined int
		if err := rows.Scan(&b.ID, &b.NetworkID, &b.Name, &b.Kind, &b.Topic, &joined, &b.CreatedAt); err != nil {
			return nil, err
		}
		b.Joined = joined == 1
		out = append(out, b)
	}
	return out, rows.Err()
}

// StoredMessage mirrors the messages table for API responses.
type StoredMessage struct {
	ID        int64
	NetworkID int64
	BufferID  int64
	MsgID     string
	TS        string
	Sender    string
	Account   string
	Kind      string
	Target    string
	Content   string
}

// RecentMessages returns the last `limit` rows for a buffer, ordered
// oldest-first so the UI can append them in display order.
func RecentMessages(ctx context.Context, d *sql.DB, bufferID int64, limit int) ([]StoredMessage, error) {
	return messagesQuery(ctx, d,
		`SELECT id, network_id, buffer_id, COALESCE(msgid,''), ts, sender,
		        COALESCE(account,''), kind, COALESCE(target,''), content
		 FROM (
		   SELECT * FROM messages WHERE buffer_id = ? ORDER BY id DESC LIMIT ?
		 ) ORDER BY id ASC`,
		bufferID, limit)
}

// MessagesBefore returns up to `limit` rows whose id is strictly less
// than `before`, ordered oldest-first. Used for infinite-scroll
// backlog fetch.
func MessagesBefore(ctx context.Context, d *sql.DB, bufferID, before int64, limit int) ([]StoredMessage, error) {
	return messagesQuery(ctx, d,
		`SELECT id, network_id, buffer_id, COALESCE(msgid,''), ts, sender,
		        COALESCE(account,''), kind, COALESCE(target,''), content
		 FROM (
		   SELECT * FROM messages WHERE buffer_id = ? AND id < ?
		   ORDER BY id DESC LIMIT ?
		 ) ORDER BY id ASC`,
		bufferID, before, limit)
}

func messagesQuery(ctx context.Context, d *sql.DB, q string, args ...any) ([]StoredMessage, error) {
	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoredMessage
	for rows.Next() {
		var m StoredMessage
		if err := rows.Scan(&m.ID, &m.NetworkID, &m.BufferID, &m.MsgID, &m.TS,
			&m.Sender, &m.Account, &m.Kind, &m.Target, &m.Content); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// LookupBuffer returns (network_id, name) for a buffer id, used to map
// client-issued send commands to an IRC target.
func LookupBuffer(ctx context.Context, d *sql.DB, bufferID int64) (networkID int64, name, kind string, err error) {
	err = d.QueryRowContext(ctx,
		`SELECT network_id, name, kind FROM buffers WHERE id = ?`, bufferID,
	).Scan(&networkID, &name, &kind)
	return
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
// partial unique index; duplicates trigger INSERT OR IGNORE and are
// reported as (0, false, nil). On a fresh insert the row id is returned
// along with the normalized timestamp so callers can broadcast without
// re-querying.
func InsertMessage(ctx context.Context, d *sql.DB, m Message) (id int64, ts string, inserted bool, err error) {
	ts = FormatTime(m.Timestamp)
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
	res, err := d.ExecContext(ctx,
		`INSERT OR IGNORE INTO messages
		   (network_id, buffer_id, msgid, ts, sender, account, kind, target, content, raw)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.NetworkID, m.BufferID, msgid, ts, m.Sender, account, m.Kind, target, m.Content, m.Raw)
	if err != nil {
		return 0, ts, false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, ts, false, err
	}
	if affected == 0 {
		return 0, ts, false, nil
	}
	id, err = res.LastInsertId()
	return id, ts, true, err
}
