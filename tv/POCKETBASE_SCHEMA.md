# PocketBase Schema Design for TV Program Data

**Project:** Finnish TV Program Collector
**Backend:** PocketBase
**Version:** 1.0
**Date:** 2025-12-16

---

## Overview

This document defines the PocketBase schema for storing Finnish TV program data from the telkussa.fi API.

### Collections Summary
1. **channels** - TV channel information (77 channels)
2. **programs** - Individual program broadcasts
3. **series** - Series metadata and information
4. **fetch_logs** - API fetch operation tracking

---

## Collection: `channels`

Stores information about TV channels.

### Fields

| Field | Type | Required | Unique | Default | Description |
|-------|------|----------|--------|---------|-------------|
| `id` | Text | ✅ | ✅ | - | Channel ID from API (e.g., "1", "13") |
| `name` | Text | ✅ | ✅ | - | Channel name (e.g., "Yle TV1") |
| `show_order` | Number | ✅ | ❌ | 0 | Display order from API |
| `category` | Select | ❌ | ❌ | null | Channel category |
| `logo_url` | URL | ❌ | ❌ | null | Channel logo (future use) |
| `active` | Bool | ✅ | ❌ | true | Whether channel is actively tracked |
| `last_updated` | Date | ✅ | ❌ | now | Last time channel info was updated |

### Field Details

**id** (Text)
- Primary identifier
- Use API's channel ID directly (1-133+)
- Format: String representation of integer
- Example: "1", "13", "22"

**name** (Text)
- Display name of channel
- Max length: 100 characters
- Example: "Yle TV1", "Yle Teema & Fem"

**show_order** (Number)
- Sort order for displaying channels
- Comes from API's `showOrder` field
- Lower numbers appear first
- Range: 1-9999

**category** (Select)
Options:
- `public` - Public broadcasters (Yle)
- `commercial` - Commercial channels (MTV3, Nelonen)
- `sports` - Sports channels
- `movies` - Movie channels
- `kids` - Children's channels
- `music` - Music channels
- `international` - International channels
- `documentary` - Documentary channels
- `other` - Other categories

**active** (Bool)
- true = Collect data for this channel
- false = Skip this channel in collection
- Allows disabling channels without deletion

### Indexes
- `name` (for search)
- `show_order` (for sorting)
- `active` (for filtering)

### API Rules

```javascript
// List: Any user can list active channels
listRule: "active = true"

// View: Any user can view channel details
viewRule: ""

// Create/Update/Delete: Admin only
createRule: "@request.auth.id != ''"
updateRule: "@request.auth.id != ''"
deleteRule: "@request.auth.id != ''"
```

---

## Collection: `programs`

Stores individual program broadcasts.

### Fields

| Field | Type | Required | Unique | Default | Description |
|-------|------|----------|--------|---------|-------------|
| `id` | Text | ✅ | ✅ | - | Program ID from API |
| `channel` | Relation | ✅ | ❌ | - | Related channel |
| `name` | Text | ✅ | ❌ | - | Program title |
| `episode` | Text | ❌ | ❌ | "" | Episode identifier |
| `description` | Editor | ❌ | ❌ | "" | Program description |
| `start_time` | Date | ✅ | ❌ | - | Broadcast start time |
| `end_time` | Date | ✅ | ❌ | - | Broadcast end time |
| `duration` | Number | ✅ | ❌ | 0 | Duration in minutes (calculated) |
| `series` | Relation | ❌ | ❌ | null | Related series (if applicable) |
| `age_limit` | Number | ✅ | ❌ | 0 | Age restriction |
| `rating` | Number | ❌ | ❌ | 0 | User rating metric |
| `is_series` | Bool | ✅ | ❌ | false | Whether program is part of a series |
| `created` | Date | ✅ | ❌ | now | Record creation time |
| `updated` | Date | ✅ | ❌ | now | Record update time |

### Field Details

**id** (Text)
- Primary identifier
- Use API's program `id` directly
- Format: String representation of integer
- Example: "77723755"
- Ensures no duplicate program entries

