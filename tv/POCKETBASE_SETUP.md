# PocketBase Setup Guide for TV Program Collector

**Project:** Finnish TV Program Data Collector
**Backend:** PocketBase with Go Job Scheduling
**Version:** 1.0
**Date:** 2025-12-16

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Database Schema Setup](#database-schema-setup)
4. [Job Scheduler Implementation](#job-scheduler-implementation)
5. [Configuration](#configuration)
6. [Testing](#testing)
7. [Deployment](#deployment)
8. [Monitoring](#monitoring)

---

## Prerequisites

### Required Software

- **Go:** 1.21+ (for extending PocketBase)
- **Git:** For version control
- **Python:** 3.9+ (for data collection scripts)

### System Requirements

- **OS:** Linux, macOS, or Windows
- **RAM:** 512MB minimum, 1GB recommended
- **Disk:** 10GB minimum (for program data storage)
- **Network:** Stable internet connection for API calls

---

## Installation

### Step 1: Install PocketBase

```bash
# Create project directory
mkdir -p ~/tv-pocketbase
cd ~/tv-pocketbase

# Download PocketBase (Linux AMD64)
wget https://github.com/pocketbase/pocketbase/releases/download/v0.22.0/pocketbase_0.22.0_linux_amd64.zip

# Extract
unzip pocketbase_0.22.0_linux_amd64.zip

# Clean up
rm pocketbase_0.22.0_linux_amd64.zip

# Make executable
chmod +x pocketbase
```

### Step 2: Initialize PocketBase

```bash
# Run PocketBase to initialize database
./pocketbase serve

# Access admin UI at: http://127.0.0.1:8090/_/
```

### Step 3: Create Admin Account

Visit http://127.0.0.1:8090/_/ and create your admin account.

Or use CLI:

```bash
./pocketbase admin create
```

---

## Database Schema Setup

### Option 1: Manual Setup (via Admin UI)

1. Go to http://127.0.0.1:8090/_/
2. Navigate to **Collections**
3. Create collections as defined in `POCKETBASE_SCHEMA.md`

### Option 2: Programmatic Setup (Recommended)

Create a setup script to initialize collections:

**File:** `setup_collections.go`

```go
package main

import (
    "log"
    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/models"
    "github.com/pocketbase/pocketbase/models/schema"
)

func setupCollections(app *pocketbase.PocketBase) error {
    // Create channels collection
    channelsCollection := &models.Collection{}
    channelsCollection.Name = "channels"
    channelsCollection.Type = models.CollectionTypeBase
    channelsCollection.System = false
    channelsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "name",
        Type:     schema.FieldTypeText,
        Required: true,
        Options: &schema.TextOptions{
            Max: 100,
        },
    })
    channelsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "show_order",
        Type:     schema.FieldTypeNumber,
        Required: true,
        Options: &schema.NumberOptions{
            Min: 0,
        },
    })
    channelsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "category",
        Type:     schema.FieldTypeSelect,
        Required: false,
        Options: &schema.SelectOptions{
            MaxSelect: 1,
            Values: []string{
                "public", "commercial", "sports", "movies",
                "kids", "music", "international", "documentary", "other",
            },
        },
    })
    channelsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "logo_url",
        Type:     schema.FieldTypeUrl,
        Required: false,
    })
    channelsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "active",
        Type:     schema.FieldTypeBool,
        Required: true,
        Options: &schema.BoolOptions{},
    })

    channelsCollection.ListRule = types.Pointer("active = true")
    channelsCollection.ViewRule = types.Pointer("")

    if err := app.Dao().SaveCollection(channelsCollection); err != nil {
        return err
    }

    // Create series collection
    seriesCollection := &models.Collection{}
    seriesCollection.Name = "series"
    seriesCollection.Type = models.CollectionTypeBase
    seriesCollection.System = false
    seriesCollection.Schema.AddField(&schema.SchemaField{
        Name:     "name",
        Type:     schema.FieldTypeText,
        Required: true,
        Options: &schema.TextOptions{
            Max: 200,
        },
    })
    seriesCollection.Schema.AddField(&schema.SchemaField{
        Name:     "description",
        Type:     schema.FieldTypeEditor,
        Required: false,
        Options: &schema.EditorOptions{
            MaxSize: 5000,
        },
    })
    seriesCollection.Schema.AddField(&schema.SchemaField{
        Name:     "episode_count",
        Type:     schema.FieldTypeNumber,
        Required: false,
        Options: &schema.NumberOptions{
            Min: 0,
        },
    })
    seriesCollection.Schema.AddField(&schema.SchemaField{
        Name:     "active",
        Type:     schema.FieldTypeBool,
        Required: true,
        Options: &schema.BoolOptions{},
    })

    seriesCollection.ListRule = types.Pointer("")
    seriesCollection.ViewRule = types.Pointer("")

    if err := app.Dao().SaveCollection(seriesCollection); err != nil {
        return err
    }

    // Create programs collection
    programsCollection := &models.Collection{}
    programsCollection.Name = "programs"
    programsCollection.Type = models.CollectionTypeBase
    programsCollection.System = false
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "channel",
        Type:     schema.FieldTypeRelation,
        Required: true,
        Options: &schema.RelationOptions{
            CollectionId: channelsCollection.Id,
            MaxSelect:    types.Pointer(1),
            CascadeDelete: true,
        },
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "name",
        Type:     schema.FieldTypeText,
        Required: true,
        Options: &schema.TextOptions{
            Max: 200,
        },
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "episode",
        Type:     schema.FieldTypeText,
        Required: false,
        Options: &schema.TextOptions{
            Max: 100,
        },
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "description",
        Type:     schema.FieldTypeEditor,
        Required: false,
        Options: &schema.EditorOptions{
            MaxSize: 5000,
        },
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "start_time",
        Type:     schema.FieldTypeDate,
        Required: true,
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "end_time",
        Type:     schema.FieldTypeDate,
        Required: true,
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "duration",
        Type:     schema.FieldTypeNumber,
        Required: true,
        Options: &schema.NumberOptions{
            Min: 0,
        },
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "series",
        Type:     schema.FieldTypeRelation,
        Required: false,
        Options: &schema.RelationOptions{
            CollectionId: seriesCollection.Id,
            MaxSelect:    types.Pointer(1),
        },
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "age_limit",
        Type:     schema.FieldTypeNumber,
        Required: true,
        Options: &schema.NumberOptions{
            Min: 0,
        },
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "rating",
        Type:     schema.FieldTypeNumber,
        Required: false,
        Options: &schema.NumberOptions{
            Min: 0,
        },
    })
    programsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "is_series",
        Type:     schema.FieldTypeBool,
        Required: true,
        Options: &schema.BoolOptions{},
    })

    programsCollection.ListRule = types.Pointer("")
    programsCollection.ViewRule = types.Pointer("")

    if err := app.Dao().SaveCollection(programsCollection); err != nil {
        return err
    }

    // Create fetch_logs collection
    logsCollection := &models.Collection{}
    logsCollection.Name = "fetch_logs"
    logsCollection.Type = models.CollectionTypeBase
    logsCollection.System = false
    logsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "channel",
        Type:     schema.FieldTypeRelation,
        Required: false,
        Options: &schema.RelationOptions{
            CollectionId: channelsCollection.Id,
            MaxSelect:    types.Pointer(1),
        },
    })
    logsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "target_date",
        Type:     schema.FieldTypeText,
        Required: true,
        Options: &schema.TextOptions{
            Max: 8, // YYYYMMDD
        },
    })
    logsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "success",
        Type:     schema.FieldTypeBool,
        Required: true,
        Options: &schema.BoolOptions{},
    })
    logsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "programs_count",
        Type:     schema.FieldTypeNumber,
        Required: false,
        Options: &schema.NumberOptions{
            Min: 0,
        },
    })
    logsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "error_message",
        Type:     schema.FieldTypeText,
        Required: false,
        Options: &schema.TextOptions{
            Max: 500,
        },
    })
    logsCollection.Schema.AddField(&schema.SchemaField{
        Name:     "duration_ms",
        Type:     schema.FieldTypeNumber,
        Required: false,
        Options: &schema.NumberOptions{
            Min: 0,
        },
    })

    if err := app.Dao().SaveCollection(logsCollection); err != nil {
        return err
    }

    log.Println("✅ All collections created successfully")
    return nil
}
```

---

## Job Scheduler Implementation

PocketBase includes built-in job scheduling using Go's cron syntax. We'll use this to fetch TV program data every night at 01:00.

### Step 1: Create Main Application

**File:** `main.go`

```go
package main

import (
    "log"
    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/core"
    "github.com/pocketbase/pocketbase/plugins/cron"
)

func main() {
    app := pocketbase.New()

    // Setup collections on first run
    app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
        // Check if collections exist, if not create them
        if _, err := app.Dao().FindCollectionByNameOrId("channels"); err != nil {
            log.Println("Setting up collections...")
            if err := setupCollections(app); err != nil {
                return err
            }
        }

        return nil
    })

    // Register job scheduler
    app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
        scheduler := cron.New()

        // Job 1: Fetch TV program data daily at 01:00
        scheduler.MustAdd("fetch_programs", "0 1 * * *", func() {
            log.Println("🔄 Starting nightly program data fetch...")
            if err := fetchTVPrograms(app); err != nil {
                log.Printf("❌ Program fetch failed: %v", err)
            } else {
                log.Println("✅ Program fetch completed successfully")
            }
        })

        // Job 2: Clean up old programs daily at 02:00
        scheduler.MustAdd("cleanup_old_data", "0 2 * * *", func() {
            log.Println("🧹 Starting data cleanup...")
            if err := cleanupOldData(app); err != nil {
                log.Printf("❌ Cleanup failed: %v", err)
            } else {
                log.Println("✅ Cleanup completed successfully")
            }
        })

        // Job 3: Update channel list weekly (Sunday at 03:00)
        scheduler.MustAdd("update_channels", "0 3 * * 0", func() {
            log.Println("📡 Updating channel list...")
            if err := updateChannelList(app); err != nil {
                log.Printf("❌ Channel update failed: %v", err)
            } else {
                log.Println("✅ Channel list updated successfully")
            }
        })

        scheduler.Start()

        log.Println("✅ Job scheduler started:")
        log.Println("   - fetch_programs: Daily at 01:00")
        log.Println("   - cleanup_old_data: Daily at 02:00")
        log.Println("   - update_channels: Weekly on Sunday at 03:00")

        return nil
    })

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### Step 2: Implement Data Fetcher

**File:** `fetcher.go`

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"
    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/models"
)

const (
    APIBaseURL = "https://telkussa.fi/API"
    DaysAhead  = 7
    RateLimit  = 1 * time.Second
)

type Program struct {
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

type Channel struct {
    ID        int    `json:"id"`
    Name      string `json:"name"`
    ShowOrder int    `json:"showOrder"`
}

func fetchTVPrograms(app *pocketbase.PocketBase) error {
    // Get active channels
    channels := []models.Record{}
    if err := app.Dao().RecordQuery("channels").
        Where("active = true").
        All(&channels); err != nil {
        return fmt.Errorf("failed to fetch channels: %w", err)
    }

    log.Printf("📊 Fetching programs for %d active channels", len(channels))

    // Fetch programs for today + N days ahead
    today := time.Now()

    for dayOffset := 0; dayOffset <= DaysAhead; dayOffset++ {
        targetDate := today.AddDate(0, 0, dayOffset)
        dateStr := targetDate.Format("20060102") // YYYYMMDD

        log.Printf("📅 Fetching programs for %s", targetDate.Format("2006-01-02"))

        for _, channel := range channels {
            channelID := channel.GetString("id")
            channelName := channel.GetString("name")

            startTime := time.Now()

            // Fetch programs from API
            programs, err := fetchChannelPrograms(channelID, dateStr)

            duration := time.Since(startTime).Milliseconds()

            if err != nil {
                log.Printf("  ⚠️  %s: %v", channelName, err)

                // Log failure
                logRecord := models.NewRecord(app.Dao().FindCollectionByNameOrId("fetch_logs"))
                logRecord.Set("channel", channelID)
                logRecord.Set("target_date", dateStr)
                logRecord.Set("success", false)
                logRecord.Set("programs_count", 0)
                logRecord.Set("error_message", err.Error())
                logRecord.Set("duration_ms", duration)
                app.Dao().SaveRecord(logRecord)

                continue
            }

            // Store programs
            stored := 0
            seriesMap := make(map[int]bool)

            for _, prog := range programs {
                if err := storeProgram(app, prog, channelID); err != nil {
                    log.Printf("    ⚠️  Failed to store program: %v", err)
                } else {
                    stored++
                }

                // Track series
                if prog.SeriesID > 0 {
                    seriesMap[prog.SeriesID] = true
                }
            }

            // Update series records
            for seriesID := range seriesMap {
                updateSeries(app, seriesID, programs)
            }

            log.Printf("  ✅ %s: %d programs stored", channelName, stored)

            // Log success
            logRecord := models.NewRecord(app.Dao().FindCollectionByNameOrId("fetch_logs"))
            logRecord.Set("channel", channelID)
            logRecord.Set("target_date", dateStr)
            logRecord.Set("success", true)
            logRecord.Set("programs_count", stored)
            logRecord.Set("duration_ms", duration)
            app.Dao().SaveRecord(logRecord)

            // Rate limiting
            time.Sleep(RateLimit)
        }
    }

    return nil
}

func fetchChannelPrograms(channelID, date string) ([]Program, error) {
    url := fmt.Sprintf("%s/Channel/%s/%s", APIBaseURL, channelID, date)

    client := &http.Client{
        Timeout: 15 * time.Second,
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
    req.Header.Set("Accept", "application/json")

    resp, err := client.Do(req)
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

    var programs []Program
    if err := json.Unmarshal(body, &programs); err != nil {
        return nil, err
    }

    return programs, nil
}

func storeProgram(app *pocketbase.PocketBase, prog Program, channelID string) error {
    collection, err := app.Dao().FindCollectionByNameOrId("programs")
    if err != nil {
        return err
    }

    // Check if program already exists
    existing, _ := app.Dao().FindFirstRecordByData("programs", "id", fmt.Sprintf("%d", prog.ID))

    var record *models.Record
    if existing != nil {
        record = existing
    } else {
        record = models.NewRecord(collection)
        record.Set("id", fmt.Sprintf("%d", prog.ID))
    }

    // Convert timestamps (assuming they're in a custom format)
    // You may need to adjust this based on the actual timestamp format
    startTime := time.Unix(prog.Start, 0).Format(time.RFC3339)
    endTime := time.Unix(prog.Stop, 0).Format(time.RFC3339)
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
        record.Set("series", fmt.Sprintf("%d", prog.SeriesID))
    }

    return app.Dao().SaveRecord(record)
}

func updateSeries(app *pocketbase.PocketBase, seriesID int, programs []Program) error {
    seriesIDStr := fmt.Sprintf("%d", seriesID)

    // Check if series exists
    existing, _ := app.Dao().FindFirstRecordByData("series", "id", seriesIDStr)

    var record *models.Record
    if existing != nil {
        record = existing
    } else {
        collection, err := app.Dao().FindCollectionByNameOrId("series")
        if err != nil {
            return err
        }
        record = models.NewRecord(collection)
        record.Set("id", seriesIDStr)

        // Find first program with this series_id to get name
        for _, prog := range programs {
            if prog.SeriesID == seriesID {
                record.Set("name", prog.Name)
                break
            }
        }

        record.Set("active", true)
    }

    // Update last_seen
    record.Set("last_seen", time.Now().Format(time.RFC3339))

    return app.Dao().SaveRecord(record)
}

func updateChannelList(app *pocketbase.PocketBase) error {
    url := fmt.Sprintf("%s/Channels", APIBaseURL)

    client := &http.Client{
        Timeout: 15 * time.Second,
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }

    req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
    req.Header.Set("Accept", "application/json")

    resp, err := client.Do(req)
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

    var channels []Channel
    if err := json.Unmarshal(body, &channels); err != nil {
        return err
    }

    log.Printf("📡 Found %d channels in API", len(channels))

    collection, err := app.Dao().FindCollectionByNameOrId("channels")
    if err != nil {
        return err
    }

    for _, ch := range channels {
        channelID := fmt.Sprintf("%d", ch.ID)

        existing, _ := app.Dao().FindFirstRecordByData("channels", "id", channelID)

        var record *models.Record
        if existing != nil {
            record = existing
        } else {
            record = models.NewRecord(collection)
            record.Set("id", channelID)
            record.Set("active", true) // New channels are active by default
        }

        record.Set("name", ch.Name)
        record.Set("show_order", ch.ShowOrder)

        if err := app.Dao().SaveRecord(record); err != nil {
            log.Printf("  ⚠️  Failed to save channel %s: %v", ch.Name, err)
        }
    }

    log.Printf("✅ Channel list updated")
    return nil
}

func cleanupOldData(app *pocketbase.PocketBase) error {
    // Delete programs older than 30 days
    cutoffDate := time.Now().AddDate(0, 0, -30).Format(time.RFC3339)

    result, err := app.Dao().DB().
        NewQuery("DELETE FROM programs WHERE start_time < {:cutoff}").
        Bind(map[string]any{
            "cutoff": cutoffDate,
        }).
        Execute()

    if err != nil {
        return err
    }

    rowsAffected, _ := result.RowsAffected()
    log.Printf("🗑️  Deleted %d old programs", rowsAffected)

    // Delete fetch logs older than 30 days
    logCutoff := time.Now().AddDate(0, 0, -30).Format(time.RFC3339)

    result, err = app.Dao().DB().
        NewQuery("DELETE FROM fetch_logs WHERE created < {:cutoff}").
        Bind(map[string]any{
            "cutoff": logCutoff,
        }).
        Execute()

    if err != nil {
        return err
    }

    rowsAffected, _ = result.RowsAffected()
    log.Printf("🗑️  Deleted %d old fetch logs", rowsAffected)

    return nil
}
```

### Step 3: Create Go Module

**File:** `go.mod`

```go
module tv-pocketbase

go 1.21

require github.com/pocketbase/pocketbase v0.22.0
```

### Step 4: Build and Run

```bash
# Initialize Go module
go mod init tv-pocketbase

# Download dependencies
go mod tidy

# Build the application
go build -o tv-pocketbase

# Run
./tv-pocketbase serve
```

---

## Configuration

### Environment Variables

Create `.env` file:

```bash
# PocketBase Settings
PB_DATA_DIR=./pb_data
PB_PUBLIC_DIR=./pb_public

# Server Settings
PB_BIND=127.0.0.1:8090

# API Settings
TELKUSSA_API_URL=https://telkussa.fi/API
FETCH_DAYS_AHEAD=7
RATE_LIMIT_SECONDS=1

# Data Retention
KEEP_PROGRAMS_DAYS=30
KEEP_LOGS_DAYS=30
```

### Cron Schedule Syntax

```
* * * * *
│ │ │ │ │
│ │ │ │ └─── Day of week (0-7, 0 or 7 is Sunday)
│ │ │ └───── Month (1-12)
│ │ └─────── Day of month (1-31)
│ └───────── Hour (0-23)
└─────────── Minute (0-59)
```

**Examples:**
- `0 1 * * *` - Daily at 01:00
- `0 */6 * * *` - Every 6 hours
- `0 3 * * 0` - Weekly on Sunday at 03:00
- `*/30 * * * *` - Every 30 minutes

---

## Testing

### Test Job Scheduler

Temporarily change cron schedule for testing:

```go
// Instead of: "0 1 * * *"
// Use: "*/2 * * * *" (every 2 minutes)
scheduler.MustAdd("fetch_programs", "*/2 * * * *", func() {
    // ...
})
```

### Manual Trigger

Add API endpoints to trigger jobs manually:

```go
app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
    e.Router.POST("/api/trigger/fetch", func(c echo.Context) error {
        // Check auth
        admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
        if admin == nil {
            return apis.NewForbiddenError("Admin auth required", nil)
        }

        go fetchTVPrograms(app)

        return c.JSON(200, map[string]string{
            "message": "Fetch job triggered",
        })
    })

    return nil
})
```

Test with curl:

```bash
curl -X POST http://127.0.0.1:8090/api/trigger/fetch \
  -H "Authorization: Admin YOUR_ADMIN_TOKEN"
```

---

## Deployment

### Systemd Service

Create service file: `/etc/systemd/system/tv-pocketbase.service`

```ini
[Unit]
Description=TV PocketBase Service
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/tv-pocketbase
ExecStart=/opt/tv-pocketbase/tv-pocketbase serve --http=127.0.0.1:8090
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable tv-pocketbase
sudo systemctl start tv-pocketbase
sudo systemctl status tv-pocketbase
```

### Nginx Reverse Proxy

```nginx
server {
    listen 80;
    server_name tv.example.com;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## Monitoring

### Logs

View PocketBase logs:

```bash
# Systemd service logs
sudo journalctl -u tv-pocketbase -f

# Application logs
tail -f pb_data/logs/*.log
```

### Health Check

Create health check endpoint:

```go
app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
    e.Router.GET("/health", func(c echo.Context) error {
        return c.JSON(200, map[string]interface{}{
            "status": "ok",
            "timestamp": time.Now().Format(time.RFC3339),
        })
    })

    return nil
})
```

### Monitoring Dashboard

Query fetch logs via API:

```bash
# Recent successful fetches
curl "http://127.0.0.1:8090/api/collections/fetch_logs/records?filter=success=true&sort=-created&perPage=10"

# Recent failures
curl "http://127.0.0.1:8090/api/collections/fetch_logs/records?filter=success=false&sort=-created&perPage=10"

# Statistics
curl "http://127.0.0.1:8090/api/collections/programs/records?page=1&perPage=1"
```

---

## Troubleshooting

### Job Not Running

1. Check logs for scheduler messages
2. Verify cron syntax
3. Check system timezone matches expectation
4. Test with shorter interval (e.g., every minute)

### API Fetch Failures

1. Check network connectivity
2. Verify API endpoint is accessible
3. Check rate limiting delays
4. Review fetch_logs for error messages

### Database Issues

1. Check disk space
2. Verify file permissions
3. Check PocketBase data directory
4. Review database backup strategy

---

## Backup Strategy

### Automated Backups

Add backup job:

```go
scheduler.MustAdd("backup_database", "0 4 * * *", func() {
    log.Println("💾 Starting database backup...")

    backupPath := fmt.Sprintf("backups/pb_data_%s.zip",
        time.Now().Format("20060102"))

    if err := app.CreateBackup(backupPath); err != nil {
        log.Printf("❌ Backup failed: %v", err)
    } else {
        log.Printf("✅ Backup saved: %s", backupPath)
    }
})
```

### Manual Backup

```bash
# Copy entire data directory
cp -r pb_data pb_data_backup_$(date +%Y%m%d)

# Or use built-in backup command
./tv-pocketbase backup
```

---

## Next Steps

1. ✅ Install PocketBase
2. ✅ Set up collections
3. ✅ Implement job scheduler
4. ⬜ Test data collection
5. ⬜ Deploy to production
6. ⬜ Set up monitoring
7. ⬜ Configure backups
8. ⬜ Build web interface

---

**Setup Version:** 1.0
**Last Updated:** 2025-12-16
**Status:** ✅ Ready for implementation
