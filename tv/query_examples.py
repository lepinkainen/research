#!/usr/bin/env python3
"""
Example queries for TV Program Database
Demonstrates various ways to query the SQLite database
"""

from tv_database import TVDatabase
from datetime import datetime, timedelta
import json

def print_separator(title=""):
    """Print a nice separator"""
    if title:
        print(f"\n{'='*60}")
        print(f"  {title}")
        print(f"{'='*60}\n")
    else:
        print(f"{'='*60}\n")

def format_time(iso_time):
    """Format ISO time to HH:MM"""
    try:
        dt = datetime.fromisoformat(iso_time)
        return dt.strftime("%H:%M")
    except:
        return iso_time

def what_is_on_now():
    """Show what's currently airing on all channels"""
    print_separator("What's On Now")

    db = TVDatabase()
    programs = db.get_programs_now()

    if not programs:
        print("No programs currently airing (or no data in database)")
        return

    for program in programs:
        start = format_time(program['start_time'])
        end = format_time(program['end_time'])
        print(f"{program['channel_name']:20} {start}-{end}  {program['title']}")

def tonight_prime_time():
    """Show tonight's prime time programs (20:00-23:00)"""
    print_separator("Tonight's Prime Time (20:00-23:00)")

    db = TVDatabase()
    today = datetime.now().date().isoformat()
    programs = db.get_programs_by_date(today)

    prime_time = []
    for program in programs:
        start_time = program['start_time']
        # Extract hour from time string
        if 'T' in start_time:
            hour = int(start_time.split('T')[1].split(':')[0])
        else:
            try:
                hour = int(start_time.split(':')[0])
            except:
                continue

        if 20 <= hour < 23:
            prime_time.append(program)

    # Group by channel
    channels = {}
    for p in prime_time:
        ch = p['channel_name']
        if ch not in channels:
            channels[ch] = []
        channels[ch].append(p)

    for channel, progs in sorted(channels.items()):
        print(f"\n{channel}:")
        for p in progs:
            start = format_time(p['start_time'])
            print(f"  {start}  {p['title']}")

def search_shows(query):
    """Search for shows by title"""
    print_separator(f"Search Results for '{query}'")

    db = TVDatabase()
    results = db.search_programs(query)

    if not results:
        print(f"No programs found matching '{query}'")
        return

    for program in results[:20]:  # Limit to 20 results
        start = format_time(program['start_time'])
        date = program['start_time'].split('T')[0] if 'T' in program['start_time'] else ''
        print(f"{date} {start}  {program['channel_name']:15}  {program['title']}")

def shows_by_genre(genre):
    """Find shows by genre"""
    print_separator(f"Programs in Genre: {genre}")

    db = TVDatabase()
    programs = db.get_programs_by_genre(genre, limit=20)

    if not programs:
        print(f"No programs found for genre '{genre}'")
        return

    for program in programs:
        start = format_time(program['start_time'])
        date = program['start_time'].split('T')[0] if 'T' in program['start_time'] else ''
        print(f"{date} {start}  {program['channel_name']:15}  {program['title']}")

def channel_schedule(channel_name, date=None):
    """Show full schedule for a specific channel"""
    if date is None:
        date = datetime.now().date().isoformat()

    print_separator(f"Schedule for {channel_name} on {date}")

    db = TVDatabase()

    # Get channel ID by name
    channels = db.get_channels()
    channel_id = None
    for ch in channels:
        if channel_name.lower() in ch['name'].lower():
            channel_id = ch['id']
            channel_name = ch['name']
            break

    if not channel_id:
        print(f"Channel '{channel_name}' not found")
        return

    programs = db.get_programs_by_date(date, channel_id)

    if not programs:
        print(f"No programs found for {channel_name} on {date}")
        return

    for program in programs:
        start = format_time(program['start_time'])
        end = format_time(program['end_time'])
        duration = program.get('duration', '?')
        category = program.get('category', '')

        title = program['title']
        if program.get('episode'):
            title += f" (S{program.get('season', '?')}E{program['episode']})"

        info = f"{start}-{end}  {title}"
        if category:
            info += f"  [{category}]"

        print(info)

def database_statistics():
    """Show comprehensive database statistics"""
    print_separator("Database Statistics")

    db = TVDatabase()
    stats = db.get_statistics()

    print(f"Total Programs: {stats['total_programs']:,}")
    print(f"Total Channels: {stats['total_channels']}")

    if stats['date_range']['earliest']:
        print(f"\nDate Range:")
        print(f"  Earliest: {stats['date_range']['earliest']}")
        print(f"  Latest:   {stats['date_range']['latest']}")

    if 'last_fetch' in stats:
        print(f"\nLast Fetch:")
        print(f"  Date: {stats['last_fetch']['date']}")
        print(f"  Time: {stats['last_fetch']['time']}")

    if stats.get('programs_per_channel'):
        print(f"\nPrograms Per Channel:")
        for ch in stats['programs_per_channel'][:10]:
            print(f"  {ch['name']:30} {ch['count']:>6} programs")

    if stats.get('top_genres'):
        print(f"\nTop Genres:")
        for genre in stats['top_genres'][:10]:
            print(f"  {genre['name']:30} {genre['count']:>6} programs")

