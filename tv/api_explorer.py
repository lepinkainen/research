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

        def get_structure(obj, prefix="", indent=0):
            """Recursively analyze object structure"""
            spaces = "  " * indent
            if isinstance(obj, dict):
                for key, value in obj.items():
                    full_key = f"{prefix}.{key}" if prefix else key
                    value_type = type(value).__name__
                    if isinstance(value, list):
                        print(f"{spaces}{full_key}: [{value_type}] (length: {len(value)})")
                        if value:
                            get_structure(value[0], full_key, indent + 1)
                    elif isinstance(value, dict):
                        print(f"{spaces}{full_key}: {{{value_type}}}")
                        get_structure(value, full_key, indent + 1)
                    else:
                        print(f"{spaces}{full_key}: {value_type} = {repr(value)[:50]}")
            elif isinstance(obj, list) and obj:
                print(f"{spaces}{prefix}[0]: {type(obj[0]).__name__}")
                get_structure(obj[0], prefix, indent)

        print("\n=== Data Structure Analysis ===")
        get_structure(data)

    def test_additional_endpoints(self):
        """Test potential additional API endpoints"""
        endpoints_to_test = [
            "/Channels",
            "/Programs",
            "/Now",
            "/Schedule/" + datetime.now().strftime("%Y%m%d"),
        ]

        results = {}
        for endpoint in endpoints_to_test:
            url = f"{self.BASE_URL}{endpoint}"
            print(f"\nTesting: {url}")
            try:
                response = self.session.get(url, timeout=10)
                if response.status_code == 200:
                    print(f"✓ Success!")
                    data = response.json()
                    filename = self.output_dir / f"endpoint_{endpoint.replace('/', '_')}.json"
                    with open(filename, 'w', encoding='utf-8') as f:
                        json.dump(data, f, indent=2, ensure_ascii=False)
                    results[endpoint] = "success"
                else:
                    print(f"✗ HTTP {response.status_code}")
                    results[endpoint] = f"http_{response.status_code}"
            except Exception as e:
                print(f"✗ Error: {e}")
                results[endpoint] = f"error: {e}"

            sleep(0.5)

        return results

if __name__ == "__main__":
    explorer = TelkussaExplorer()

    print("=== Telkussa API Explorer ===\n")
    print("This script will explore the telkussa.fi API structure.")
    print("Results will be saved to 'data/samples' directory.\n")

    # Test a known channel
    print("1. Testing known channel (13)...")
    success, data = explorer.test_channel(13)
    if not success:
        print("\nWarning: Could not fetch channel 13. API might be unreachable or require different authentication.")
        print("Continuing with other tests...\n")

    # Test additional endpoints
    print("\n2. Testing additional endpoints...")
    endpoint_results = explorer.test_additional_endpoints()

    # Discover channels
    print("\n3. Discovering channels (1-50)...")
    print("This will take a while (about 25 seconds with rate limiting)...")
    channels = explorer.discover_channels(1, 50)
    print(f"\nFound {len(channels)} valid channels")

    # Test date range (only if we found channels)
    if channels:
        print("\n4. Testing date range availability...")
        print("Testing past and future dates (this will take a few minutes)...")
        date_results = explorer.test_date_range(channels[0]['id'])

        # Analyze results
        past_available = sum(1 for r in date_results['past'] if r['available'])
        future_available = sum(1 for r in date_results['future'] if r['available'])
        print(f"\nDate range results:")
        print(f"  Past: {past_available}/30 days available")
        print(f"  Future: {future_available}/30 days available")

    # Analyze structure
    print("\n5. Analyzing data structure...")
    sample_files = list(Path("data/samples").glob("channel_*.json"))
    if sample_files:
        explorer.analyze_structure(sample_files[0])
    else:
        print("No sample files found to analyze.")

    print("\n" + "="*50)
    print("✓ Exploration complete!")
    print(f"Check '{explorer.output_dir}' directory for detailed results.")
    print("\nNext steps:")
    print("1. Review the discovered channels in 'discovered_channels.json'")
    print("2. Check the data structure in sample JSON files")
    print("3. Update collector.py with the correct channel list")
    print("4. Adapt the data parsing based on actual API structure")
