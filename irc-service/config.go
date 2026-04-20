package main

import (
	"os"
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
