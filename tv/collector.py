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
from tv_database import TVDatabase

class TelkussaCollector:
    BASE_URL = "https://telkussa.fi/API"
    HEADERS = {
        'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36',
        'Accept': 'application/json',
        'Accept-Language': 'fi-FI,fi;q=0.9,en;q=0.8',
    }

    # List of channels to fetch (update after running api_explorer.py)
    # This is a placeholder - replace with actual channels after discovery
    CHANNELS = [
        {'id': 1, 'name': 'YLE TV1', 'category': 'public'},
        {'id': 2, 'name': 'YLE TV2', 'category': 'public'},
        {'id': 3, 'name': 'MTV3', 'category': 'commercial'},
        {'id': 4, 'name': 'Nelonen', 'category': 'commercial'},
        {'id': 13, 'name': 'Channel 13', 'category': 'unknown'},
        # Add more channels after running api_explorer.py
    ]

    def __init__(self, db_path="tv_programs.db"):
        self.db = TVDatabase(db_path)
        self.session = requests.Session()
        self.session.headers.update(self.HEADERS)

        # Setup logging
        log_dir = Path("logs")
        log_dir.mkdir(exist_ok=True)

        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(levelname)s - %(message)s',
            handlers=[
                logging.FileHandler(log_dir / 'collector.log'),
                logging.StreamHandler()
            ]
        )
        self.logger = logging.getLogger(__name__)

    def fetch_channel_data(self, channel_id, date_str, retry_count=3):
        """Fetch program data for a specific channel and date"""
        url = f"{self.BASE_URL}/Channel/{channel_id}/{date_str}"

        for attempt in range(retry_count):
            try:
                response = self.session.get(url, timeout=15)
                response.raise_for_status()
                return response.json()
            except requests.RequestException as e:
                if attempt < retry_count - 1:
                    wait_time = (attempt + 1) * 2  # Exponential backoff
                    self.logger.warning(
                        f"Attempt {attempt + 1} failed for channel {channel_id} on {date_str}. "
                        f"Retrying in {wait_time}s... Error: {e}"
                    )
                    sleep(wait_time)
                else:
                    self.logger.error(
                        f"Failed to fetch channel {channel_id} for {date_str} after {retry_count} attempts: {e}"
                    )
                    return None

        return None

    def parse_and_store_programs(self, channel_id, data, date_str):
        """
        Parse API response and store programs in database

        NOTE: This method needs to be adapted based on the actual API structure
        discovered by running api_explorer.py
        """
        if not data:
            return 0

        # Extract programs from response
        # Adapt this based on actual API structure
        programs = data.get('programs', [])
        if not programs and isinstance(data, list):
            programs = data

        stored_count = 0

        for program in programs:
            try:
                # Build program data dict
                # Adapt field names based on actual API response
                program_data = {
                    'external_id': self._get_program_id(program, channel_id, date_str),
                    'channel_id': channel_id,
                    'title': program.get('title') or program.get('name'),
                    'description': program.get('description') or program.get('desc'),
                    'start_time': self._parse_time(program.get('startTime') or program.get('start')),
                    'end_time': self._parse_time(program.get('endTime') or program.get('end')),
                    'duration': program.get('duration') or program.get('length'),
                    'category': program.get('category') or program.get('type'),
                    'is_series': program.get('series', False) or program.get('isSeries', False),
                    'season': program.get('season'),
                    'episode': program.get('episode'),
                    'episode_title': program.get('episodeTitle') or program.get('episodeName'),
                    'age_rating': program.get('ageRating') or program.get('rating'),
                    'image_url': program.get('image') or program.get('imageUrl'),
                    'year': program.get('year'),
                    'country': program.get('country'),
                    'is_rerun': program.get('rerun', False) or program.get('isRerun', False),
                }

                # Add genres if available
                if 'genres' in program and program['genres']:
                    program_data['genres'] = program['genres']
                elif 'genre' in program:
                    program_data['genres'] = [program['genre']]

                # Add people (actors, directors) if available
                people = []
                if 'actors' in program:
                    people.extend([{'name': actor, 'role': 'actor'} for actor in program['actors']])
                if 'director' in program:
                    people.append({'name': program['director'], 'role': 'director'})
                if people:
                    program_data['people'] = people

                # Store in database
                if self.db.insert_program(program_data):
                    stored_count += 1

            except Exception as e:
                self.logger.error(
                    f"Error parsing program {program.get('title', 'Unknown')} "
                    f"on channel {channel_id}: {e}"
                )
                continue

        return stored_count

    def _get_program_id(self, program, channel_id, date_str):
        """Generate a unique program ID"""
        if 'id' in program:
            return str(program['id'])

        # Generate ID from title and start time
        title = program.get('title', program.get('name', 'unknown'))
        start = program.get('startTime', program.get('start', ''))
        return f"{channel_id}_{date_str}_{title}_{start}".replace(' ', '_').replace(':', '')

    def _parse_time(self, time_str):
        """
        Parse time string to ISO format

        Adapt this based on actual time format from API
        Possible formats:
        - ISO: 2025-12-16T18:00:00
        - Time only: 18:00
        - Unix timestamp: 1734364800
        """
        if not time_str:
            return None

        # If already ISO format, return as is
        if 'T' in str(time_str):
            return time_str

        # If Unix timestamp
        if isinstance(time_str, int):
            return datetime.fromtimestamp(time_str).isoformat()

        # Add more parsing logic based on actual API format
        return time_str

    def collect_daily_data(self, date=None, days_ahead=0):
        """Collect data for a specific date (default: today) and days ahead"""
        if date is None:
            date = datetime.now()

        total_programs = 0

        # Collect for today and N days ahead
        for day_offset in range(days_ahead + 1):
            target_date = date + timedelta(days=day_offset)
            date_str = target_date.strftime("%Y%m%d")

            self.logger.info(f"{'='*50}")
            self.logger.info(f"Collecting data for {date_str}")
            self.logger.info(f"{'='*50}")

            date_programs = 0

            for channel in self.CHANNELS:
                channel_id = channel['id']
                channel_name = channel['name']

                self.logger.info(f"  Fetching {channel_name} (ID: {channel_id})")

                data = self.fetch_channel_data(channel_id, date_str)

                if data:
                    count = self.parse_and_store_programs(channel_id, data, date_str)
                    self.db.log_fetch(channel_id, date_str, True, count)
                    self.logger.info(f"    ✓ Stored {count} programs")
                    date_programs += count
                else:
                    self.db.log_fetch(channel_id, date_str, False, 0, "Fetch failed")
                    self.logger.warning(f"    ✗ Failed to fetch data")

                # Rate limiting - be nice to the server
                sleep(1)

            total_programs += date_programs
            self.logger.info(f"Completed {date_str}: {date_programs} programs stored\n")

        return total_programs

    def update_channels(self):
        """Update channel information in database"""
        self.logger.info("Updating channel information...")
        for channel in self.CHANNELS:
            self.db.upsert_channel(
                channel['id'],
                channel['name'],
                channel.get('logo_url'),
                channel.get('category')
            )
        self.logger.info(f"Updated {len(self.CHANNELS)} channels")

    def cleanup_old_data(self, days_to_keep=30):
        """Remove old program data"""
        self.logger.info(f"Cleaning up programs older than {days_to_keep} days...")
        deleted = self.db.cleanup_old_programs(days_to_keep)
        self.logger.info(f"Removed {deleted} old programs")

