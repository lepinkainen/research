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
// calling Send.
type fakeIRCServer struct {
	t    *testing.T
	ln   net.Listener
	conn net.Conn
	rd   *bufio.Reader
	wr   *bufio.Writer
	ready chan struct{}
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
			// Announce no caps so girc skips request phase and sends CAP END.
			s.send(":fake CAP * LS :")
		case strings.HasPrefix(line, "CAP REQ"):
			s.send(":fake CAP * NAK :")
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
	host, port := srv.addr()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := NewManager(store)
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
