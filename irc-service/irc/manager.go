// Package irc manages persistent IRC connections and writes inbound events
// into the SQLite store.
package irc

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/lrstanley/girc"

	ircdb "github.com/lepinkainen/research/irc-service/db"
	"github.com/lepinkainen/research/irc-service/hub"
)

// debugWriter returns os.Stderr when IRC_DEBUG is set, io.Discard otherwise.
func debugWriter() io.Writer {
	if os.Getenv("IRC_DEBUG") != "" {
		return os.Stderr
	}
	return io.Discard
}

// NetworkConfig is the runtime configuration for a single IRC connection.
// This is the value the caller hands the manager at startup; it is also
// what we upsert into the networks table.
type NetworkConfig struct {
	Name     string   // display/unique key, e.g. "libera"
	Host     string   // hostname of the IRC server
	Port     int      // TCP port
	TLS      bool     // use TLS
	Nick     string   // desired nickname
	User     string   // ident
	Realname string   // realname / gecos
	Channels []string // channels to autojoin after connect
	SASLUser string   // empty disables SASL
	SASLPass string
}

// Manager owns one IRC client per network.
type Manager struct {
	db   *sql.DB
	hub  *hub.Hub
	wg   sync.WaitGroup
	mu   sync.Mutex
	conn map[int64]*girc.Client // keyed by network id so the API can find a send target
}

// NewManager returns a Manager backed by the given database. If h is
// non-nil, inbound events are also published to it for fan-out to API
// consumers; pass nil in tests that don't care about broadcasts.
func NewManager(d *sql.DB, h *hub.Hub) *Manager {
	return &Manager{db: d, hub: h, conn: map[int64]*girc.Client{}}
}

// Start launches a goroutine per network that keeps the connection alive
// until ctx is cancelled. Returns after all goroutines have been spawned.
func (m *Manager) Start(ctx context.Context, nets []NetworkConfig) error {
	for _, nc := range nets {
		nrow, err := ircdb.UpsertNetwork(ctx, m.db, ircdb.Network{
			Name:     nc.Name,
			Host:     nc.Host,
			Port:     nc.Port,
			TLS:      nc.TLS,
			Nick:     nc.Nick,
			Realname: nc.Realname,
			SASLUser: nc.SASLUser,
			SASLPass: nc.SASLPass,
		})
		if err != nil {
			return err
		}
		m.wg.Add(1)
		go m.runNetwork(ctx, nrow.ID, nc)
	}
	return nil
}

// Wait blocks until every network goroutine has exited.
func (m *Manager) Wait() { m.wg.Wait() }

// ErrNotConnected indicates there's no live IRC connection for the given
// network id. Callers should surface this as a client-visible error.
var ErrNotConnected = errors.New("irc: network not connected")

// Send writes an IRC PRIVMSG to target on the given network. When
// echo-message is negotiated, the server's echo will trigger our normal
// persistence path — no local insert here.
func (m *Manager) Send(networkID int64, target, content string) error {
	m.mu.Lock()
	c := m.conn[networkID]
	m.mu.Unlock()
	if c == nil || !c.IsConnected() {
		return ErrNotConnected
	}
	c.Cmd.Message(target, content)
	return nil
}

// Join sends a JOIN for channel on the given network.
func (m *Manager) Join(networkID int64, channel string) error {
	m.mu.Lock()
	c := m.conn[networkID]
	m.mu.Unlock()
	if c == nil || !c.IsConnected() {
		return ErrNotConnected
	}
	c.Cmd.Join(channel)
	return nil
}

// Part sends a PART for channel on the given network with an optional reason.
func (m *Manager) Part(networkID int64, channel, reason string) error {
	m.mu.Lock()
	c := m.conn[networkID]
	m.mu.Unlock()
	if c == nil || !c.IsConnected() {
		return ErrNotConnected
	}
	c.Cmd.Part(channel, reason)
	return nil
}

// runNetwork keeps a single network connected, with exponential backoff
// on transient connect errors. It exits cleanly when ctx is cancelled.
func (m *Manager) runNetwork(ctx context.Context, networkID int64, nc NetworkConfig) {
	defer m.wg.Done()

	log := slog.With("network", nc.Name, "network_id", networkID)
	backoff := time.Second
	const maxBackoff = 5 * time.Minute

	for {
		if ctx.Err() != nil {
			return
		}
		client := m.buildClient(ctx, networkID, nc)

		m.mu.Lock()
		m.conn[networkID] = client
		m.mu.Unlock()

		log.Info("connecting", "host", nc.Host, "port", nc.Port, "tls", nc.TLS)
		err := client.Connect()

		m.mu.Lock()
		delete(m.conn, networkID)
		m.mu.Unlock()

		if ctx.Err() != nil {
			log.Info("connection closed on shutdown")
			return
		}
		if err != nil {
			log.Warn("connection failed", "err", err, "backoff", backoff)
		} else {
			log.Info("connection ended, reconnecting", "backoff", backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (m *Manager) buildClient(ctx context.Context, networkID int64, nc NetworkConfig) *girc.Client {
	user := nc.User
	if user == "" {
		user = nc.Nick
	}
	cfg := girc.Config{
		Server:      nc.Host,
		Port:        nc.Port,
		SSL:         nc.TLS,
		Nick:        nc.Nick,
		User:        user,
		Name:        nc.Realname,
		Version:     "irc-service",
		PingDelay:   60 * time.Second,
		PingTimeout: 30 * time.Second,
		RecoverFunc: girc.DefaultRecoverHandler,
		Debug:       debugWriter(),
		// girc auto-negotiates server-time, message-tags, msgid, batch,
		// account-tag/notify, extended-join, multi-prefix, userhost-in-names,
		// away-notify, chghost, invite-notify. We opt in to the caps that
		// need explicit consent: echoing our own outbound messages back (so
		// we can persist them with server-assigned msgid + server-time) and
		// labeled replies for matching echoes to pending sends.
		SupportedCaps: map[string][]string{
			"echo-message":     nil,
			"labeled-response": nil,
		},
	}
	if nc.TLS {
		cfg.TLSConfig = &tls.Config{ServerName: nc.Host, MinVersion: tls.VersionTLS12}
	}
	if nc.SASLUser != "" {
		cfg.SASL = &girc.SASLPlain{User: nc.SASLUser, Pass: nc.SASLPass}
	}

	client := girc.New(cfg)
	h := &handler{db: m.db, hub: m.hub, networkID: networkID, networkName: nc.Name, autojoin: nc.Channels}
	h.register(client)

	// Close the client when the parent context is cancelled. This unblocks
	// the blocking Connect() call in runNetwork.
	go func() {
		<-ctx.Done()
		client.Close()
	}()

	return client
}