if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="Collect TV program data from telkussa.fi")
    parser.add_argument(
        '--days-ahead',
        type=int,
        default=7,
        help='Number of days ahead to fetch (default: 7)'
    )
    parser.add_argument(
        '--db-path',
        type=str,
        default='tv_programs.db',
        help='Path to SQLite database (default: tv_programs.db)'
    )
    parser.add_argument(
        '--cleanup',
        action='store_true',
        help='Clean up old data (older than 30 days)'
    )
    parser.add_argument(
        '--update-channels-only',
        action='store_true',
        help='Only update channel information, don\'t fetch programs'
    )

    args = parser.parse_args()

    collector = TelkussaCollector(args.db_path)

    print("=" * 60)
    print("Telkussa TV Program Collector")
    print("=" * 60)
    print()

    # Update channels first
    collector.update_channels()

    if args.update_channels_only:
        print("\n✓ Channels updated. Exiting.")
        exit(0)

    # Collect program data
    print(f"\nCollecting today's data + {args.days_ahead} days ahead...")
    total = collector.collect_daily_data(days_ahead=args.days_ahead)

    # Cleanup if requested
    if args.cleanup:
        collector.cleanup_old_data()

    # Print statistics
    print("\n" + "=" * 60)
    print("Collection Complete!")
    print("=" * 60)

    stats = collector.db.get_statistics()
    print(f"\nDatabase Statistics:")
    print(f"  Total programs: {stats['total_programs']}")
    print(f"  Total channels: {stats['total_channels']}")
    print(f"  Date range: {stats['date_range']['earliest']} to {stats['date_range']['latest']}")

    if 'last_fetch' in stats:
        print(f"  Last fetch: {stats['last_fetch']['date']} at {stats['last_fetch']['time']}")

    print(f"\n  Programs stored in this run: {total}")

    if stats.get('programs_per_channel'):
        print("\n  Programs per channel:")
        for ch in stats['programs_per_channel'][:5]:
            print(f"    {ch['name']}: {ch['count']}")

    if stats.get('top_genres'):
        print("\n  Top genres:")
        for genre in stats['top_genres'][:5]:
            print(f"    {genre['name']}: {genre['count']}")

    print("\n✓ Done!")
