# Telkussa.fi API Investigation Findings

**Investigation Date:** 2025-12-16
**Status:** ✅ Complete

## Executive Summary

Successfully investigated the telkussa.fi API structure and discovered:
- **77 total channels** available in the system
- **16+ active channels** with current program data
- **REST API** with simple, predictable endpoints
- **JSON responses** with consistent structure
- **No authentication required** for public program data

---

## API Endpoints Discovered

### 1. Channels List Endpoint
```
GET https://telkussa.fi/API/Channels
```

**Response:** Array of channel objects

**Structure:**
```json
[
  {
    "id": 1,
    "name": "Yle TV1",
    "showOrder": 1
  },
  {
    "id": 2,
    "name": "Yle TV2",
    "showOrder": 2
  }
  // ... 77 total channels
]
```

**Fields:**
- `id` (integer): Unique channel identifier
- `name` (string): Channel display name
- `showOrder` (integer): Display order/sorting index

### 2. Channel Programs Endpoint
```
GET https://telkussa.fi/API/Channel/{channel_id}/{date}
```

**Parameters:**
- `channel_id`: Integer (1-133+)
- `date`: YYYYMMDD format (e.g., 20251216)

**Response:** Array of program objects

**Structure:**
```json
[
  {
    "name": "BUU-klubben",
    "episode": "",
    "description": "Lumimaa jatkuu. Sitä ennen...",
    "start": 29431076,
    "stop": 29431102,
    "series_id": 1109,
    "id": 77723755,
    "agelimit": 0,
    "channel": 13,
    "rating": 20
  }
]
```

**Fields:**
- `name` (string): Program title
- `episode` (string): Episode identifier (often empty)
- `description` (string): Program synopsis (can be empty)
- `start` (integer): Unix timestamp for start time
- `stop` (integer): Unix timestamp for end time
- `series_id` (integer): Series identifier (0 = standalone program)
- `id` (integer): Unique program instance ID
- `agelimit` (integer): Age restriction (0, 7, 12, etc.)
- `channel` (integer): Channel ID
- `rating` (integer): User rating metric (0, 10, 20)

### 3. Current Programs Endpoint
```
GET https://telkussa.fi/API/Now
```

Returns currently airing programs across all channels.

### 4. Schedule Endpoint
```
GET https://telkussa.fi/API/Schedule/{date}
```

Returns schedule for all channels on a given date.

---

## Active Channels Discovered

From testing channels 1-50, the following have current program data:

| Channel ID | Channel Name | Programs | Notes |
|-----------|--------------|----------|-------|
| 1 | Yle TV1 | ~48 | Public broadcaster |
| 2 | Yle TV2 | ~48 | Public broadcaster |
| 3 | MTV3 | ~39 | Commercial channel |
| 4 | Nelonen | ~34 | Commercial channel |
| 5 | SubTV | ~30 | Youth-oriented |
| 6 | Discovery Channel | ~30 | Documentary |
| 9 | Animal Planet | ~39 | Nature/Animals |
| 12 | TLC | ~31 | Lifestyle |
| 13 | Yle Teema & Fem | ~31 | Culture/Swedish |
| 17 | MTV Aitio | ~45 | Premium content |
| 22 | TV Finland | ~33 | International |
| 29 | Deutsche Welle | ~81 | News channel |
| 30 | MTV Max | ~3 | Movie channel |
| 32 | MTV Finland | ~41 | Music/Culture |
| 33 | V Film Premiere | ~15 | Premium movies |
| 35 | V Film Action | ~15 | Action movies |

**Total Channels in System:** 77 channels (from /API/Channels endpoint)

**Major Finnish Channels:**
- **Yle:** TV1, TV2, Teema & Fem
- **MTV:** MTV3, MTV Max, MTV Aitio, MTV Finland
- **Sanoma Media:** Nelonen, SubTV, Jim, Liv, Hero
- **International:** Discovery, Animal Planet, TLC, BBC Earth, National Geographic
- **Premium:** V Film series, C More series

---

## Data Characteristics

### Timestamps
- Unix timestamps are used (seconds since epoch)
- Example: `29431076` = approximately year 2900 BCE
- **NOTE:** These appear to be custom timestamps, not standard Unix time
- They likely represent minutes or another time unit relative to a base date

### Age Ratings
- `0`: No age restriction
- `7`: Ages 7+
- `12`: Ages 12+
- Higher values possible (Finnish content rating system)

### Series Identification
- `series_id: 0` = Standalone program (movie, special, etc.)
- `series_id: > 0` = Part of a series
- `episode` field often empty even for series content

### Program Ratings
- Values observed: `0`, `10`, `20`
- Likely represents engagement or quality metrics
- Scale and meaning unclear (may be internal)

---

## API Behavior Notes

### Rate Limiting
- No strict rate limiting observed during testing
- Recommended: 1-2 second delay between requests (good practice)
- Tested ~50 channels with 0.5s delay without issues

### Authentication
- **No authentication required** for public endpoints
- All tested endpoints work without API keys or tokens

### Headers Required
- `User-Agent`: Standard browser user agent recommended
- `Accept`: application/json
- `Accept-Language`: fi-FI,fi;q=0.9,en;q=0.8 (optional, but polite)

