#!/usr/bin/env python3
"""
Download Hacker News comments for a user using multiple fallback methods.
This version tries multiple APIs and includes better error handling.
"""

import requests
import json
import time
from datetime import datetime, timedelta
from typing import List, Dict, Optional
import sys


class HNCommentDownloader:
    def __init__(self, username: str):
        self.username = username
        self.session = requests.Session()
        self.session.headers.update({
            'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
        })

    def try_algolia_api(self, days_ago: int = 180) -> Optional[List[Dict]]:
        """Try to fetch comments using Algolia HN API."""
        print("Attempting to fetch via Algolia API...")

        cutoff_date = datetime.now() - timedelta(days=days_ago)
        cutoff_timestamp = int(cutoff_date.timestamp())

        base_url = "https://hn.algolia.com/api/v1/search_by_date"
        all_comments = []
        page = 0

        while True:
            params = {
                'tags': f'comment,author_{self.username}',
                'numericFilters': f'created_at_i>{cutoff_timestamp}',
                'hitsPerPage': 100,
                'page': page
            }

            try:
                response = self.session.get(base_url, params=params, timeout=10)
                response.raise_for_status()
                data = response.json()

                hits = data.get('hits', [])
                if not hits:
                    break

                all_comments.extend(hits)
                print(f"  Page {page}: fetched {len(hits)} comments (total: {len(all_comments)})")

                if page >= data.get('nbPages', 0) - 1:
                    break

                page += 1
                time.sleep(0.3)

            except Exception as e:
                print(f"  Algolia API error: {e}")
                return None if page == 0 else all_comments

        return all_comments

    def try_firebase_api(self) -> Optional[List[Dict]]:
        """Try to fetch user data from HN Firebase API."""
        print("Attempting to fetch via Firebase API...")

        try:
            # Get user info
            user_url = f"https://hacker-news.firebaseio.com/v0/user/{self.username}.json"
            response = self.session.get(user_url, timeout=10)
            response.raise_for_status()
            user_data = response.json()

            submitted = user_data.get('submitted', [])
            print(f"  Found {len(submitted)} submitted items")

            # Fetch each item and filter for comments
            comments = []
            for i, item_id in enumerate(submitted[:500]):  # Limit to recent 500 items
                item_url = f"https://hacker-news.firebaseio.com/v0/item/{item_id}.json"
                try:
                    item_response = self.session.get(item_url, timeout=5)
                    item_response.raise_for_status()
                    item = item_response.json()

                    if item and item.get('type') == 'comment':
                        comments.append(item)
                        if (i + 1) % 50 == 0:
                            print(f"  Processed {i + 1} items, found {len(comments)} comments")

                    time.sleep(0.1)  # Rate limiting

                except Exception as e:
                    print(f"  Error fetching item {item_id}: {e}")
                    continue

            print(f"  Total comments found: {len(comments)}")
            return comments

        except Exception as e:
            print(f"  Firebase API error: {e}")
            return None

    def save_comments(self, comments: List[Dict], output_file: str):
        """Save comments to JSON file."""
        with open(output_file, 'w', encoding='utf-8') as f:
            json.dump(comments, f, indent=2, ensure_ascii=False)
        print(f"\nComments saved to {output_file}")

    def print_stats(self, comments: List[Dict]):
        """Print statistics about the comments."""
        if not comments:
            print("No comments to analyze.")
            return

        print("\n" + "="*70)
        print("COMMENT STATISTICS")
        print("="*70)
        print(f"Total comments: {len(comments)}")

        # Determine API format
        if comments[0].get('story_title'):
            # Algolia format
            self._print_algolia_stats(comments)
        else:
            # Firebase format
            self._print_firebase_stats(comments)

    def _print_algolia_stats(self, comments: List[Dict]):
        """Print stats for Algolia format."""
        stories = {}
        for comment in comments:
            story_id = comment.get('story_id')
            story_title = comment.get('story_title', 'Unknown')
            if story_id:
                stories[story_id] = story_title

        print(f"Unique stories: {len(stories)}")

        dates = [datetime.fromtimestamp(c.get('created_at_i', 0)) for c in comments if c.get('created_at_i')]
        if dates:
            dates.sort()
            print(f"Date range: {dates[0].strftime('%Y-%m-%d')} to {dates[-1].strftime('%Y-%m-%d')}")

        # Show sample
        if comments:
            sample = comments[0]
            print("\nSample comment:")
            print(f"  Story: {sample.get('story_title', 'N/A')}")
            print(f"  Text: {sample.get('comment_text', 'N/A')[:150]}...")
            print(f"  URL: https://news.ycombinator.com/item?id={sample.get('objectID', '')}")

    def _print_firebase_stats(self, comments: List[Dict]):
        """Print stats for Firebase format."""
        dates = [datetime.fromtimestamp(c.get('time', 0)) for c in comments if c.get('time')]
        if dates:
            dates.sort()
            print(f"Date range: {dates[0].strftime('%Y-%m-%d')} to {dates[-1].strftime('%Y-%m-%d')}")

        # Show sample
        if comments:
            sample = comments[0]
            print("\nSample comment:")
            print(f"  ID: {sample.get('id', 'N/A')}")
            print(f"  Text: {sample.get('text', 'N/A')[:150]}...")
            print(f"  Parent: {sample.get('parent', 'N/A')}")
            print(f"  URL: https://news.ycombinator.com/item?id={sample.get('id', '')}")


def main():
    username = 'theshrike79'
    days_back = 180
    output_file = 'blog/hn_comments.json'

    downloader = HNCommentDownloader(username)

    # Try Algolia first (more complete data)
    comments = downloader.try_algolia_api(days_back)

    # Fallback to Firebase API
    if not comments:
        print("\nAlgolia failed, trying Firebase API...")
        comments = downloader.try_firebase_api()

    if not comments:
        print("\n❌ Failed to fetch comments from all available APIs.")
        print("This might be due to network restrictions or API changes.")
        print("\nAlternative: You can manually export your comments and use the")
        print("categorize_comments.py script to process them.")
        sys.exit(1)

    # Save and display stats
    downloader.save_comments(comments, output_file)
    downloader.print_stats(comments)

    print("\n✅ Success! Comments downloaded.")
    print(f"Next step: Run 'python3 blog/categorize_comments.py' to group and summarize")


if __name__ == '__main__':
    main()