**channel** (Relation)
- Relation to `channels` collection
- Many-to-one relationship
- Required field
- Cascade delete: Delete programs when channel is deleted
- Display fields: `name`

**name** (Text)
- Program title
- Max length: 200 characters
- Required field
- Example: "BUU-klubben", "Uutiset"

**episode** (Text)
- Episode identifier or number
- Often empty in API response
- Max length: 100 characters
- Example: "S01E05", "3/6"

**description** (Editor)
- Program synopsis/description
- Can be empty
- Supports rich text (for future use)
- Max length: 5000 characters

**start_time** (Date)
- Broadcast start time
- Converted from API's Unix timestamp
- ISO 8601 format: "2025-12-16T18:00:00Z"
- Required field

**end_time** (Date)
- Broadcast end time
- Converted from API's Unix timestamp
- ISO 8601 format: "2025-12-16T19:30:00Z"
- Required field

**duration** (Number)
- Duration in minutes
- Calculated: (end_time - start_time) / 60
- Min: 0, Max: 1440 (24 hours)

**series** (Relation)
- Relation to `series` collection
- Optional (null for non-series content)
- Many-to-one relationship
- Display fields: `name`

**age_limit** (Number)
- Minimum age requirement
- Values: 0, 7, 12, 16, 18
- 0 = No restriction
- Finnish content rating system

**rating** (Number)
- User rating or engagement metric from API
- Values observed: 0, 10, 20
- Exact meaning unclear
- Min: 0, Max: 100

**is_series** (Bool)
- true = Part of a series
- false = Standalone program
- Calculated: (series_id from API > 0)

### Indexes
- `channel` (for filtering by channel)
- `start_time` (for time-based queries)
- `end_time` (for "what's on now" queries)
- `name` (for text search)
- `series` (for grouping episodes)
- Composite: `channel, start_time` (for common query pattern)

### API Rules

```javascript
// List: Any user can list programs
listRule: ""

// View: Any user can view program details
viewRule: ""

// Create/Update/Delete: Admin only
createRule: "@request.auth.id != ''"
updateRule: "@request.auth.id != ''"
deleteRule: "@request.auth.id != ''"
```

---

## Collection: `series`

Stores metadata about TV series.

### Fields

| Field | Type | Required | Unique | Default | Description |
|-------|------|----------|--------|---------|-------------|
| `id` | Text | ✅ | ✅ | - | Series ID from API |
| `name` | Text | ✅ | ❌ | - | Series name |
| `description` | Editor | ❌ | ❌ | "" | Series description |
| `episode_count` | Number | ❌ | ❌ | 0 | Total episodes (calculated) |
| `first_seen` | Date | ✅ | ❌ | now | First time series was seen |
| `last_seen` | Date | ✅ | ❌ | now | Last time series was seen |
| `active` | Bool | ✅ | ❌ | true | Currently airing |

### Field Details

**id** (Text)
- Primary identifier
- Use API's `series_id` directly
- Format: String representation of integer
- Example: "1109", "15376"

**name** (Text)
- Series name
- Extracted from first program with this series_id
- Max length: 200 characters
- Example: "BUU-klubben", "Studio 65"

**description** (Editor)
- Series description
- Aggregated/curated from episode descriptions
- Optional field
- Max length: 5000 characters

**episode_count** (Number)
- Total number of episodes in database
- Calculated field (count of related programs)
- Updated periodically

**first_seen / last_seen** (Date)
- Tracking when series appears in data
- Helps identify active vs. ended series
- ISO 8601 format

**active** (Bool)
- true = Currently airing new episodes
- false = No recent episodes
- Can be auto-calculated (last_seen within 30 days)

### Indexes
- `name` (for search)
- `active` (for filtering)
- `last_seen` (for sorting)

### API Rules

```javascript
// List/View: Any user
listRule: ""
viewRule: ""

// Create/Update/Delete: Admin only
createRule: "@request.auth.id != ''"
updateRule: "@request.auth.id != ''"
deleteRule: "@request.auth.id != ''"
```

---

## Collection: `fetch_logs`

