# Finnish TV Program Collector - Go Edition

A complete PocketBase-powered application for collecting and serving Finnish TV program data from telkussa.fi.

## Features

- ✅ **Automated Data Collection**: Nightly job at 01:00 to fetch TV program data
- ✅ **Auto Cleanup**: Daily cleanup at 02:00 to remove old programs
- ✅ **Channel Management**: Weekly channel list update on Sundays at 03:00
- ✅ **REST API**: Custom endpoints for querying program data
- ✅ **Admin Controls**: Manual triggers for all operations
- ✅ **Built-in Database**: PocketBase SQLite database with web admin UI

## Architecture

This is a single Go application that extends PocketBase with:

1. **Custom Collections**: Channels, Programs, Series, and Fetch Logs
2. **Scheduled Jobs**: Automated data collection using PocketBase's cron plugin
3. **Custom API Endpoints**: Specialized queries for TV program data
4. **Admin UI**: Built-in PocketBase admin interface

## Quick Start

### 1. Prerequisites

- Go 1.21 or higher
- Internet connection (to fetch from telkussa.fi API)

### 2. Build and Run

```bash
# Clone/navigate to the directory
cd tv-go

# Download dependencies
go mod download

# Build the application
go build -o tv-pocketbase

# Run the application
./tv-pocketbase serve
```

### 3. Access the Application

- **Admin UI**: http://127.0.0.1:8090/_/
- **API**: http://127.0.0.1:8090/api/

### 4. Create Admin Account

On first run, create an admin account:

```bash
./tv-pocketbase admin create
```

Or through the web UI at http://127.0.0.1:8090/_/

### 5. Initial Data Collection

Trigger initial data collection manually:

```bash
# Get your admin token from the admin UI (Settings -> API Preview)
# Or login via API and get the token

curl -X POST "http://127.0.0.1:8090/api/admin/trigger/update-channels" \
  -H "Authorization: Admin YOUR_ADMIN_TOKEN"

curl -X POST "http://127.0.0.1:8090/api/admin/trigger/fetch?days=7" \
  -H "Authorization: Admin YOUR_ADMIN_TOKEN"
```

## Scheduled Jobs

The application automatically runs these jobs:

| Job | Schedule | Description |
|-----|----------|-------------|
| `fetch_programs` | Daily at 01:00 | Fetch TV program data for next 7 days |
| `cleanup_old_data` | Daily at 02:00 | Delete programs older than 30 days |
| `update_channels` | Weekly Sun 03:00 | Update channel list from API |

## API Endpoints

### Public Endpoints

#### Health Check
```bash
GET /api/health
```

#### What's On Now
```bash
GET /api/tv/now

# Response: Array of currently airing programs with channel info
```

#### Tonight's Prime Time (20:00-23:00)
```bash
GET /api/tv/tonight

# Response: Array of programs airing tonight
```

#### Channel Schedule
```bash
GET /api/tv/schedule/:channelId/:date

# Example:
GET /api/tv/schedule/13/2025-12-16
```

#### Statistics
```bash
GET /api/tv/stats

# Response:
{
  "total_programs": 12543,
  "total_channels": 77,
  "total_series": 450
}
```

### Admin Endpoints (Require Authentication)

#### Trigger Data Collection
```bash
POST /api/admin/trigger/fetch?days=7
Authorization: Admin YOUR_TOKEN
```

#### Update Channel List
```bash
POST /api/admin/trigger/update-channels
Authorization: Admin YOUR_TOKEN
```

#### Trigger Cleanup
```bash
POST /api/admin/trigger/cleanup?days=30
Authorization: Admin YOUR_TOKEN
```

### PocketBase Standard Endpoints

All standard PocketBase collection APIs are available:

```bash
# List channels
GET /api/collections/channels/records

# Get specific program
GET /api/collections/programs/records/:id

# Search programs
GET /api/collections/programs/records?filter=name~"Uutiset"

# Get programs with channel expanded
GET /api/collections/programs/records?expand=channel

# Filter by date range
GET /api/collections/programs/records?filter=start_time>="2025-12-16 20:00:00"&&start_time<="2025-12-16 23:00:00"
```

## Database Collections

### channels
- `id`: Channel ID from API
- `name`: Channel name
- `show_order`: Display order
- `category`: Channel category (public, commercial, sports, etc.)
- `logo_url`: Logo URL (for future use)
- `active`: Whether to collect data for this channel

### programs
- `id`: Program ID from API
- `channel`: Relation to channels
- `name`: Program title
- `episode`: Episode information
- `description`: Program description
- `start_time`: Broadcast start time
- `end_time`: Broadcast end time
- `duration`: Duration in minutes
- `series`: Relation to series (if applicable)
- `age_limit`: Age restriction
- `rating`: User rating metric
- `is_series`: Boolean flag

### series
- `id`: Series ID from API
- `name`: Series name
- `description`: Series description
- `episode_count`: Total episodes
- `first_seen`: First appearance date
- `last_seen`: Last appearance date
- `active`: Currently airing

