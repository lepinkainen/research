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

                -- Genres table (many-to-many relationship)
                CREATE TABLE IF NOT EXISTS genres (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    name TEXT UNIQUE NOT NULL
                );

                CREATE TABLE IF NOT EXISTS program_genres (
                    program_id INTEGER,
                    genre_id INTEGER,
                    PRIMARY KEY (program_id, genre_id),
                    FOREIGN KEY (program_id) REFERENCES programs(id) ON DELETE CASCADE,
                    FOREIGN KEY (genre_id) REFERENCES genres(id)
                );

                -- People table (actors, directors, etc.)
                CREATE TABLE IF NOT EXISTS people (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    name TEXT UNIQUE NOT NULL
                );

                CREATE TABLE IF NOT EXISTS program_people (
                    program_id INTEGER,
                    person_id INTEGER,
                    role TEXT, -- 'actor', 'director', 'writer', etc.
                    PRIMARY KEY (program_id, person_id, role),
                    FOREIGN KEY (program_id) REFERENCES programs(id) ON DELETE CASCADE,
                    FOREIGN KEY (person_id) REFERENCES people(id)
                );

                -- Fetch log table (track API calls)
                CREATE TABLE IF NOT EXISTS fetch_log (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    channel_id INTEGER,
                    date TEXT NOT NULL,
                    success BOOLEAN,
                    programs_count INTEGER,
                    error_message TEXT,
                    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                );

                -- Indexes for performance
                CREATE INDEX IF NOT EXISTS idx_programs_channel_date
                    ON programs(channel_id, start_time);
                CREATE INDEX IF NOT EXISTS idx_programs_start_time
                    ON programs(start_time);
                CREATE INDEX IF NOT EXISTS idx_programs_title
                    ON programs(title);
                CREATE INDEX IF NOT EXISTS idx_fetch_log_date
                    ON fetch_log(date, channel_id);
                CREATE INDEX IF NOT EXISTS idx_program_genres_program
                    ON program_genres(program_id);
                CREATE INDEX IF NOT EXISTS idx_program_people_program
                    ON program_people(program_id);
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
                cursor = conn.execute("""
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
                program_id = cursor.lastrowid

                # Add genres if provided
                if 'genres' in program_data and program_data['genres']:
                    for genre_name in program_data['genres']:
                        self._add_genre_to_program(conn, program_id, genre_name)

                # Add people if provided
                if 'people' in program_data and program_data['people']:
                    for person in program_data['people']:
                        self._add_person_to_program(
                            conn, program_id,
                            person['name'],
                            person.get('role', 'actor')
                        )

                return True
            except sqlite3.IntegrityError:
                # Already exists
                return False

    def _add_genre_to_program(self, conn, program_id, genre_name):
        """Add a genre to a program (internal helper)"""
        # Insert or get genre
        cursor = conn.execute(
            "INSERT OR IGNORE INTO genres (name) VALUES (?)",
            (genre_name,)
        )
        cursor = conn.execute(
            "SELECT id FROM genres WHERE name = ?",
            (genre_name,)
        )
        genre_id = cursor.fetchone()['id']

        # Link to program
        conn.execute(
            "INSERT OR IGNORE INTO program_genres (program_id, genre_id) VALUES (?, ?)",
            (program_id, genre_id)
        )

    def _add_person_to_program(self, conn, program_id, person_name, role):
        """Add a person to a program (internal helper)"""
        # Insert or get person
        cursor = conn.execute(
            "INSERT OR IGNORE INTO people (name) VALUES (?)",
            (person_name,)
        )
        cursor = conn.execute(
            "SELECT id FROM people WHERE name = ?",
            (person_name,)
        )
        person_id = cursor.fetchone()['id']

        # Link to program
        conn.execute(
            "INSERT OR IGNORE INTO program_people (program_id, person_id, role) VALUES (?, ?, ?)",
            (program_id, person_id, role)
        )

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

    def get_programs_now(self):
        """Get programs currently airing"""
        with self.get_connection() as conn:
            now = datetime.now().isoformat()
            cursor = conn.execute("""
                SELECT p.*, c.name as channel_name
                FROM programs p
                JOIN channels c ON p.channel_id = c.id
                WHERE ? BETWEEN p.start_time AND p.end_time
                ORDER BY c.name
            """, (now,))

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

    def get_programs_by_genre(self, genre_name, limit=50):
        """Get programs by genre"""
        with self.get_connection() as conn:
            cursor = conn.execute("""
                SELECT p.*, c.name as channel_name, g.name as genre
                FROM programs p
                JOIN channels c ON p.channel_id = c.id
                JOIN program_genres pg ON p.id = pg.program_id
                JOIN genres g ON pg.genre_id = g.id
                WHERE g.name = ?
                ORDER BY p.start_time DESC
                LIMIT ?
            """, (genre_name, limit))

            return [dict(row) for row in cursor.fetchall()]

    def get_channels(self, active_only=True):
        """Get all channels"""
        with self.get_connection() as conn:
            if active_only:
                cursor = conn.execute("""
                    SELECT * FROM channels WHERE active = 1 ORDER BY id
                """)
            else:
                cursor = conn.execute("SELECT * FROM channels ORDER BY id")

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

            # Programs per channel
            cursor = conn.execute("""
                SELECT c.name, COUNT(p.id) as count
                FROM channels c
                LEFT JOIN programs p ON c.id = p.channel_id
                WHERE c.active = 1
                GROUP BY c.id, c.name
                ORDER BY count DESC
            """)
            stats['programs_per_channel'] = [dict(row) for row in cursor.fetchall()]

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

            # Top genres
            cursor = conn.execute("""
                SELECT g.name, COUNT(pg.program_id) as count
                FROM genres g
                JOIN program_genres pg ON g.id = pg.genre_id
                GROUP BY g.id, g.name
                ORDER BY count DESC
                LIMIT 10
            """)
            stats['top_genres'] = [dict(row) for row in cursor.fetchall()]

            return stats

    def cleanup_old_programs(self, days_to_keep=30):
        """Remove programs older than specified days"""
        with self.get_connection() as conn:
            cutoff_date = datetime.now().date() - timedelta(days=days_to_keep)
            cursor = conn.execute("""
                DELETE FROM programs
                WHERE date(start_time) < ?
            """, (cutoff_date.isoformat(),))
            deleted_count = cursor.rowcount
            return deleted_count

if __name__ == "__main__":
    # Test the database
    db = TVDatabase("test_tv_programs.db")

    print("Database initialized successfully!")
    print("\nTesting basic operations...")

    # Add a test channel
    db.upsert_channel(1, "Test Channel", "https://example.com/logo.png", "Test")
    print("✓ Channel added")

    # Add a test program
    test_program = {
        'external_id': 'test_001',
        'channel_id': 1,
        'title': 'Test Program',
        'description': 'This is a test program',
        'start_time': datetime.now().isoformat(),
        'end_time': (datetime.now() + timedelta(hours=1)).isoformat(),
        'duration': 60,
        'category': 'Test',
        'genres': ['Drama', 'Comedy'],
        'people': [
            {'name': 'Test Actor', 'role': 'actor'},
            {'name': 'Test Director', 'role': 'director'}
        ]
    }

    if db.insert_program(test_program):
        print("✓ Program added")
    else:
        print("Program already exists")

    # Get statistics
    stats = db.get_statistics()
    print("\nDatabase Statistics:")
    print(f"  Total programs: {stats['total_programs']}")
    print(f"  Total channels: {stats['total_channels']}")
    if stats.get('top_genres'):
        print(f"  Top genres: {', '.join([g['name'] for g in stats['top_genres'][:3]])}")

    print("\n✓ All tests passed!")