### Error Handling
- Empty channels return: `[]` (empty array)
- Invalid channel IDs return: `[]` (empty array, not 404)
- Invalid date formats may return errors or empty arrays

### Date Range
- Future dates: Available up to ~14 days ahead (needs confirmation)
- Past dates: Historical data availability unknown (needs testing)

---

## Data Quality Observations

### Complete Data
- ✅ Channel names and IDs
- ✅ Program titles
- ✅ Start/stop times
- ✅ Series identification
- ✅ Age ratings

### Often Present
- ✅ Descriptions (but can be empty)
- ✅ Episode information (for series)

### Missing/Unavailable
- ❌ Channel logos (not in API response)
- ❌ Program images/thumbnails
- ❌ Genre categories (not explicitly provided)
- ❌ Cast/crew information
- ❌ Country of origin
- ❌ Year of production
- ❌ Rerun indication

---

## Recommended Implementation Strategy

### 1. Data Collection
- Fetch channel list once daily (changes infrequently)
- Fetch program data for each channel daily
- Collect today + 7 days ahead
- Schedule: Run at 6:00 AM daily (off-peak)

### 2. Storage Strategy (PocketBase)
Three main collections:
1. **channels** - Channel information (77 records)
2. **programs** - Program instances (thousands of records)
3. **series** - Series metadata (for series_id > 0)

### 3. Update Strategy
- **Incremental updates:** Check program ID before inserting
- **Duplicate prevention:** Use program `id` as unique identifier
- **Historical data:** Keep programs for 30 days, then archive/delete

### 4. Query Patterns
Common queries to optimize for:
- "What's on now?" (current time lookup)
- "What's on tonight?" (date + time range)
- "Find program by title" (text search)
- "All programs for a channel" (channel_id filter)
- "Series episodes" (series_id grouping)

---

## Technical Recommendations

### PocketBase Schema
- Use `id` from API as unique identifier (not auto-increment)
- Store timestamps as integers (or convert to ISO datetime)
- Index: channel, start_time, series_id, program title
- Relationship: program → channel (many-to-one)

### Data Processing
- Convert timestamps to human-readable format for display
- Calculate duration from start/stop times
- Extract genre from description using keywords (if needed)
- Detect reruns by checking duplicate title+channel combinations

### Error Handling
- Log failed channel fetches
- Retry logic for network errors (3 attempts with backoff)
- Alert if channel returns zero programs unexpectedly
- Handle missing descriptions gracefully

### Performance
- Batch inserts where possible
- Use transactions for multiple programs
- Implement connection pooling
- Cache channel list (updates infrequently)

---

## Sample API Responses

### Channel List
See: `data/samples/endpoint__Channels.json`

### Program Data
- Yle TV1: `data/samples/channel_1_20251216.json` (48 programs)
- Yle Teema & Fem: `data/samples/channel_13_20251216.json` (31 programs)
- Deutsche Welle: `data/samples/channel_29_20251216.json` (81 programs)

---

## Next Steps

1. ✅ API structure documented
2. ✅ Sample responses collected
3. ⬜ Design PocketBase schema
4. ⬜ Implement PocketBase data layer
5. ⬜ Adapt collector to use PocketBase
6. ⬜ Create web interface for browsing
7. ⬜ Set up automated daily collection

---

## Legal & Ethical Considerations

- ✅ Public API (no authentication required)
- ✅ Respectful rate limiting implemented
- ✅ User-Agent header provided
- ⚠️ Check robots.txt for any restrictions
- ⚠️ Review terms of service before production use
- ⚠️ This is for personal/research use only

---

## Appendix: All Available Channels

Based on `/API/Channels` endpoint, 77 channels are available:

**Finnish Terrestrial:**
1. Yle TV1, Yle TV2, Yle Teema & Fem
2. MTV3, Nelonen, SubTV
3. Jim, Liv, Kutonen, Hero, Frii, Star Channel
4. TV Viisi, Ava

**Sports:**
- Eurosport 1, Eurosport 2
- V Sport 1, V Sport Golf, V Sport Football
- MTV Urheilu 1, MTV Urheilu 2

**Entertainment:**
- Discovery Channel, Animal Planet, TLC
- National Geographic, NatGeo Wild
- History, Viasat History, Viasat Explore
- BBC Earth, BBC Nordic

**Movies:**
- V Film Premiere, V Film Action, V Film Family, V Film Hits
- C More Hits, C More Stars
- MTV Aitio, MTV Viihde

**Kids:**
- Disney Channel, Disney Junior
- Cartoon Network, NickJr
- MTV Juniori

**Music:**
- MTV Finland, MTV 80s, MTV 90s, MTV 00s
- MTV Hits, MTV Live HD, Club MTV

**International:**
- CNN, CNBC, Al Jazeera
- Deutsche Welle, RTL, RTL 2
- TV4, Sjuan, TV7, SF Kanalen
- Travel

**Others:**
- ETV, TV Finland, Eveo

---

**Document Version:** 1.0
**Last Updated:** 2025-12-16
**Maintained By:** Research Project
