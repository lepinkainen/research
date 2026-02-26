// Package watchlist fetches Finnish TV programs for today and filters them
// against a user-defined list of shows.
//
// It queries the telkussa.fi public API directly; no PocketBase instance is
// required. The watchlist is loaded from a JSON config file:
//
//	{
//	  "shows": ["Uutiset", "Formula 1", "Dark"]
//	}
//
// Usage:
//
//	cfg, err := watchlist.LoadConfig("watchlist.json")
//	matches, err := watchlist.NewClient().TodayMatches(cfg)
//	for _, m := range matches {
//	    fmt.Printf("%s  %s  %s\n", m.StartTime.Format("15:04"), m.Channel.Name, m.Program.Name)
//	}
package watchlist

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const apiBase = "https://telkussa.fi/API"

// Config holds the list of show names to watch for.
type Config struct {
	Shows []string `json:"shows"`
}

// LoadConfig reads a JSON config file and returns a Config.
// The file must contain an object with a "shows" key holding an array of strings.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Program is a single TV program as returned by the telkussa.fi API.
type Program struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Episode     string `json:"episode"`
	Description string `json:"description"`
	Start       int64  `json:"start"`
	Stop        int64  `json:"stop"`
	SeriesID    int    `json:"series_id"`
	AgeLimit    int    `json:"agelimit"`
}

// Channel is a TV channel as returned by the telkussa.fi API.
type Channel struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShowOrder int    `json:"showOrder"`
}

// Match is a program airing today whose name matched an entry in the watchlist.
type Match struct {
	Program     Program
	Channel     Channel
	StartTime   time.Time
	EndTime     time.Time
	MatchedShow string // the watchlist entry that triggered the match
}

// Client fetches TV schedule data from the telkussa.fi API.
type Client struct {
	http     *http.Client
	rateWait time.Duration
}

// NewClient returns a Client with sensible defaults (15 s timeout, 300 ms rate wait).
func NewClient() *Client {
	return &Client{
		http:     &http.Client{Timeout: 15 * time.Second},
		rateWait: 300 * time.Millisecond,
	}
}

// TodayMatches fetches all channels and their programs for today, then returns
// every program whose name contains any of the show names in cfg (case-insensitive
// substring match). Results are sorted by start time.
func (c *Client) TodayMatches(cfg *Config) ([]Match, error) {
	channels, err := c.fetchChannels()
	if err != nil {
		return nil, fmt.Errorf("fetch channels: %w", err)
	}

	date := time.Now().Format("20060102")
	var matches []Match

	for _, ch := range channels {
		programs, err := c.fetchPrograms(ch.ID, date)
		if err != nil {
			// Individual channels can fail (404, empty data) — skip silently.
			continue
		}

		for _, prog := range programs {
			if matched, show := cfg.matchedBy(prog.Name); matched {
				matches = append(matches, Match{
					Program:     prog,
					Channel:     ch,
					StartTime:   time.Unix(prog.Start, 0),
					EndTime:     time.Unix(prog.Stop, 0),
					MatchedShow: show,
				})
			}
		}

		time.Sleep(c.rateWait)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].StartTime.Before(matches[j].StartTime)
	})

	return matches, nil
}

// matchedBy reports whether programName contains any watchlist entry (case-insensitive).
// Returns the matched entry if found.
func (cfg *Config) matchedBy(programName string) (bool, string) {
	lower := strings.ToLower(programName)
	for _, show := range cfg.Shows {
		if strings.Contains(lower, strings.ToLower(show)) {
			return true, show
		}
	}
	return false, ""
}

func (c *Client) fetchChannels() ([]Channel, error) {
	body, err := c.get(apiBase + "/Channels")
	if err != nil {
		return nil, err
	}
	var channels []Channel
	if err := json.Unmarshal(body, &channels); err != nil {
		return nil, err
	}
	return channels, nil
}

func (c *Client) fetchPrograms(channelID int, date string) ([]Program, error) {
	url := fmt.Sprintf("%s/Channel/%d/%s", apiBase, channelID, date)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}
	var programs []Program
	if err := json.Unmarshal(body, &programs); err != nil {
		return nil, err
	}
	return programs, nil
}

func (c *Client) get(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "fi-FI,fi;q=0.9,en;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}
