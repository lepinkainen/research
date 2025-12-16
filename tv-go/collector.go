package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	APIBaseURL = "https://telkussa.fi/API"
	RateLimit  = 1 * time.Second
)

type TVProgram struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Episode     string `json:"episode"`
	Description string `json:"description"`
	Start       int64  `json:"start"`
	Stop        int64  `json:"stop"`
	SeriesID    int    `json:"series_id"`
	AgeLimit    int    `json:"agelimit"`
	Channel     int    `json:"channel"`
	Rating      int    `json:"rating"`
}

type APIChannel struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShowOrder int    `json:"showOrder"`
}

type TVCollector struct {
	app    *pocketbase.PocketBase
	client *http.Client
}

func NewTVCollector(app *pocketbase.PocketBase) *TVCollector {
	return &TVCollector{
		app: app,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *TVCollector) FetchAllPrograms(daysAhead int) error {
	// Get active channels
	channels := []*models.Record{}
	err := c.app.Dao().RecordQuery("channels").
		AndWhere(c.app.Dao().DB().NewExp("active = {:active}", map[string]any{"active": true})).
		OrderBy("show_order ASC").
		All(&channels)

	if err != nil {
		return fmt.Errorf("failed to fetch channels: %w", err)
	}

	log.Printf("📊 Fetching programs for %d active channels", len(channels))

	// Fetch programs for today + N days ahead
	today := time.Now()

	for dayOffset := 0; dayOffset <= daysAhead; dayOffset++ {
		targetDate := today.AddDate(0, 0, dayOffset)
		dateStr := targetDate.Format("20060102")

		log.Printf("📅 Fetching programs for %s", targetDate.Format("2006-01-02"))

		for _, channel := range channels {
			channelID := channel.Id
			channelName := channel.GetString("name")

			startTime := time.Now()

			// Fetch programs from API
			programs, err := c.fetchChannelPrograms(channelID, dateStr)
			duration := time.Since(startTime).Milliseconds()

			if err != nil {
				log.Printf("  ⚠️  %s: %v", channelName, err)
				c.logFetch(channelID, dateStr, false, 0, err.Error(), int(duration))
				continue
			}

			// Store programs
			stored := 0
			seriesMap := make(map[int]string)

			for _, prog := range programs {
				if err := c.storeProgram(prog, channelID); err != nil {
					log.Printf("    ⚠️  Failed to store program: %v", err)
				} else {
					stored++
				}

				// Track series
				if prog.SeriesID > 0 {
					seriesMap[prog.SeriesID] = prog.Name
				}
			}

			// Update series records
			for seriesID, name := range seriesMap {
				c.updateSeries(seriesID, name)
			}

			log.Printf("  ✅ %s: %d programs stored", channelName, stored)

			// Log success
			c.logFetch(channelID, dateStr, true, stored, "", int(duration))

			// Rate limiting
			time.Sleep(RateLimit)
		}
	}

	return nil
}

func (c *TVCollector) fetchChannelPrograms(channelID, date string) ([]TVProgram, error) {
	url := fmt.Sprintf("%s/Channel/%s/%s", APIBaseURL, channelID, date)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "fi-FI,fi;q=0.9,en;q=0.8")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var programs []TVProgram
	if err := json.Unmarshal(body, &programs); err != nil {
		return nil, err
	}

	return programs, nil
}

func (c *TVCollector) storeProgram(prog TVProgram, channelID string) error {
	collection, err := c.app.Dao().FindCollectionByNameOrId("programs")
	if err != nil {
		return err
	}

	programID := strconv.Itoa(prog.ID)

	// Check if program already exists
	existingRecord, _ := c.app.Dao().FindRecordById("programs", programID)

	var record *models.Record
	if existingRecord != nil {
		record = existingRecord
	} else {
		record = models.NewRecord(collection)
		record.SetId(programID)
	}

	// Convert timestamps
	// NOTE: The API returns custom timestamps that need conversion
	// For now, treating them as Unix timestamps
	// You may need to adjust this based on actual timestamp format
	startTime := types.DateTime{Time: time.Unix(prog.Start, 0)}
	endTime := types.DateTime{Time: time.Unix(prog.Stop, 0)}
	duration := (prog.Stop - prog.Start) / 60 // Convert to minutes

	record.Set("channel", channelID)
	record.Set("name", prog.Name)
	record.Set("episode", prog.Episode)
	record.Set("description", prog.Description)
	record.Set("start_time", startTime)
	record.Set("end_time", endTime)
	record.Set("duration", duration)
	record.Set("age_limit", prog.AgeLimit)
	record.Set("rating", prog.Rating)
	record.Set("is_series", prog.SeriesID > 0)

	if prog.SeriesID > 0 {
		record.Set("series", strconv.Itoa(prog.SeriesID))
	}

	return c.app.Dao().SaveRecord(record)
}

func (c *TVCollector) updateSeries(seriesID int, name string) error {
	collection, err := c.app.Dao().FindCollectionByNameOrId("series")
	if err != nil {
		return err
	}

	seriesIDStr := strconv.Itoa(seriesID)

	// Check if series exists
	existingRecord, _ := c.app.Dao().FindRecordById("series", seriesIDStr)

	var record *models.Record
	if existingRecord != nil {
		record = existingRecord
	} else {
		record = models.NewRecord(collection)
		record.SetId(seriesIDStr)
		record.Set("name", name)
		record.Set("active", true)
		record.Set("first_seen", types.DateTime{Time: time.Now()})
		record.Set("episode_count", 0)
	}

	// Update last_seen
	record.Set("last_seen", types.DateTime{Time: time.Now()})

	return c.app.Dao().SaveRecord(record)
}

func (c *TVCollector) logFetch(channelID, targetDate string, success bool, count int, errorMsg string, durationMs int) error {
	collection, err := c.app.Dao().FindCollectionByNameOrId("fetch_logs")
	if err != nil {
		return err
	}

	record := models.NewRecord(collection)

	if channelID != "" {
		record.Set("channel", channelID)
	}
	record.Set("target_date", targetDate)
	record.Set("success", success)
	record.Set("programs_count", count)
	record.Set("error_message", errorMsg)
	record.Set("duration_ms", durationMs)

	return c.app.Dao().SaveRecord(record)
}

func (c *TVCollector) UpdateChannelList() error {
	url := fmt.Sprintf("%s/Channels", APIBaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var channels []APIChannel
	if err := json.Unmarshal(body, &channels); err != nil {
		return err
	}

	log.Printf("📡 Found %d channels in API", len(channels))

	collection, err := c.app.Dao().FindCollectionByNameOrId("channels")
	if err != nil {
		return err
	}

	for _, ch := range channels {
		channelID := strconv.Itoa(ch.ID)

		existingRecord, _ := c.app.Dao().FindRecordById("channels", channelID)

		var record *models.Record
		if existingRecord != nil {
			record = existingRecord
		} else {
			record = models.NewRecord(collection)
			record.SetId(channelID)
			record.Set("active", true) // New channels are active by default
		}

		record.Set("name", ch.Name)
		record.Set("show_order", ch.ShowOrder)

		if err := c.app.Dao().SaveRecord(record); err != nil {
			log.Printf("  ⚠️  Failed to save channel %s: %v", ch.Name, err)
		}
	}

	log.Printf("✅ Channel list updated")
	return nil
}
