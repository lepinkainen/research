package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	ircdb "github.com/lepinkainen/research/irc-service/db"
)

// clientCmd is the set of verbs a client can send. We union-decode from
// JSON into this struct and dispatch on Type; fields irrelevant to the
// chosen verb are simply ignored.
type clientCmd struct {
	Type     string `json:"type"`
	ReqID    string `json:"req_id,omitempty"`
	BufferID int64  `json:"buffer_id,omitempty"`
	Content  string `json:"content,omitempty"`
	Before   int64  `json:"before,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// ack / error envelopes. We keep these separate from IRC event types so
// the client can tell them apart by the top-level "type".
type ackEnvelope struct {
	Type  string `json:"type"`
	ReqID string `json:"req_id"`
}

type errorEnvelope struct {
	Type    string `json:"type"`
	ReqID   string `json:"req_id,omitempty"`
	Message string `json:"message"`
}

type historyResult struct {
	Type     string       `json:"type"`
	ReqID    string       `json:"req_id"`
	BufferID int64        `json:"buffer_id"`
	Messages []messageDTO `json:"messages"`
}

// stream is the WebSocket endpoint. It subscribes to the event hub and
// forwards every published event as JSON; it also reads client commands
// and dispatches them to the IRC manager or the SQLite store.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	// The service is expected to be reachable only via loopback or a
	// Tailnet IP, so we don't bother with Origin checks.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Error("ws accept", "err", err)
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	events, unsub := s.Hub.Subscribe(256)
	defer unsub()

	// writer goroutine: forwards hub events to the socket. A 10s write
	// deadline prevents one wedged client from hanging the server.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
				err := wsjson.Write(wctx, c, ev)
				wcancel()
				if err != nil {
					return
				}
			}
		}
	}()

	// reader loop: run in the request goroutine so we exit cleanly when
	// the client disconnects.
	for {
		var cmd clientCmd
		err := wsjson.Read(ctx, c, &cmd)
		if err != nil {
			var ce websocket.CloseError
			if !errors.As(err, &ce) && ctx.Err() == nil {
				slog.Debug("ws read", "err", err)
			}
			cancel()
			<-done
			return
		}
		s.handleCmd(ctx, c, cmd)
	}
}

func (s *Server) handleCmd(ctx context.Context, c *websocket.Conn, cmd clientCmd) {
	switch cmd.Type {
	case "send":
		s.cmdSend(ctx, c, cmd)
	case "history":
		s.cmdHistory(ctx, c, cmd)
	case "join":
		s.cmdJoin(ctx, c, cmd)
	case "part":
		s.cmdPart(ctx, c, cmd)
	default:
		writeWSErr(ctx, c, cmd.ReqID, "unknown command: "+cmd.Type)
	}
}

func (s *Server) cmdSend(ctx context.Context, c *websocket.Conn, cmd clientCmd) {
	if cmd.BufferID == 0 || cmd.Content == "" {
		writeWSErr(ctx, c, cmd.ReqID, "send requires buffer_id and content")
		return
	}
	networkID, name, kind, err := ircdb.LookupBuffer(ctx, s.DB, cmd.BufferID)
	if err != nil {
		writeWSErr(ctx, c, cmd.ReqID, "unknown buffer")
		return
	}
	if kind == ircdb.BufferStatus {
		writeWSErr(ctx, c, cmd.ReqID, "cannot send to status buffer")
		return
	}
	if err := s.Manager.Send(networkID, name, cmd.Content); err != nil {
		writeWSErr(ctx, c, cmd.ReqID, err.Error())
		return
	}
	writeWSAck(ctx, c, cmd.ReqID)
}

func (s *Server) cmdHistory(ctx context.Context, c *websocket.Conn, cmd clientCmd) {
	if cmd.BufferID == 0 {
		writeWSErr(ctx, c, cmd.ReqID, "history requires buffer_id")
		return
	}
	limit := cmd.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var (
		msgs []ircdb.StoredMessage
		err  error
	)
	if cmd.Before > 0 {
		msgs, err = ircdb.MessagesBefore(ctx, s.DB, cmd.BufferID, cmd.Before, limit)
	} else {
		msgs, err = ircdb.RecentMessages(ctx, s.DB, cmd.BufferID, limit)
	}
	if err != nil {
		writeWSErr(ctx, c, cmd.ReqID, err.Error())
		return
	}
	_ = wsjson.Write(ctx, c, historyResult{
		Type: "history_result", ReqID: cmd.ReqID,
		BufferID: cmd.BufferID, Messages: toMessageDTOs(msgs),
	})
}

func (s *Server) cmdJoin(ctx context.Context, c *websocket.Conn, cmd clientCmd) {
	// Minimal implementation: treat content as "<network_id> <channel>"
	// via the dedicated fields on the command. We reuse Content for the
	// channel name to avoid proliferating fields for v1.
	if cmd.BufferID == 0 && cmd.Content == "" {
		writeWSErr(ctx, c, cmd.ReqID, "join requires buffer_id (network status) and content (channel)")
		return
	}
	networkID, _, _, err := ircdb.LookupBuffer(ctx, s.DB, cmd.BufferID)
	if err != nil {
		writeWSErr(ctx, c, cmd.ReqID, "unknown network")
		return
	}
	if err := s.Manager.Join(networkID, cmd.Content); err != nil {
		writeWSErr(ctx, c, cmd.ReqID, err.Error())
		return
	}
	writeWSAck(ctx, c, cmd.ReqID)
}

func (s *Server) cmdPart(ctx context.Context, c *websocket.Conn, cmd clientCmd) {
	if cmd.BufferID == 0 {
		writeWSErr(ctx, c, cmd.ReqID, "part requires buffer_id")
		return
	}
	networkID, name, kind, err := ircdb.LookupBuffer(ctx, s.DB, cmd.BufferID)
	if err != nil || kind != ircdb.BufferChannel {
		writeWSErr(ctx, c, cmd.ReqID, "part only works on channel buffers")
		return
	}
	if err := s.Manager.Part(networkID, name, cmd.Content); err != nil {
		writeWSErr(ctx, c, cmd.ReqID, err.Error())
		return
	}
	writeWSAck(ctx, c, cmd.ReqID)
}

func writeWSAck(ctx context.Context, c *websocket.Conn, reqID string) {
	_ = wsjson.Write(ctx, c, ackEnvelope{Type: "ack", ReqID: reqID})
}

func writeWSErr(ctx context.Context, c *websocket.Conn, reqID, msg string) {
	_ = wsjson.Write(ctx, c, errorEnvelope{Type: "error", ReqID: reqID, Message: msg})
}

// Unused but exposed so json.Marshal errors in hub events surface in
// tests rather than silently dropping bytes on the wire.
var _ = json.Marshal
