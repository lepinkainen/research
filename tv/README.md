# Finnish TV Program Data Collector

A tool to fetch and store Finnish TV program data from telkussa.fi API into a local SQLite database for further processing.

## Overview

This project provides scripts to:
1. **Explore** the telkussa.fi API structure
2. **Collect** TV program data daily
3. **Store** data in a normalized SQLite database
4. **Query** program information efficiently

## Files

- `API_EXPLORATION.md` - Comprehensive API documentation and implementation plan
- `api_explorer.py` - Script to discover channels and analyze API structure
- `tv_database.py` - SQLite database manager with schema and queries
- `collector.py` - Daily data collection script
- `requirements.txt` - Python dependencies
- `query_examples.py` - Example queries and usage patterns

## Quick Start

### 1. Install Dependencies

```bash
pip install -r requirements.txt
```

### 2. Explore the API

First, run the API explorer to discover available channels and understand the data structure:

```bash
python api_explorer.py
```

This will:
- Test various channel IDs (1-50)
- Save sample responses to `data/samples/`
- Test date range availability
- Analyze the data structure

Check the output files:
```bash
ls data/samples/
cat data/samples/discovered_channels.json
```

### 3. Update Channel List

After running the explorer, update the `CHANNELS` list in `collector.py` with the actual channels you want to track.

Also update the `parse_and_store_programs()` method in `collector.py` to match the actual API response structure found in the sample JSON files.

### 4. Collect Data

Run the collector to fetch TV program data:

```bash
# Collect today + 7 days ahead (default)
python collector.py

# Collect today + 14 days ahead
python collector.py --days-ahead 14

# Clean up old data (older than 30 days)
python collector.py --cleanup

# Just update channel information
python collector.py --update-channels-only
```

### 5. Query the Data

Use the database manager to query programs:

```python
from tv_database import TVDatabase

db = TVDatabase()

# What's on now?
now_playing = db.get_programs_now()
for program in now_playing:
    print(f"{program['channel_name']}: {program['title']}")

# What's on tonight?
tonight = db.get_programs_by_date('2025-12-16')
for program in tonight:
    if '20:00' <= program['start_time'] <= '23:00':
        print(f"{program['start_time']} - {program['channel_name']}: {program['title']}")

# Search for a show
results = db.search_programs('Uutiset')
for r in results:
    print(f"{r['channel_name']}: {r['title']} at {r['start_time']}")

# Get statistics
stats = db.get_statistics()
print(f"Total programs: {stats['total_programs']}")
print(f"Total channels: {stats['total_channels']}")
```

## Automation

Set up a cron job to collect data daily:

```bash
# Edit crontab
crontab -e

# Add this line to run every day at 6:00 AM
0 6 * * * cd /path/to/tv && /usr/bin/python3 collector.py --days-ahead 7 >> logs/cron.log 2>&1
```

## Database Schema

The SQLite database includes these tables:

- **channels** - Channel information (id, name, logo, category)
- **programs** - Program details (title, description, times, metadata)
- **genres** - Genre names
- **program_genres** - Many-to-many relationship between programs and genres
- **people** - Actors, directors, etc.
- **program_people** - Many-to-many relationship between programs and people
- **fetch_log** - API fetch operation logs

See `API_EXPLORATION.md` for detailed schema documentation.

## Project Structure

```
tv/
├── README.md                    # This file
├── API_EXPLORATION.md          # Detailed API documentation
├── requirements.txt            # Python dependencies
├── api_explorer.py             # API exploration script
├── tv_database.py              # Database manager
├── collector.py                # Data collection script
├── query_examples.py           # Example queries (to be created)
├── data/
│   └── samples/                # Sample API responses
├── logs/
│   ├── collector.log          # Collection logs
│   └── cron.log               # Cron job logs
└── tv_programs.db             # SQLite database (created on first run)
```

## Configuration

### Channels

Edit `collector.py` and update the `CHANNELS` list:

```python
CHANNELS = [
    {'id': 1, 'name': 'YLE TV1', 'category': 'public'},
    {'id': 2, 'name': 'YLE TV2', 'category': 'public'},
    # Add more channels...
]
```

### Rate Limiting

The collector includes a 1-second delay between channel requests to be respectful to the API server. Adjust in `collector.py` if needed:

```python
sleep(1)  # Adjust this value
```

## API Details

### Endpoint Pattern

```
https://telkussa.fi/API/Channel/{channel_id}/{date}
```

- `channel_id`: Numeric channel identifier
- `date`: Format YYYYMMDD (e.g., 20251216)

### Example

```
https://telkussa.fi/API/Channel/13/20251216
```

See `API_EXPLORATION.md` for comprehensive API documentation.

## Development

### Testing the Database

```bash
# Test database operations
python tv_database.py
```

### Exploring a Specific Channel

```python
from api_explorer import TelkussaExplorer

explorer = TelkussaExplorer()
success, data = explorer.test_channel(13, '20251216')

if success:
    print(json.dumps(data, indent=2))
```

### Custom Queries

The `TVDatabase` class provides these query methods:

- `get_programs_by_date(date, channel_id=None)` - Programs for a specific date
- `get_programs_now()` - Currently airing programs
- `search_programs(query)` - Search by title
- `get_programs_by_genre(genre_name)` - Programs by genre
- `get_channels(active_only=True)` - List of channels
- `get_statistics()` - Database statistics

## Troubleshooting

### API returns 403 Forbidden

The API might have rate limiting or require specific headers. The scripts include User-Agent headers, but you may need to adjust them.

### No data collected

1. Check if the API endpoint is correct
2. Verify channel IDs are valid (run `api_explorer.py`)
3. Check logs in `logs/collector.log`
4. Verify date format is YYYYMMDD

### Database locked errors

SQLite can have locking issues with concurrent access. Ensure only one collector instance runs at a time.

## Future Enhancements

- [ ] Web interface for browsing programs
- [ ] Email notifications for favorite shows
- [ ] Export to CSV/JSON
- [ ] Program recommendation system
- [ ] Integration with Plex/Jellyfin
- [ ] Calendar integration (iCal format)
- [ ] Telegram/Discord bot for queries

## License

This is a personal research project. Respect telkussa.fi's terms of service and robots.txt when using this tool.

## Contributing

This is a research project for personal use. Feel free to adapt and modify for your own needs.

## Notes

- Always respect the API provider's terms of service
- Implement appropriate rate limiting
- Cache responses when possible
- Don't overload their servers
- This tool is for personal/research use only