Tracks API fetch operations for monitoring and debugging.

### Fields

| Field | Type | Required | Unique | Default | Description |
|-------|------|----------|--------|---------|-------------|
| `channel` | Relation | ❌ | ❌ | null | Related channel (if channel-specific) |
| `fetch_date` | Date | ✅ | ❌ | now | Date/time of fetch operation |
| `target_date` | Text | ✅ | ❌ | - | Target date being fetched (YYYYMMDD) |
| `success` | Bool | ✅ | ❌ | false | Whether fetch succeeded |
| `programs_count` | Number | ❌ | ❌ | 0 | Number of programs fetched |
| `error_message` | Text | ❌ | ❌ | "" | Error message (if failed) |
| `duration_ms` | Number | ❌ | ❌ | 0 | Fetch duration in milliseconds |

### Field Details

**channel** (Relation)
- Optional relation to `channels` collection
- null = system-wide operation (e.g., channel list fetch)
- Display fields: `name`

**fetch_date** (Date)
- When the fetch operation occurred
- Auto-populated
- ISO 8601 format

**target_date** (Text)
- The date being fetched (program schedule date)
- Format: YYYYMMDD (e.g., "20251216")
- Allows tracking which dates have been collected

**success** (Bool)
- true = Fetch completed successfully
- false = Fetch failed

**programs_count** (Number)
- Number of programs retrieved
- 0 if fetch failed or no programs available
- Useful for detecting anomalies

**error_message** (Text)
- Error details if fetch failed
- Empty string if successful
- Max length: 500 characters

**duration_ms** (Number)
- How long the fetch took
- In milliseconds
- Useful for performance monitoring

### Indexes
- `fetch_date` (for time-based queries)
- `success` (for error tracking)
- Composite: `channel, target_date` (for checking fetch status)

### API Rules

```javascript
// List/View: Admin only
listRule: "@request.auth.id != ''"
viewRule: "@request.auth.id != ''"

// Create: Admin or system
createRule: "@request.auth.id != ''"

// Update/Delete: Admin only
updateRule: "@request.auth.id != ''"
deleteRule: "@request.auth.id != ''"
```

---

## Relationships Diagram

```
channels (1) ─────< programs (many)
                      │
                      │ (many)
                      │
                      ▼
                   series (1)

fetch_logs >───── channels (optional)
```

---

## Indexes Strategy

### Performance-Critical Indexes

1. **programs.start_time** (Asc)
   - For "what's on now" queries
   - For date range queries

2. **programs.channel + start_time** (Composite)
   - For channel schedule views
   - Most common query pattern

3. **programs.name** (Text search)
   - For program search functionality
   - Consider full-text search index

4. **channels.show_order** (Asc)
   - For sorted channel listing

5. **series.last_seen** (Desc)
   - For finding active series

### Monitoring Indexes

6. **fetch_logs.fetch_date** (Desc)
   - For recent logs
   - For debugging

---

## Data Retention Policy

### Programs
- **Keep:** Programs from today forward (future programs)
- **Archive:** Programs older than 7 days
- **Delete:** Programs older than 30 days (optional)

### Fetch Logs
- **Keep:** Last 7 days of logs
- **Delete:** Logs older than 30 days

### Channels & Series
- **Keep:** Indefinitely (reference data)
- **Mark inactive:** Channels with no programs for 90 days

---

## Migration from SQLite

If migrating from the existing SQLite schema:

### Mapping

| SQLite Table | PocketBase Collection | Notes |
|--------------|----------------------|-------|
| `channels` | `channels` | Direct mapping |
| `programs` | `programs` | Add relation to channel |
| `genres` | - | Remove (not in API) |
| `program_genres` | - | Remove (not in API) |
| `people` | - | Remove (not in API) |
| `program_people` | - | Remove (not in API) |
| `fetch_log` | `fetch_logs` | Rename and add fields |

### Data Conversion

1. Convert SQLite integer IDs to text strings
2. Convert Unix timestamps to ISO 8601 dates
3. Create series records from unique series_id values
4. Establish channel → program relations
5. Establish series → program relations

