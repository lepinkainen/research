#!/usr/bin/env python3
"""
PocketBase TV Program Data Collector
Fetches TV program data from telkussa.fi and stores in PocketBase
"""

import os
import sys
import requests
import json
from datetime import datetime, timedelta
from time import sleep
import logging
from pathlib import Path
from typing import Dict, List, Optional, Tuple
from dotenv import load_dotenv

# Load environment variables
load_dotenv()


class PocketBaseClient:
    """Simple PocketBase API client"""

    def __init__(self, base_url: str, admin_email: str = None, admin_password: str = None):
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
        self.session.headers.update({
            'Content-Type': 'application/json',
        })
        self.admin_token = None

        # Authenticate as admin if credentials provided
        if admin_email and admin_password:
            self.authenticate_admin(admin_email, admin_password)

    def authenticate_admin(self, email: str, password: str) -> bool:
        """Authenticate as admin user"""
        try:
            response = self.session.post(
                f"{self.base_url}/api/admins/auth-with-password",
                json={'identity': email, 'password': password}
            )
            response.raise_for_status()
            data = response.json()
            self.admin_token = data['token']
            self.session.headers.update({
                'Authorization': f"Admin {self.admin_token}"
            })
            return True
        except requests.exceptions.RequestException as e:
            logging.error(f"Authentication failed: {e}")
            return False

    def get_records(self, collection: str, filter: str = None, expand: str = None,
                    sort: str = None, page: int = 1, per_page: int = 50) -> List[Dict]:
        """Get records from a collection"""
        params = {
            'page': page,
            'perPage': per_page,
        }
        if filter:
            params['filter'] = filter
        if expand:
            params['expand'] = expand
        if sort:
            params['sort'] = sort

        try:
            response = self.session.get(
                f"{self.base_url}/api/collections/{collection}/records",
                params=params
            )
            response.raise_for_status()
            return response.json().get('items', [])
        except requests.exceptions.RequestException as e:
            logging.error(f"Failed to get records from {collection}: {e}")
            return []

    def create_record(self, collection: str, data: Dict) -> Optional[Dict]:
        """Create a new record"""
        try:
            response = self.session.post(
                f"{self.base_url}/api/collections/{collection}/records",
                json=data
            )
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logging.error(f"Failed to create record in {collection}: {e}")
            return None

    def update_record(self, collection: str, record_id: str, data: Dict) -> Optional[Dict]:
        """Update an existing record"""
        try:
            response = self.session.patch(
                f"{self.base_url}/api/collections/{collection}/records/{record_id}",
                json=data
            )
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logging.error(f"Failed to update record {record_id} in {collection}: {e}")
            return None

    def get_record_by_id(self, collection: str, record_id: str) -> Optional[Dict]:
        """Get a single record by ID"""
        try:
            response = self.session.get(
                f"{self.base_url}/api/collections/{collection}/records/{record_id}"
            )
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException:
            return None