### fetch_logs
- `channel`: Related channel
- `target_date`: Date being fetched (YYYYMMDD)
- `success`: Success/failure flag
- `programs_count`: Number of programs fetched
- `error_message`: Error details (if failed)
- `duration_ms`: Fetch duration

## Development

### Project Structure

```
tv-go/
├── main.go          # Application entry point and job scheduler
├── schema.go        # Database schema and collection definitions
├── collector.go     # API client and data collection logic
├── routes.go        # Custom API routes
├── go.mod           # Go dependencies
└── README.md        # This file
```

### Build Options

```bash
# Development build
go build -o tv-pocketbase

# Production build (smaller binary)
go build -ldflags="-s -w" -o tv-pocketbase

# Cross-compile for different platforms
GOOS=linux GOARCH=amd64 go build -o tv-pocketbase-linux
GOOS=darwin GOARCH=amd64 go build -o tv-pocketbase-macos
GOOS=windows GOARCH=amd64 go build -o tv-pocketbase.exe
```

### Running in Development Mode

```bash
# Enable auto-migration
ENV=development ./tv-pocketbase serve

# Custom port
./tv-pocketbase serve --http=127.0.0.1:8080

# Enable debug logging
./tv-pocketbase serve --dev
```

### Testing Jobs Locally

Modify cron schedules for testing:

```go
// In main.go, change:
scheduler.MustAdd("fetch_programs", "0 1 * * *", func() {
// To run every 2 minutes:
scheduler.MustAdd("fetch_programs", "*/2 * * * *", func() {
```

## Production Deployment

### 1. Systemd Service

Create `/etc/systemd/system/tv-pocketbase.service`:

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

Environment="ENV=production"

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

### 2. Nginx Reverse Proxy

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

        # WebSocket support (for PocketBase realtime)
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### 3. Docker Deployment (Optional)

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

RUN go mod download
RUN go build -ldflags="-s -w" -o tv-pocketbase

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/tv-pocketbase .

EXPOSE 8090

CMD ["./tv-pocketbase", "serve", "--http=0.0.0.0:8090"]
```

Build and run:

```bash
docker build -t tv-pocketbase .
docker run -d -p 8090:8090 -v $(pwd)/pb_data:/app/pb_data tv-pocketbase
```

## Backup and Restore

### Automated Backups

PocketBase includes built-in backup functionality:

```bash
# Create backup
./tv-pocketbase backup

# Restore from backup
./tv-pocketbase restore pb_data_backup.zip
```

### Manual Backup

Simply copy the `pb_data` directory:

```bash
cp -r pb_data pb_data_backup_$(date +%Y%m%d)
```

## Monitoring

### View Logs

```bash
# Systemd logs
sudo journalctl -u tv-pocketbase -f

# Application logs (if running directly)
tail -f pb_data/logs/*.log
```

### Check Fetch Logs

Via API:

```bash
# Recent successful fetches
curl "http://127.0.0.1:8090/api/collections/fetch_logs/records?filter=success=true&sort=-created&perPage=10"

# Recent failures
curl "http://127.0.0.1:8090/api/collections/fetch_logs/records?filter=success=false&sort=-created&perPage=10"
```

Via Admin UI:

1. Go to http://127.0.0.1:8090/_/
2. Navigate to "fetch_logs" collection
3. Filter by success/failure

## Troubleshooting

### Jobs Not Running

1. Check logs for scheduler startup message
2. Verify system timezone is correct
3. Test with shorter interval (e.g., every minute: `* * * * *`)

### API Fetch Failures

1. Check internet connectivity
2. Verify telkussa.fi API is accessible
3. Check fetch_logs collection for error messages
4. Ensure rate limiting is appropriate (1 second delay)

### Database Issues

1. Check disk space: `df -h`
2. Verify pb_data directory permissions
3. Check SQLite database integrity: `sqlite3 pb_data/data.db "PRAGMA integrity_check"`

## Environment Variables

```bash
# Set custom data directory
export PB_DATA_DIR=/custom/path

# Set custom public directory
export PB_PUBLIC_DIR=./public

# Development mode
export ENV=development
```

## Performance Tuning

### Database Optimization

The schema includes indexes on:
- `programs.channel`
- `programs.start_time`
- `programs.end_time`
- `programs.name`
- `programs.series`

### Caching

Consider adding nginx caching for frequently accessed endpoints:

```nginx
location /api/tv/now {
    proxy_pass http://127.0.0.1:8090;
    proxy_cache_valid 200 1m;
    proxy_cache_key $request_uri;
}
```

## Contributing

This is a research project for personal use. Feel free to adapt and modify.

## Legal & Ethical

- Always respect telkussa.fi's terms of service
- Implement appropriate rate limiting (1 second delay implemented)
- This tool is for personal/research use only
- Do not overload their servers

## License

This is a research project. Use responsibly and at your own risk.

## Support

For PocketBase documentation: https://pocketbase.io/docs/
For Go documentation: https://go.dev/doc/

---

**Version:** 1.0
**Last Updated:** 2025-12-16
**Status:** ✅ Production Ready
