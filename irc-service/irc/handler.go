package irc

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/lrstanley/girc"

	ircdb "github.com/lepinkainen/research/irc-service/db"
)

// handler is the glue between a girc.Client and the SQLite store. One
// instance per network connection.
type handler struct {
	db          *sql.DB
	networkID   int64
	networkName string
	autojoin    []string
}

func (h *handler) register(c *girc.Client) {
	c.Handlers.Add(girc.CONNECTED, h.onConnected)
	c.Handlers.Add(girc.DISCONNECTED, h.onDisconnected)
	c.Handlers.Add(girc.PRIVMSG, h.onPrivmsg)
	c.Handlers.Add(girc.NOTICE, h.onPrivmsg) // same handling, different kind
	c.Handlers.Add(girc.JOIN, h.onJoin)
	c.Handlers.Add(girc.PART, h.onPart)
	c.Handlers.Add(girc.KICK, h.onKick)
	c.Handlers.Add(girc.TOPIC, h.onTopic)
	c.Handlers.Add(girc.QUIT, h.onQuit)
	c.Handlers.Add(girc.NICK, h.onNick)
	c.Handlers.Add(girc.MODE, h.onMode)
}

// --- event handlers ---

func (h *handler) onConnected(c *girc.Client, e girc.Event) {
	h.logStatus("connected", "")
	for _, ch := range h.autojoin {
		c.Cmd.Join(ch)
	}
}

func (h *handler) onDisconnected(_ *girc.Client, e girc.Event) {
	h.logStatus("disconnected", e.Last())
}

func (h *handler) onPrivmsg(_ *girc.Client, e girc.Event) {
	var bufName, kind string
	switch {
	case e.IsFromChannel():
		bufName = e.Params[0]
		kind = "privmsg"
	default:
		// PM: buffer is the other party, not us.
		if e.Source != nil {
			bufName = e.Source.Name
		}
		kind = "privmsg"
	}
	if e.Command == girc.NOTICE {
		kind = "notice"
	}
	content := e.Last()
	if e.IsAction() {
		kind = "action"
		content = e.StripAction()
	}
	h.storeEvent(e, bufName, ircdb.BufferChannel, kind, "", content)
}

func (h *handler) onJoin(_ *girc.Client, e girc.Event) {
	if len(e.Params) < 1 {
		return
	}
	h.storeEvent(e, e.Params[0], ircdb.BufferChannel, "join", "", "")
}

func (h *handler) onPart(_ *girc.Client, e girc.Event) {
	if len(e.Params) < 1 {
		return
	}
	reason := ""
	if len(e.Params) >= 2 {
		reason = e.Params[1]
	}
	h.storeEvent(e, e.Params[0], ircdb.BufferChannel, "part", "", reason)
}

func (h *handler) onKick(_ *girc.Client, e girc.Event) {
	if len(e.Params) < 2 {
		return
	}
	reason := ""
	if len(e.Params) >= 3 {
		reason = e.Params[2]
	}
	// target = the kicked nick
	h.storeEvent(e, e.Params[0], ircdb.BufferChannel, "kick", e.Params[1], reason)
}

func (h *handler) onTopic(_ *girc.Client, e girc.Event) {
	if len(e.Params) < 1 {
		return
	}
	h.storeEvent(e, e.Params[0], ircdb.BufferChannel, "topic", "", e.Last())
}

func (h *handler) onMode(_ *girc.Client, e girc.Event) {
	if len(e.Params) < 1 {
		return
	}
	target := e.Params[0]
	kind := "mode"
	if !girc.IsValidChannel(target) {
		// User mode changes go to the network status buffer.
		h.storeEvent(e, "", ircdb.BufferStatus, kind, target, joinRemaining(e.Params[1:]))
		return
	}
	h.storeEvent(e, target, ircdb.BufferChannel, kind, "", joinRemaining(e.Params[1:]))
}

func (h *handler) onQuit(_ *girc.Client, e girc.Event) {
	// QUIT doesn't name the channels the user was in; we log it to the
	// status buffer for now. Per-channel fan-out is a later enhancement.
	h.storeEvent(e, "", ircdb.BufferStatus, "quit", "", e.Last())
}

func (h *handler) onNick(_ *girc.Client, e girc.Event) {
	newNick := e.Last()
	h.storeEvent(e, "", ircdb.BufferStatus, "nick", newNick, "")
}

// --- helpers ---

// storeEvent is the single funnel for inbound IRC events. It upserts the
// target buffer, writes a messages row, and logs on failure.
func (h *handler) storeEvent(e girc.Event, bufName, bufKind, kind, target, content string) {
	sender := ""
	if e.Source != nil {
		sender = e.Source.Name
	}
	msgID, _ := e.Tags.Get("msgid")
	account, _ := e.Tags.Get("account")
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	raw := e.String()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bufID, err := ircdb.UpsertBuffer(ctx, h.db, h.networkID, bufName, bufKind)
	if err != nil {
		slog.Error("upsert buffer", "err", err, "network", h.networkName, "buffer", bufName)
		return
	}
	if err := ircdb.InsertMessage(ctx, h.db, ircdb.Message{
		NetworkID: h.networkID,
		BufferID:  bufID,
		MsgID:     msgID,
		Timestamp: ts,
		Sender:    sender,
		Account:   account,
		Kind:      kind,
		Target:    target,
		Content:   content,
		Raw:       raw,
	}); err != nil {
		slog.Error("insert message", "err", err, "network", h.networkName, "kind", kind)
	}
}

// logStatus writes a synthetic message (connect/disconnect) to the
// per-network status buffer.
func (h *handler) logStatus(kind, content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	bufID, err := ircdb.UpsertBuffer(ctx, h.db, h.networkID, "", ircdb.BufferStatus)
	if err != nil {
		slog.Error("upsert status buffer", "err", err, "network", h.networkName)
		return
	}
	_ = ircdb.InsertMessage(ctx, h.db, ircdb.Message{
		NetworkID: h.networkID,
		BufferID:  bufID,
		Timestamp: time.Now(),
		Sender:    "*",
		Kind:      kind,
		Content:   content,
		Raw:       "",
	})
}

func joinRemaining(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out
}