class TelkussaPocketBaseCollector:
    """Collects TV program data and stores in PocketBase"""

    API_BASE_URL = "https://telkussa.fi/API"
    HEADERS = {
        'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36',
        'Accept': 'application/json',
        'Accept-Language': 'fi-FI,fi;q=0.9,en;q=0.8',
    }

    def __init__(self, pocketbase_url: str, admin_email: str, admin_password: str):
        self.pb = PocketBaseClient(pocketbase_url, admin_email, admin_password)
        self.session = requests.Session()
        self.session.headers.update(self.HEADERS)

        # Setup logging
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(levelname)s - %(message)s',
            handlers=[
                logging.FileHandler('logs/pocketbase_collector.log'),
                logging.StreamHandler()
            ]
        )
        self.logger = logging.getLogger(__name__)

    def fetch_channel_programs(self, channel_id: str, date_str: str) -> Tuple[bool, List[Dict]]:
        """Fetch program data for a specific channel and date"""
        url = f"{self.API_BASE_URL}/Channel/{channel_id}/{date_str}"

        try:
            response = self.session.get(url, timeout=15)
            response.raise_for_status()
            data = response.json()

            if not isinstance(data, list):
                self.logger.warning(f"Unexpected response format for channel {channel_id}")
                return False, []

            return True, data

        except requests.RequestException as e:
            self.logger.error(f"Failed to fetch channel {channel_id} for {date_str}: {e}")
            return False, []

    def convert_timestamp(self, timestamp: int) -> str:
        """
        Convert API timestamp to ISO 8601 format

        Note: The API uses custom timestamps that need conversion.
        Adjust this function based on the actual timestamp format.
        """
        # TODO: Determine the actual timestamp format from the API
        # For now, assuming Unix timestamp
        try:
            dt = datetime.fromtimestamp(timestamp)
            return dt.strftime('%Y-%m-%d %H:%M:%S')
        except (ValueError, OSError):
            # If timestamp is invalid, return current time
            return datetime.now().strftime('%Y-%m-%d %H:%M:%S')

    def store_program(self, program: Dict, channel_id: str) -> bool:
        """Store a single program in PocketBase"""
        program_id = str(program.get('id'))

        # Check if program already exists
        existing = self.pb.get_record_by_id('programs', program_id)

        # Convert timestamps
        start_time = self.convert_timestamp(program.get('start', 0))
        end_time = self.convert_timestamp(program.get('stop', 0))

        # Calculate duration in minutes
        try:
            start_dt = datetime.strptime(start_time, '%Y-%m-%d %H:%M:%S')
            end_dt = datetime.strptime(end_time, '%Y-%m-%d %H:%M:%S')
            duration = int((end_dt - start_dt).total_seconds() / 60)
        except ValueError:
            duration = 0

        # Prepare program data
        program_data = {
            'id': program_id,
            'channel': channel_id,
            'name': program.get('name', ''),
            'episode': program.get('episode', ''),
            'description': program.get('description', ''),
            'start_time': start_time,
            'end_time': end_time,
            'duration': duration,
            'age_limit': program.get('agelimit', 0),
            'rating': program.get('rating', 0),
            'is_series': program.get('series_id', 0) > 0,
        }

        # Add series relation if applicable
        series_id = program.get('series_id', 0)
        if series_id > 0:
            program_data['series'] = str(series_id)

        # Create or update record
        if existing:
            result = self.pb.update_record('programs', program_id, program_data)
        else:
            result = self.pb.create_record('programs', program_data)

        return result is not None

    def update_series(self, series_id: int, program_name: str) -> bool:
        """Create or update series record"""
        series_id_str = str(series_id)

        # Check if series exists
        existing = self.pb.get_record_by_id('series', series_id_str)

        series_data = {
            'id': series_id_str,
            'name': program_name,
            'active': True,
            'last_seen': datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        }

        if not existing:
            series_data['first_seen'] = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
            series_data['episode_count'] = 0

        # Create or update
        if existing:
            result = self.pb.update_record('series', series_id_str, series_data)
        else:
            result = self.pb.create_record('series', series_data)

        return result is not None

    def log_fetch(self, channel_id: str, target_date: str, success: bool,
                  programs_count: int = 0, error_msg: str = None,
                  duration_ms: int = 0) -> bool:
        """Log fetch operation"""
        log_data = {
            'channel': channel_id if channel_id else None,
            'target_date': target_date,
            'success': success,
            'programs_count': programs_count,
            'error_message': error_msg or '',
            'duration_ms': duration_ms
        }

        result = self.pb.create_record('fetch_logs', log_data)
        return result is not None

    def collect_daily_data(self, days_ahead: int = 7) -> None:
        """Collect program data for active channels"""
        # Get active channels
        channels = self.pb.get_records('channels', filter='active = true', sort='show_order')

        if not channels:
            self.logger.warning("No active channels found")
            return

        self.logger.info(f"📊 Starting data collection for {len(channels)} channels")

        # Collect for today + N days ahead
        today = datetime.now()

        for day_offset in range(days_ahead + 1):
            target_date = today + timedelta(days=day_offset)
            date_str = target_date.strftime("%Y%m%d")

            self.logger.info(f"📅 Collecting programs for {target_date.strftime('%Y-%m-%d')}")

            for channel in channels:
                channel_id = channel['id']
                channel_name = channel['name']

                self.logger.info(f"  📺 Fetching {channel_name}")

                start_time = datetime.now()

                # Fetch programs
                success, programs = self.fetch_channel_programs(channel_id, date_str)

                duration_ms = int((datetime.now() - start_time).total_seconds() * 1000)

                if not success:
                    self.log_fetch(channel_id, date_str, False, 0,
                                   "Failed to fetch data", duration_ms)
                    continue

                # Store programs
                stored = 0
                series_map = {}

                for program in programs:
                    if self.store_program(program, channel_id):
                        stored += 1

                    # Track series
                    series_id = program.get('series_id', 0)
                    if series_id > 0:
                        series_map[series_id] = program.get('name', '')

                # Update series records
                for series_id, program_name in series_map.items():
                    self.update_series(series_id, program_name)

                self.logger.info(f"    ✅ Stored {stored}/{len(programs)} programs")

                # Log success
                self.log_fetch(channel_id, date_str, True, stored, None, duration_ms)

                # Rate limiting
                sleep(1.0)

        self.logger.info("✅ Data collection completed")

    def update_channel_list(self) -> None:
        """Fetch and update channel list from API"""
        self.logger.info("📡 Updating channel list...")

        try:
            response = self.session.get(f"{self.API_BASE_URL}/Channels", timeout=15)
            response.raise_for_status()
            channels = response.json()

            if not isinstance(channels, list):
                self.logger.error("Invalid channel list response")
                return

            self.logger.info(f"Found {len(channels)} channels")

            for channel in channels:
                channel_id = str(channel.get('id'))
                channel_name = channel.get('name', '')
                show_order = channel.get('showOrder', 0)

                # Check if channel exists
                existing = self.pb.get_record_by_id('channels', channel_id)

                channel_data = {
                    'id': channel_id,
                    'name': channel_name,
                    'show_order': show_order,
                }

                # Only set active=true for new channels
                if not existing:
                    channel_data['active'] = True

                # Create or update
                if existing:
                    self.pb.update_record('channels', channel_id, channel_data)
                else:
                    self.pb.create_record('channels', channel_data)

            self.logger.info("✅ Channel list updated")

        except requests.RequestException as e:
            self.logger.error(f"Failed to fetch channel list: {e}")

    def cleanup_old_data(self, days: int = 30) -> None:
        """Remove programs older than specified days"""
        self.logger.info(f"🧹 Cleaning up programs older than {days} days...")

        cutoff_date = (datetime.now() - timedelta(days=days)).strftime('%Y-%m-%d %H:%M:%S')

        # Get old programs
        old_programs = self.pb.get_records(
            'programs',
            filter=f'start_time < "{cutoff_date}"',
            per_page=500
        )

        deleted = 0
        for program in old_programs:
            # Delete using direct API call since SDK doesn't have delete method
            try:
                response = self.pb.session.delete(
                    f"{self.pb.base_url}/api/collections/programs/records/{program['id']}"
                )
                if response.status_code == 204:
                    deleted += 1
            except Exception as e:
                self.logger.error(f"Failed to delete program {program['id']}: {e}")

        self.logger.info(f"🗑️  Deleted {deleted} old programs")

        # Cleanup old fetch logs
        self.logger.info(f"🧹 Cleaning up fetch logs older than {days} days...")

        log_cutoff = (datetime.now() - timedelta(days=days)).strftime('%Y-%m-%d %H:%M:%S')

        old_logs = self.pb.get_records(
            'fetch_logs',
            filter=f'created < "{log_cutoff}"',
            per_page=500
        )

        deleted_logs = 0
        for log in old_logs:
            try:
                response = self.pb.session.delete(
                    f"{self.pb.base_url}/api/collections/fetch_logs/records/{log['id']}"
                )
                if response.status_code == 204:
                    deleted_logs += 1
            except Exception as e:
                self.logger.error(f"Failed to delete log {log['id']}: {e}")

        self.logger.info(f"🗑️  Deleted {deleted_logs} old fetch logs")


