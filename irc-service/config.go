package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/lepinkainen/research/irc-service/irc"
)

type Config struct {
	DBPath string
	Addr   string
}

func loadConfig() Config {
	return Config{
		DBPath: envOr("DB_PATH", "./data/irc.db"),
		Addr:   envOr("ADDR", ":8080"),
	}
}

// seedNetworksFromEnv returns at most one network configured via env vars.
// Intended as a stopgap until M4 adds the network CRUD API. If IRC_NETWORK
// is empty, no networks are seeded and the service just idles on HTTP.
func seedNetworksFromEnv() []irc.NetworkConfig {
	name := os.Getenv("IRC_NETWORK")
	if name == "" {
		return nil
	}
	port, _ := strconv.Atoi(envOr("IRC_PORT", "6697"))
	useTLS := strings.EqualFold(envOr("IRC_TLS", "true"), "true")
	nick := envOr("IRC_NICK", "ircsvc")
	channels := splitCSV(os.Getenv("IRC_CHANNELS"))
	return []irc.NetworkConfig{{
		Name:     name,
		Host:     envOr("IRC_SERVER", ""),
		Port:     port,
		TLS:      useTLS,
		Nick:     nick,
		User:     envOr("IRC_USER", nick),
		Realname: envOr("IRC_NAME", nick),
		Channels: channels,
	}}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