---

## API Client Code Structure

### Creating Records

```python
from pocketbase import PocketBase

pb = PocketBase('http://127.0.0.1:8090')

# Authenticate as admin (for data collection script)
pb.admins.auth_with_password('admin@email.com', 'password')

# Create/update channel
channel_data = {
    'id': '13',
    'name': 'Yle Teema & Fem',
    'show_order': 6,
    'category': 'public',
    'active': True
}
pb.collection('channels').create(channel_data)

# Create program
program_data = {
    'id': '77723755',
    'channel': '13',  # Relation by ID
    'name': 'BUU-klubben',
    'episode': '',
    'description': 'Lumimaa jatkuu...',
    'start_time': '2025-12-16T08:00:00Z',
    'end_time': '2025-12-16T08:26:00Z',
    'duration': 26,
    'series': '1109',  # Relation by ID
    'age_limit': 0,
    'rating': 20,
    'is_series': True
}
pb.collection('programs').create(program_data)
```

### Querying Records

```python
# What's on now?
from datetime import datetime

now = datetime.now().isoformat()
programs = pb.collection('programs').get_list(
    1, 50,
    {
        'filter': f'start_time <= "{now}" && end_time >= "{now}"',
        'expand': 'channel',
        'sort': 'channel.show_order'
    }
)

# Tonight's prime time (20:00-23:00)
programs = pb.collection('programs').get_list(
    1, 100,
    {
        'filter': 'start_time >= "2025-12-16T20:00:00Z" && start_time <= "2025-12-16T23:00:00Z"',
        'expand': 'channel',
        'sort': 'start_time'
    }
)

# Search programs
programs = pb.collection('programs').get_list(
    1, 50,
    {
        'filter': 'name ~ "Uutiset"',
        'expand': 'channel',
        'sort': '-start_time'
    }
)

# Get channel schedule
programs = pb.collection('programs').get_list(
    1, 100,
    {
        'filter': 'channel = "13" && start_time >= "2025-12-16T00:00:00Z" && start_time < "2025-12-17T00:00:00Z"',
        'sort': 'start_time'
    }
)
```

---

## Setup Instructions

### 1. Install PocketBase

```bash
# Download PocketBase
wget https://github.com/pocketbase/pocketbase/releases/download/v0.22.0/pocketbase_0.22.0_linux_amd64.zip

# Extract
unzip pocketbase_0.22.0_linux_amd64.zip

# Make executable
chmod +x pocketbase

# Run
./pocketbase serve
```

Access admin UI: http://127.0.0.1:8090/_/

### 2. Create Collections

In the PocketBase admin UI:
1. Go to "Collections"
2. Click "New collection"
3. Create each collection as defined above
4. Set up fields, relations, and indexes
5. Configure API rules

### 3. Create Initial Admin User

```bash
./pocketbase admin create
```

### 4. Install Python SDK

```bash
pip install pocketbase
```

---

## Environment Configuration

### .env File

```bash
POCKETBASE_URL=http://127.0.0.1:8090
POCKETBASE_ADMIN_EMAIL=admin@example.com
POCKETBASE_ADMIN_PASSWORD=your_secure_password

# API Settings
TELKUSSA_API_URL=https://telkussa.fi/API
FETCH_DAYS_AHEAD=7
RATE_LIMIT_DELAY=1.0

# Data Retention
KEEP_PROGRAMS_DAYS=30
KEEP_LOGS_DAYS=30
```

---

## Testing Checklist

- [ ] Create all collections
- [ ] Set up all relationships
- [ ] Create indexes
- [ ] Test channel creation
- [ ] Test program creation with channel relation
- [ ] Test series creation and linking
- [ ] Test "what's on now" query
- [ ] Test program search
- [ ] Test fetch log creation
- [ ] Verify API rules work correctly
- [ ] Test data import from API
- [ ] Test duplicate prevention (unique IDs)

---

**Schema Version:** 1.0
**Last Updated:** 2025-12-16
**Status:** ✅ Complete and ready for implementation