def main():
    """Main entry point"""
    # Get configuration from environment
    pocketbase_url = os.getenv('POCKETBASE_URL', 'http://127.0.0.1:8090')
    admin_email = os.getenv('POCKETBASE_ADMIN_EMAIL')
    admin_password = os.getenv('POCKETBASE_ADMIN_PASSWORD')
    days_ahead = int(os.getenv('FETCH_DAYS_AHEAD', '7'))

    if not admin_email or not admin_password:
        print("Error: POCKETBASE_ADMIN_EMAIL and POCKETBASE_ADMIN_PASSWORD must be set")
        print("Create a .env file with these variables or export them")
        sys.exit(1)

    # Ensure logs directory exists
    Path('logs').mkdir(exist_ok=True)

    # Create collector
    collector = TelkussaPocketBaseCollector(
        pocketbase_url,
        admin_email,
        admin_password
    )

    # Parse command line arguments
    if len(sys.argv) > 1:
        command = sys.argv[1]

        if command == '--update-channels':
            collector.update_channel_list()
        elif command == '--cleanup':
            days = int(sys.argv[2]) if len(sys.argv) > 2 else 30
            collector.cleanup_old_data(days)
        elif command == '--help':
            print("Usage: pocketbase_collector.py [OPTIONS]")
            print("\nOptions:")
            print("  (no args)              Collect TV program data")
            print("  --update-channels      Update channel list from API")
            print("  --cleanup [DAYS]       Delete old data (default: 30 days)")
            print("  --help                 Show this help message")
        else:
            print(f"Unknown command: {command}")
            print("Use --help for usage information")
    else:
        # Default: collect program data
        collector.collect_daily_data(days_ahead)


if __name__ == "__main__":
    main()