def list_all_channels():
    """List all channels in the database"""
    print_separator("All Channels")

    db = TVDatabase()
    channels = db.get_channels()

    for ch in channels:
        status = "✓" if ch['active'] else "✗"
        category = ch.get('category', 'N/A')
        print(f"{status} ID {ch['id']:3}  {ch['name']:30}  [{category}]")

def upcoming_series_episodes():
    """Show upcoming episodes of series"""
    print_separator("Upcoming Series Episodes")

    db = TVDatabase()

    # Get future programs that are series
    with db.get_connection() as conn:
        cursor = conn.execute("""
            SELECT p.*, c.name as channel_name
            FROM programs p
            JOIN channels c ON p.channel_id = c.id
            WHERE p.is_series = 1
              AND p.start_time >= datetime('now')
              AND p.episode IS NOT NULL
            ORDER BY p.start_time
            LIMIT 30
        """)

        programs = [dict(row) for row in cursor.fetchall()]

    if not programs:
        print("No upcoming series episodes found")
        return

    for program in programs:
        start = format_time(program['start_time'])
        date = program['start_time'].split('T')[0] if 'T' in program['start_time'] else ''

        title = program['title']
        if program.get('season') and program.get('episode'):
            title += f" (S{program['season']:02d}E{program['episode']:02d})"
        if program.get('episode_title'):
            title += f" - {program['episode_title']}"

        print(f"{date} {start}  {program['channel_name']:15}  {title}")

def movies_this_week():
    """Show movies airing this week"""
    print_separator("Movies This Week")

    db = TVDatabase()

    # Get programs categorized as movies
    start_date = datetime.now().date().isoformat()
    end_date = (datetime.now() + timedelta(days=7)).date().isoformat()

    with db.get_connection() as conn:
        cursor = conn.execute("""
            SELECT p.*, c.name as channel_name
            FROM programs p
            JOIN channels c ON p.channel_id = c.id
            WHERE (p.category LIKE '%movie%' OR p.category LIKE '%elokuva%')
              AND date(p.start_time) >= ?
              AND date(p.start_time) <= ?
            ORDER BY p.start_time
            LIMIT 50
        """, (start_date, end_date))

        movies = [dict(row) for row in cursor.fetchall()]

    if not movies:
        print("No movies found this week")
        print("Note: This depends on the category field being populated correctly")
        return

    for movie in movies:
        start = format_time(movie['start_time'])
        date = movie['start_time'].split('T')[0] if 'T' in movie['start_time'] else ''

        title = movie['title']
        if movie.get('year'):
            title += f" ({movie['year']})"

        print(f"{date} {start}  {movie['channel_name']:15}  {title}")

def export_to_json(output_file="programs_export.json"):
    """Export today's programs to JSON"""
    print_separator(f"Exporting to {output_file}")

    db = TVDatabase()
    today = datetime.now().date().isoformat()
    programs = db.get_programs_by_date(today)

    with open(output_file, 'w', encoding='utf-8') as f:
        json.dump(programs, f, indent=2, ensure_ascii=False)

    print(f"✓ Exported {len(programs)} programs to {output_file}")

# Main menu
if __name__ == "__main__":
    import sys

    print("""
╔════════════════════════════════════════════════════════════╗
║         TV Program Database Query Examples                 ║
╚════════════════════════════════════════════════════════════╝
    """)

    # Check if database exists
    db = TVDatabase()
    stats = db.get_statistics()

    if stats['total_programs'] == 0:
        print("⚠️  Database is empty!")
        print("   Run 'python collector.py' to fetch program data first.\n")
        sys.exit(1)

    print(f"Database contains {stats['total_programs']:,} programs from {stats['total_channels']} channels\n")

    examples = {
        '1': ('What\'s on now?', what_is_on_now),
        '2': ('Tonight\'s prime time (20:00-23:00)', tonight_prime_time),
        '3': ('Search shows', lambda: search_shows(input('Search query: '))),
        '4': ('Shows by genre', lambda: shows_by_genre(input('Genre name: '))),
        '5': ('Channel schedule', lambda: channel_schedule(input('Channel name: '))),
        '6': ('Upcoming series episodes', upcoming_series_episodes),
        '7': ('Movies this week', movies_this_week),
        '8': ('List all channels', list_all_channels),
        '9': ('Database statistics', database_statistics),
        '0': ('Export today\'s programs to JSON', export_to_json),
        'a': ('Run all examples', None),
    }

    print("Choose an example:")
    for key, (description, _) in examples.items():
        if key != 'a':
            print(f"  {key}) {description}")
    print(f"  a) Run all examples (except search/input-based)")
    print()

    choice = input("Enter choice (or press Enter to run all): ").strip().lower()

    if choice == '' or choice == 'a':
        # Run all non-interactive examples
        what_is_on_now()
        tonight_prime_time()
        upcoming_series_episodes()
        movies_this_week()
        list_all_channels()
        database_statistics()
    elif choice in examples and examples[choice][1]:
        examples[choice][1]()
    else:
        print("Invalid choice")

    print("\n✓ Done!\n")
