# Telkussa.fi TV Program API Exploration

## Overview

This document explores the telkussa.fi API for retrieving Finnish TV program data, with the goal of building a daily data collection tool that stores program information in a local SQLite database.

## API Endpoint Structure

### Base URL
```
https://telkussa.fi/API/Channel/{channel_id}/{date}
```

### Parameters
- **channel_id**: Numeric identifier for the TV channel (e.g., 13)
- **date**: Date in format YYYYMMDD (e.g., 20251216 for December 16, 2025)

### Known Endpoints
```
https://telkussa.fi/API/Channel/13/20251216
https://telkussa.fi/sivu/1/20251216 (main page with channel listings)
```

## Investigation Tasks

### 1. Channel Discovery

**Task**: Determine all available channel IDs

**Approach**:
1. Scrape the main page (https://telkussa.fi/sivu/1/YYYYMMDD) to extract channel IDs
2. Try sequential channel IDs (1-100) to find valid channels
3. Check for a channels list endpoint (e.g., `/API/Channels`)

**Expected Information**:
- Channel ID
- Channel name (e.g., "YLE TV1", "MTV3", "Nelonen")
- Channel logo URL (if available)
- Channel category/type (terrestrial, cable, streaming)

### 2. Program Data Structure

**Task**: Analyze the JSON response from channel endpoints

**Expected Fields** (based on typical TV guide APIs):
```json
{
  "channel": {
    "id": 13,
    "name": "Channel Name",
    "logo": "url_to_logo"
  },
  "date": "2025-12-16",
  "programs": [
    {
      "id": "unique_program_id",
      "title": "Program Title",
      "description": "Program description",
      "startTime": "2025-12-16T18:00:00",
      "endTime": "2025-12-16T19:30:00",
      "duration": 90,
      "category": "Drama/News/Sports/etc",
      "series": true/false,
      "season": 2,
      "episode": 5,
      "episodeTitle": "Episode specific title",
      "ageRating": "S",
      "image": "url_to_program_image",
      "genres": ["Drama", "Thriller"],
      "actors": ["Actor 1", "Actor 2"],
      "director": "Director Name",
      "year": 2024,
      "country": "Finland",
      "rerun": true/false
    }
  ]
}
```

### 3. API Behavior Testing

**Rate Limiting**:
- Test multiple requests to understand rate limits
- Check if authentication is required
- Verify if User-Agent matters

**Date Range**:
- Test past dates (historical data availability)
- Test future dates (how many days ahead are available)
- Test invalid dates (error handling)

**Error Responses**:
- Invalid channel ID
- Invalid date format
- Missing parameters
- Network errors

### 4. Additional Endpoints to Explore

Try these potential endpoints:
```
https://telkussa.fi/API/Channels
https://telkussa.fi/API/Programs
https://telkussa.fi/API/Program/{program_id}
https://telkussa.fi/API/Search?q={query}
https://telkussa.fi/API/Schedule/{date}
https://telkussa.fi/API/Now
```

## Database Schema

### Proposed SQLite Schema

```sql
-- Channels table
CREATE TABLE channels (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    logo_url TEXT,
    category TEXT,
    active BOOLEAN DEFAULT 1,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Programs table
CREATE TABLE programs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id TEXT UNIQUE,
    channel_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    start_time TEXT NOT NULL,
    end_time TEXT NOT NULL,
    duration INTEGER,
    category TEXT,
    is_series BOOLEAN DEFAULT 0,
    season INTEGER,
    episode INTEGER,
    episode_title TEXT,
    age_rating TEXT,
    image_url TEXT,
    year INTEGER,
    country TEXT,
    is_rerun BOOLEAN DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (channel_id) REFERENCES channels(id)
);

-- Genres table (many-to-many relationship)
CREATE TABLE genres (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL
);

CREATE TABLE program_genres (
    program_id INTEGER,
    genre_id INTEGER,
    PRIMARY KEY (program_id, genre_id),
    FOREIGN KEY (program_id) REFERENCES programs(id),
    FOREIGN KEY (genre_id) REFERENCES genres(id)
);

-- People table (actors, directors, etc.)
CREATE TABLE people (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL
);

CREATE TABLE program_people (
    program_id INTEGER,
    person_id INTEGER,
    role TEXT, -- 'actor', 'director', 'writer', etc.
    PRIMARY KEY (program_id, person_id, role),
    FOREIGN KEY (program_id) REFERENCES programs(id),
    FOREIGN KEY (person_id) REFERENCES people(id)
);

-- Fetch log table (track API calls)
CREATE TABLE fetch_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id INTEGER,
    date TEXT NOT NULL,
    success BOOLEAN,
    programs_count INTEGER,
    error_message TEXT,
    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_programs_channel_date ON programs(channel_id, start_time);
CREATE INDEX idx_programs_start_time ON programs(start_time);
CREATE INDEX idx_programs_title ON programs(title);
CREATE INDEX idx_fetch_log_date ON fetch_log(date, channel_id);
```

## Implementation Plan

### Phase 1: API Exploration Script

Create a Python script to:
1. Test various channel IDs
2. Fetch and save sample responses
3. Document the actual data structure
4. Identify all available fields
5. Test rate limits and error handling

### Phase 2: Data Collector

Build the daily data collection tool:
1. Read list of channels to monitor
2. Fetch program data for each channel
3. Parse and validate JSON responses
4. Store data in SQLite database
5. Handle duplicates (upsert logic)
6. Log all fetch operations
7. Error handling and retry logic

### Phase 3: Data Analysis & Queries

Create useful queries:
- What's on now?
- Tonight's prime time (20:00-23:00)
- Search by title/actor/genre
- Series tracking
- Statistics (most common genres, etc.)

## Sample Implementation Code

### 1. API Explorer Script

```python
#!/usr/bin/env python3
"""
Telkussa API Explorer
Explores the telkussa.fi API structure
"""

import requests
import json
from datetime import datetime, timedelta
from pathlib import Path
from time import sleep

class TelkussaExplorer:
    BASE_URL = "https://telkussa.fi/API"
    HEADERS = {
        'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36',
        'Accept': 'application/json',
        'Accept-Language': 'fi-FI,fi;q=0.9,en;q=0.8',
    }

    def __init__(self, output_dir="data/samples"):
        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(parents=True, exist_ok=True)
        self.session = requests.Session()
        self.session.headers.update(self.HEADERS)

    def test_channel(self, channel_id, date_str=None):
        """Test a specific channel endpoint"""
        if date_str is None:
            date_str = datetime.now().strftime("%Y%m%d")

        url = f"{self.BASE_URL}/Channel/{channel_id}/{date_str}"
        print(f"Testing: {url}")

        try:
            response = self.session.get(url, timeout=10)
            response.raise_for_status()

            data = response.json()

            # Save sample response
            filename = self.output_dir / f"channel_{channel_id}_{date_str}.json"
            with open(filename, 'w', encoding='utf-8') as f:
                json.dump(data, f, indent=2, ensure_ascii=False)

            print(f"✓ Channel {channel_id} - Success ({len(data.get('programs', []))} programs)")
            return True, data

        except requests.HTTPError as e:
            print(f"✗ Channel {channel_id} - HTTP {e.response.status_code}")
            return False, None
        except Exception as e:
            print(f"✗ Channel {channel_id} - Error: {e}")
            return False, None

    def discover_channels(self, start=1, end=50):
        """Discover valid channel IDs"""
        valid_channels = []
        date_str = datetime.now().strftime("%Y%m%d")

        for channel_id in range(start, end + 1):
            success, data = self.test_channel(channel_id, date_str)
            if success:
                channel_info = {
                    'id': channel_id,
                    'data_sample': data
                }
                valid_channels.append(channel_info)

            # Be nice to the server
            sleep(0.5)

        # Save discovered channels
        with open(self.output_dir / "discovered_channels.json", 'w', encoding='utf-8') as f:
            json.dump(valid_channels, f, indent=2, ensure_ascii=False)

        return valid_channels

    def test_date_range(self, channel_id=13):
        """Test how far back and forward we can fetch data"""
        results = {
            'past': [],
            'future': []
        }

        # Test past dates (30 days back)
        for days_ago in range(30):
            date = datetime.now() - timedelta(days=days_ago)
            date_str = date.strftime("%Y%m%d")
            success, _ = self.test_channel(channel_id, date_str)
            results['past'].append({
                'date': date_str,
                'available': success
            })
            sleep(0.3)

        # Test future dates (30 days ahead)
        for days_ahead in range(1, 31):
            date = datetime.now() + timedelta(days=days_ahead)
            date_str = date.strftime("%Y%m%d")
            success, _ = self.test_channel(channel_id, date_str)
            results['future'].append({
                'date': date_str,
                'available': success
            })
            sleep(0.3)

        with open(self.output_dir / "date_range_test.json", 'w', encoding='utf-8') as f:
            json.dump(results, f, indent=2)

        return results

    def analyze_structure(self, sample_file):
        """Analyze the structure of a sample response"""
        with open(sample_file, 'r', encoding='utf-8') as f:
            data = json.load(f)

        def get_structure(obj, prefix=""):
            """Recursively analyze object structure"""
            if isinstance(obj, dict):
                for key, value in obj.items():
                    full_key = f"{prefix}.{key}" if prefix else key
                    print(f"{full_key}: {type(value).__name__}")
                    if isinstance(value, (dict, list)) and value:
                        get_structure(value[0] if isinstance(value, list) else value, full_key)
            elif isinstance(obj, list) and obj:
                print(f"{prefix}[]: {type(obj[0]).__name__}")
                get_structure(obj[0], prefix)

        print("\n=== Data Structure Analysis ===")
        get_structure(data)

if __name__ == "__main__":
    explorer = TelkussaExplorer()

    print("=== Telkussa API Explorer ===\n")

    # Test a known channel
    print("1. Testing known channel (13)...")
    explorer.test_channel(13)

    # Discover channels
    print("\n2. Discovering channels (1-50)...")
    channels = explorer.discover_channels(1, 50)
    print(f"\nFound {len(channels)} valid channels")

    # Test date range
    print("\n3. Testing date range availability...")
    date_results = explorer.test_date_range(13)

    # Analyze structure
    print("\n4. Analyzing data structure...")
    sample_files = list(Path("data/samples").glob("channel_*.json"))
    if sample_files:
        explorer.analyze_structure(sample_files[0])

    print("\n✓ Exploration complete. Check 'data/samples' directory for results.")
```

### 2. Database Manager

```python
#!/usr/bin/env python3
"""
SQLite Database Manager for TV Program Data
"""

import sqlite3
from datetime import datetime
from pathlib import Path
from contextlib import contextmanager

class TVDatabase:
    def __init__(self, db_path="tv_programs.db"):
        self.db_path = Path(db_path)
        self.init_database()

    @contextmanager
    def get_connection(self):
        """Context manager for database connections"""
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        try:
            yield conn
            conn.commit()
        except Exception:
            conn.rollback()
            raise
        finally:
            conn.close()

    def init_database(self):
        """Initialize database with schema"""
        with self.get_connection() as conn:
            # Read schema from file or use inline SQL
            conn.executescript("""
                -- Channels table
                CREATE TABLE IF NOT EXISTS channels (
                    id INTEGER PRIMARY KEY,
                    name TEXT NOT NULL,
                    logo_url TEXT,
                    category TEXT,
                    active BOOLEAN DEFAULT 1,
                    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                );

                -- Programs table
                CREATE TABLE IF NOT EXISTS programs (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    external_id TEXT UNIQUE,
                    channel_id INTEGER NOT NULL,
                    title TEXT NOT NULL,
                    description TEXT,
                    start_time TEXT NOT NULL,
                    end_time TEXT NOT NULL,
                    duration INTEGER,
                    category TEXT,
                    is_series BOOLEAN DEFAULT 0,
                    season INTEGER,
                    episode INTEGER,
                    episode_title TEXT,
                    age_rating TEXT,
                    image_url TEXT,
                    year INTEGER,
                    country TEXT,
                    is_rerun BOOLEAN DEFAULT 0,
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    FOREIGN KEY (channel_id) REFERENCES channels(id)
                );

                -- Fetch log table
                CREATE TABLE IF NOT EXISTS fetch_log (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    channel_id INTEGER,
                    date TEXT NOT NULL,
                    success BOOLEAN,
                    programs_count INTEGER,
                    error_message TEXT,
                    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                );

                -- Indexes
                CREATE INDEX IF NOT EXISTS idx_programs_channel_date
                    ON programs(channel_id, start_time);
                CREATE INDEX IF NOT EXISTS idx_programs_start_time
                    ON programs(start_time);
                CREATE INDEX IF NOT EXISTS idx_programs_title
                    ON programs(title);
                CREATE INDEX IF NOT EXISTS idx_fetch_log_date
                    ON fetch_log(date, channel_id);
            """)

    def upsert_channel(self, channel_id, name, logo_url=None, category=None):
        """Insert or update channel information"""
        with self.get_connection() as conn:
            conn.execute("""
                INSERT INTO channels (id, name, logo_url, category, last_updated)
                VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
                ON CONFLICT(id) DO UPDATE SET
                    name = excluded.name,
                    logo_url = excluded.logo_url,
                    category = excluded.category,
                    last_updated = CURRENT_TIMESTAMP
            """, (channel_id, name, logo_url, category))

    def insert_program(self, program_data):
        """Insert program data (skip if already exists based on external_id)"""
        with self.get_connection() as conn:
            try:
                conn.execute("""
                    INSERT INTO programs (
                        external_id, channel_id, title, description,
                        start_time, end_time, duration, category,
                        is_series, season, episode, episode_title,
                        age_rating, image_url, year, country, is_rerun
                    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """, (
                    program_data.get('external_id'),
                    program_data['channel_id'],
                    program_data['title'],
                    program_data.get('description'),
                    program_data['start_time'],
                    program_data['end_time'],
                    program_data.get('duration'),
                    program_data.get('category'),
                    program_data.get('is_series', False),
                    program_data.get('season'),
                    program_data.get('episode'),
                    program_data.get('episode_title'),
                    program_data.get('age_rating'),
                    program_data.get('image_url'),
                    program_data.get('year'),
                    program_data.get('country'),
                    program_data.get('is_rerun', False)
                ))
                return True
            except sqlite3.IntegrityError:
                # Already exists
                return False

    def log_fetch(self, channel_id, date, success, programs_count=0, error_msg=None):
        """Log a fetch operation"""
        with self.get_connection() as conn:
            conn.execute("""
                INSERT INTO fetch_log (channel_id, date, success, programs_count, error_message)
                VALUES (?, ?, ?, ?, ?)
            """, (channel_id, date, success, programs_count, error_msg))

    def get_programs_by_date(self, date, channel_id=None):
        """Get all programs for a specific date"""
        with self.get_connection() as conn:
            if channel_id:
                query = """
                    SELECT p.*, c.name as channel_name
                    FROM programs p
                    JOIN channels c ON p.channel_id = c.id
                    WHERE date(p.start_time) = ? AND p.channel_id = ?
                    ORDER BY p.start_time
                """
                cursor = conn.execute(query, (date, channel_id))
            else:
                query = """
                    SELECT p.*, c.name as channel_name
                    FROM programs p
                    JOIN channels c ON p.channel_id = c.id
                    WHERE date(p.start_time) = ?
                    ORDER BY p.channel_id, p.start_time
                """
                cursor = conn.execute(query, (date,))

            return [dict(row) for row in cursor.fetchall()]

    def search_programs(self, query):
        """Search programs by title"""
        with self.get_connection() as conn:
            cursor = conn.execute("""
                SELECT p.*, c.name as channel_name
                FROM programs p
                JOIN channels c ON p.channel_id = c.id
                WHERE p.title LIKE ?
                ORDER BY p.start_time DESC
                LIMIT 50
            """, (f"%{query}%",))

            return [dict(row) for row in cursor.fetchall()]

    def get_statistics(self):
        """Get database statistics"""
        with self.get_connection() as conn:
            stats = {}

            # Total programs
            cursor = conn.execute("SELECT COUNT(*) as count FROM programs")
            stats['total_programs'] = cursor.fetchone()['count']

            # Total channels
            cursor = conn.execute("SELECT COUNT(*) as count FROM channels WHERE active = 1")
            stats['total_channels'] = cursor.fetchone()['count']

            # Date range
            cursor = conn.execute("""
                SELECT
                    MIN(date(start_time)) as earliest,
                    MAX(date(start_time)) as latest
                FROM programs
            """)
            row = cursor.fetchone()
            stats['date_range'] = {
                'earliest': row['earliest'],
                'latest': row['latest']
            }

            # Last fetch
            cursor = conn.execute("""
                SELECT date, fetched_at
                FROM fetch_log
                ORDER BY fetched_at DESC
                LIMIT 1
            """)
            row = cursor.fetchone()
            if row:
                stats['last_fetch'] = {
                    'date': row['date'],
                    'time': row['fetched_at']
                }

            return stats
```

### 3. Daily Data Collector

```python
#!/usr/bin/env python3
"""
Daily TV Program Data Collector
Fetches TV program data from telkussa.fi and stores in SQLite
"""

import requests
import json
from datetime import datetime, timedelta
from time import sleep
import logging
from pathlib import Path

# Import the database manager (from above)
# from tv_database import TVDatabase

class TelkussaCollector:
    BASE_URL = "https://telkussa.fi/API"
    HEADERS = {
        'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36',
        'Accept': 'application/json',
        'Accept-Language': 'fi-FI,fi;q=0.9,en;q=0.8',
    }

    # List of channels to fetch (update after discovery)
    CHANNELS = [
        {'id': 13, 'name': 'Channel 13'},
        # Add more channels here
    ]

    def __init__(self, db_path="tv_programs.db"):
        self.db = TVDatabase(db_path)
        self.session = requests.Session()
        self.session.headers.update(self.HEADERS)

        # Setup logging
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(levelname)s - %(message)s',
            handlers=[
                logging.FileHandler('collector.log'),
                logging.StreamHandler()
            ]
        )
        self.logger = logging.getLogger(__name__)

    def fetch_channel_data(self, channel_id, date_str):
        """Fetch program data for a specific channel and date"""
        url = f"{self.BASE_URL}/Channel/{channel_id}/{date_str}"

        try:
            response = self.session.get(url, timeout=15)
            response.raise_for_status()
            return response.json()
        except requests.RequestException as e:
            self.logger.error(f"Failed to fetch channel {channel_id} for {date_str}: {e}")
            return None

    def parse_and_store_programs(self, channel_id, data, date_str):
        """Parse API response and store programs in database"""
        if not data:
            return 0

        # This is where you'll need to adapt to the actual API structure
        # The following is a template based on expected structure

        programs = data.get('programs', [])
        stored_count = 0

        for program in programs:
            program_data = {
                'external_id': program.get('id'),
                'channel_id': channel_id,
                'title': program.get('title'),
                'description': program.get('description'),
                'start_time': program.get('startTime'),
                'end_time': program.get('endTime'),
                'duration': program.get('duration'),
                'category': program.get('category'),
                'is_series': program.get('series', False),
                'season': program.get('season'),
                'episode': program.get('episode'),
                'episode_title': program.get('episodeTitle'),
                'age_rating': program.get('ageRating'),
                'image_url': program.get('image'),
                'year': program.get('year'),
                'country': program.get('country'),
                'is_rerun': program.get('rerun', False)
            }

            if self.db.insert_program(program_data):
                stored_count += 1

        return stored_count

    def collect_daily_data(self, date=None, days_ahead=0):
        """Collect data for a specific date (default: today)"""
        if date is None:
            date = datetime.now()

        # Collect for today and N days ahead
        for day_offset in range(days_ahead + 1):
            target_date = date + timedelta(days=day_offset)
            date_str = target_date.strftime("%Y%m%d")

            self.logger.info(f"Collecting data for {date_str}")

            for channel in self.CHANNELS:
                channel_id = channel['id']
                channel_name = channel['name']

                self.logger.info(f"  Fetching {channel_name} (ID: {channel_id})")

                data = self.fetch_channel_data(channel_id, date_str)

                if data:
                    count = self.parse_and_store_programs(channel_id, data, date_str)
                    self.db.log_fetch(channel_id, date_str, True, count)
                    self.logger.info(f"    Stored {count} programs")
                else:
                    self.db.log_fetch(channel_id, date_str, False, 0, "Fetch failed")
                    self.logger.warning(f"    Failed to fetch data")

                # Rate limiting
                sleep(1)

            self.logger.info(f"Completed {date_str}\n")

    def update_channels(self):
        """Update channel information in database"""
        for channel in self.CHANNELS:
            self.db.upsert_channel(
                channel['id'],
                channel['name'],
                channel.get('logo_url'),
                channel.get('category')
            )

if __name__ == "__main__":
    collector = TelkussaCollector()

    # Update channels first
    collector.update_channels()

    # Collect today's data + 7 days ahead
    collector.collect_daily_data(days_ahead=7)

    # Print statistics
    stats = collector.db.get_statistics()
    print("\n=== Database Statistics ===")
    print(f"Total programs: {stats['total_programs']}")
    print(f"Total channels: {stats['total_channels']}")
    print(f"Date range: {stats['date_range']['earliest']} to {stats['date_range']['latest']}")
    if 'last_fetch' in stats:
        print(f"Last fetch: {stats['last_fetch']['date']} at {stats['last_fetch']['time']}")
```

## Usage Workflow

### Step 1: Explore the API
```bash
python api_explorer.py
```

This will:
- Test various channel IDs
- Save sample responses to `data/samples/`
- Analyze data structure
- Test date range availability

### Step 2: Update Channel List

After exploration, update the `CHANNELS` list in `collector.py` with discovered channels.

### Step 3: Run Daily Collection

```bash
# Collect today + 7 days ahead
python collector.py

# Or use cron for daily execution
0 6 * * * cd /path/to/tv && python collector.py
```

### Step 4: Query the Data

```python
from tv_database import TVDatabase

db = TVDatabase()

# What's on today?
programs = db.get_programs_by_date('2025-12-16')
for p in programs:
    print(f"{p['start_time']} - {p['channel_name']}: {p['title']}")

# Search for programs
results = db.search_programs('Uutiset')
for r in results:
    print(f"{r['channel_name']}: {r['title']} at {r['start_time']}")

# Get statistics
stats = db.get_statistics()
print(stats)
```

## Considerations

### 1. Legal and Ethical
- Respect robots.txt
- Implement rate limiting (1-2 seconds between requests)
- Cache responses appropriately
- Don't overload their servers
- Check their Terms of Service

### 2. Error Handling
- Network failures
- Invalid JSON responses
- Missing fields
- Duplicate prevention
- Retry logic with exponential backoff

### 3. Data Quality
- Handle missing program descriptions
- Deal with timezone issues
- Validate date/time formats
- Handle special characters in titles
- Deal with incomplete data

### 4. Performance
- Batch inserts for better performance
- Use transactions
- Index frequently queried fields
- Implement incremental updates (only fetch new data)

### 5. Monitoring
- Log all operations
- Track success/failure rates
- Monitor database size
- Alert on errors

## Next Steps

1. **Run API Explorer**: Execute the explorer script to understand the actual API structure
2. **Document Findings**: Update this document with real data structures
3. **Implement Collector**: Build the daily collector based on actual API
4. **Setup Automation**: Configure cron job for daily execution
5. **Build Queries**: Create useful queries for your use case
6. **Add Features**:
   - Web interface for browsing programs
   - Search functionality
   - Notifications for favorite shows
   - Export to other formats

## Resources

- API Base: https://telkussa.fi/API/
- Main Site: https://telkussa.fi/
- Documentation: (To be discovered)

## Status

- [x] Document created
- [ ] API structure documented (blocked by network restrictions)
- [ ] Sample responses collected
- [ ] Database schema finalized
- [ ] Collector implemented and tested
- [ ] Automation configured
- [ ] Query examples created

---

**Note**: This document is based on the URL patterns provided. The actual API structure needs to be verified by running the explorer script with actual access to the telkussa.fi API. Update this document with real findings.
