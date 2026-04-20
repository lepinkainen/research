package irc

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lepinkainen/research/irc-service/db"
)

// fakeIRCServer is a minimal ircd that accepts one client, completes the
// USER/NICK handshake, and then lets tests drive the conversation by
// calling Send. The advertised caps are controlled by the `caps` field.
type fakeIRCServer struct {
	t     *testing.T
	ln    net.Listener
	conn  net.Conn
	rd    *bufio.Reader
	wr    *bufio.Writer
	ready chan struct{}
	caps  string // space-separated caps to advertise in CAP LS
}

func newFakeIRC(t *testing.T) *fakeIRCServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeIRCServer{t: t, ln: ln, ready: make(chan struct{})}
	go s.loop()
	return s
}

func (s *fakeIRCServer) addr() (host string, port int) {
	a := s.ln.Addr().(*net.TCPAddr)
	return a.IP.String(), a.Port
}

func (s *fakeIRCServer) loop() {
	c, err := s.ln.Accept()
	if err != nil {
		return
	}
	s.conn = c
	s.rd = bufio.NewReader(c)
	s.wr = bufio.NewWriter(c)

	// Walk the client through CAP LS -> CAP END -> NICK/USER -> 001..376.
	nick := ""
	welcomed := false
	for {
		line, err := s.rd.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "CAP LS"):
			s.send(":fake CAP * LS :%s", s.caps)
		case strings.HasPrefix(line, "CAP REQ"):
			// ACK everything the client asked for. The payload starts
			// after the trailing ':'.
			reqIdx := strings.Index(line, ":")
			req := ""
			if reqIdx >= 0 {
				req = line[reqIdx+1:]
			}
			s.send(":fake CAP * ACK :%s", req)
		case strings.HasPrefix(line, "CAP END"):
			// nothing to do
		case strings.HasPrefix(line, "NICK "):
			nick = strings.TrimPrefix(line, "NICK ")
		case strings.HasPrefix(line, "USER "):
			if nick == "" {
				nick = "tester"
			}
			if !welcomed {
				welcomed = true
				s.send(":fake 001 %s :Welcome", nick)
				s.send(":fake 002 %s :Your host", nick)
				s.send(":fake 003 %s :This server was created", nick)
				s.send(":fake 004 %s fake 1.0 o o", nick)
				s.send(":fake 005 %s CHANTYPES=# :are supported", nick)
				s.send(":fake 376 %s :End of MOTD", nick)
				close(s.ready)
			}
		case strings.HasPrefix(line, "JOIN "):
			channel := strings.TrimPrefix(line, "JOIN ")
			s.send(":%s!~u@h JOIN %s", nick, channel)
			s.send(":fake 353 %s = %s :%s", nick, channel, nick)
			s.send(":fake 366 %s %s :End of NAMES", nick, channel)
		case strings.HasPrefix(line, "PING "):
			s.send(":fake PONG fake %s", strings.TrimPrefix(line, "PING "))
		case strings.HasPrefix(line, "QUIT"):
			return
		}
	}
}

func (s *fakeIRCServer) send(format string, args ...any) {
	if s.wr == nil {
		return
	}
	fmt.Fprintf(s.wr, format+"\r\n", args...)
	s.wr.Flush()
}

func (s *fakeIRCServer) injectPrivmsg(channel, from, text string) {
	s.send(":%s!~u@h PRIVMSG %s :%s", from, channel, text)
}

// injectTaggedPrivmsg sends a PRIVMSG with IRCv3 message-tags prefix.
// tags should be the `key=value;...` payload, without the leading '@'.
func (s *fakeIRCServer) injectTaggedPrivmsg(tags, channel, from, text string) {
	s.send("@%s :%s!~u@h PRIVMSG %s :%s", tags, from, channel, text)
}

func (s *fakeIRCServer) close() { s.ln.Close(); if s.conn != nil { s.conn.Close() } }

// TestManagerPersistsMessages spins up a fake ircd, runs the manager
// against it, injects a PRIVMSG, and asserts the message lands in SQLite
// with the right buffer and kind.
func TestManagerPersistsMessages(t *testing.T) {
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	srv := newFakeIRC(t)
	defer srv.close()
	// no caps advertised for the basic test
	host, port := srv.addr()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := NewManager(store, nil)
	err = mgr.Start(ctx, []NetworkConfig{{
		Name:     "fake",
		Host:     host,
		Port:     port,
		TLS:      false,
		Nick:     "tester",
		User:     "tester",
		Realname: "tester",
		Channels: []string{"#test"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	// Wait for registration + autojoin to complete.
	select {
	case <-srv.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server never saw registration")
	}
	// Small wait for JOIN to round-trip and handlers to fire.
	waitFor(t, 5*time.Second, func() bool {
		row := store.QueryRow(`SELECT COUNT(*) FROM buffers WHERE name='#test'`)
		var n int
		row.Scan(&n)
		return n == 1
	}, "join never landed")

	srv.injectPrivmsg("#test", "alice", "hello from fake")

	waitFor(t, 5*time.Second, func() bool {
		var n int
		store.QueryRow(
			`SELECT COUNT(*) FROM messages WHERE kind='privmsg' AND content='hello from fake'`,
		).Scan(&n)
		return n == 1
	}, "privmsg never persisted")

	// FTS should index it too.
	var hit string
	err = store.QueryRow(
		`SELECT content FROM messages_fts WHERE messages_fts MATCH 'fake' LIMIT 1`,
	).Scan(&hit)
	if err != nil || !strings.Contains(hit, "hello from fake") {
		t.Fatalf("fts hit = %q, err = %v", hit, err)
	}

	cancel()
	mgr.Wait()
}

// TestMsgIDDedupAndServerTime verifies that when the server advertises
// message-tags + server-time + msgid, a tagged PRIVMSG is stored with the
// server-supplied timestamp and a repeated msgid is rejected by the
// partial unique index (INSERT OR IGNORE path in InsertMessage).
func TestMsgIDDedupAndServerTime(t *testing.T) {
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	srv := newFakeIRC(t)
	srv.caps = "message-tags server-time msgid batch"
	defer srv.close()
	host, port := srv.addr()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := NewManager(store, nil)
	err = mgr.Start(ctx, []NetworkConfig{{
		Name: "fake", Host: host, Port: port, TLS: false,
		Nick: "tester", User: "tester", Realname: "tester",
		Channels: []string{"#test"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-srv.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server never saw registration")
	}
	waitFor(t, 5*time.Second, func() bool {
		var n int
		store.QueryRow(`SELECT COUNT(*) FROM buffers WHERE name='#test'`).Scan(&n)
		return n == 1
	}, "join never landed")

	const msgid = "mid-42"
	const serverTime = "2025-01-02T03:04:05.678Z"
	tags := "time=" + serverTime + ";msgid=" + msgid
	srv.injectTaggedPrivmsg(tags, "#test", "alice", "once")

	waitFor(t, 5*time.Second, func() bool {
		var n int
		store.QueryRow(
			`SELECT COUNT(*) FROM messages WHERE msgid=?`, msgid,
		).Scan(&n)
		return n == 1
	}, "tagged privmsg never landed")

	// Replaying the exact same tagged line must not create a second row.
	srv.injectTaggedPrivmsg(tags, "#test", "alice", "once")
	time.Sleep(250 * time.Millisecond)

	var count int
	if err := store.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE msgid=?`, msgid,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected dedup via msgid, got %d rows", count)
	}

	// The stored ts must come from the tag, not local receive time.
	var gotTS string
	if err := store.QueryRow(
		`SELECT ts FROM messages WHERE msgid=?`, msgid,
	).Scan(&gotTS); err != nil {
		t.Fatal(err)
	}
	// Formatter may pad/truncate fractional seconds; compare the instant.
	gotParsed, err := time.Parse("2006-01-02T15:04:05.000Z", gotTS)
	if err != nil {
		t.Fatalf("unparseable ts %q: %v", gotTS, err)
	}
	want, _ := time.Parse(time.RFC3339Nano, serverTime)
	if !gotParsed.Equal(want) {
		t.Fatalf("ts mismatch: got %s, want %s", gotParsed, want)
	}

	cancel()
	mgr.Wait()
}

func waitFor(t *testing.T, d time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out: %s", msg)
}

// TestMain silences slog noise during tests.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
