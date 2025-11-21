#!/usr/bin/env python3
"""
Generate blog posts from categorized HN comments.
Creates markdown files with summaries and organized comments for each category.
"""

import json
import os
import re
from datetime import datetime
from typing import List, Dict
from collections import Counter
import sys


class BlogPostGenerator:
    def __init__(self, categorized_file: str = 'blog/categorized_comments.json'):
        with open(categorized_file, 'r', encoding='utf-8') as f:
            data = json.load(f)

        # Convert ISO format strings back to datetime objects
        self.categorized = {}
        for category, comments in data.items():
            self.categorized[category] = [
                {**c, 'created_at': datetime.fromisoformat(c['created_at'])}
                for c in comments
            ]

        self.output_dir = 'blog/posts'
        os.makedirs(self.output_dir, exist_ok=True)

    def _clean_text(self, text: str) -> str:
        """Clean and format comment text."""
        if not text:
            return ""

        # Remove HTML tags
        text = re.sub(r'<p>', '\n\n', text)
        text = re.sub(r'<[^>]+>', '', text)

        # Unescape HTML entities
        text = text.replace('&quot;', '"')
        text = text.replace('&#x27;', "'")
        text = text.replace('&amp;', '&')
        text = text.replace('&lt;', '<')
        text = text.replace('&gt;', '>')

        return text.strip()

    def _extract_keywords(self, comments: List[Dict], top_n: int = 10) -> List[str]:
        """Extract common keywords from comments."""
        # Common words to ignore
        stopwords = set([
            'the', 'is', 'at', 'which', 'on', 'a', 'an', 'as', 'are', 'was', 'were',
            'be', 'been', 'being', 'have', 'has', 'had', 'do', 'does', 'did', 'will',
            'would', 'could', 'should', 'may', 'might', 'must', 'can', 'this', 'that',
            'these', 'those', 'i', 'you', 'he', 'she', 'it', 'we', 'they', 'what',
            'who', 'when', 'where', 'why', 'how', 'all', 'each', 'every', 'both',
            'few', 'more', 'most', 'other', 'some', 'such', 'no', 'nor', 'not',
            'only', 'own', 'same', 'so', 'than', 'too', 'very', 'just', 'but',
            'in', 'out', 'up', 'down', 'to', 'from', 'with', 'about', 'for', 'of',
            'by', 'if', 'or', 'because', 'as', 'until', 'while', 'and', 'there',
            'here', 'then', 'now', 'also', 'its', "it's", 'their', 'my', 'your'
        ])

        # Collect all words
        all_words = []
        for comment in comments:
            text = self._clean_text(comment['text']).lower()
            words = re.findall(r'\b[a-z]{3,}\b', text)
            all_words.extend([w for w in words if w not in stopwords])

        # Count and return top keywords
        counter = Counter(all_words)
        return [word for word, count in counter.most_common(top_n)]

    def _generate_summary(self, category: str, comments: List[Dict]) -> str:
        """Generate a summary for the category."""
        # Get keywords
        keywords = self._extract_keywords(comments, top_n=8)

        # Get date range
        dates = [c['created_at'] for c in comments]
        date_range = f"{min(dates).strftime('%B %Y')} - {max(dates).strftime('%B %Y')}"

        # Get unique stories
        stories = set(c['story_title'] for c in comments if c['story_title'])

        # Build summary
        summary = f"A collection of {len(comments)} comments on {category.lower()} topics "
        summary += f"from {date_range}. "

        if stories:
            summary += f"These discussions span {len(stories)} different threads, "

        if keywords:
            summary += f"covering themes like {', '.join(keywords[:5])}, "
            if len(keywords) > 5:
                summary += f"and {', '.join(keywords[5:8])}."
            else:
                summary = summary.rstrip(', ') + '.'

        return summary

    def _generate_post(self, category: str, comments: List[Dict]) -> str:
        """Generate a markdown blog post for a category."""
        # Sort comments by date (newest first)
        comments.sort(key=lambda x: x['created_at'], reverse=True)

        # Generate summary
        summary = self._generate_summary(category, comments)

        # Start building the post
        lines = []
        lines.append(f"# {category}")
        lines.append("")
        lines.append(f"*Generated on {datetime.now().strftime('%B %d, %Y')}*")
        lines.append("")
        lines.append("## Overview")
        lines.append("")
        lines.append(summary)
        lines.append("")
        lines.append(f"**Total Comments:** {len(comments)}")
        lines.append("")

        # Group by story
        by_story = {}
        for comment in comments:
            story = comment['story_title'] or 'Unknown Thread'
            if story not in by_story:
                by_story[story] = []
            by_story[story].append(comment)

        # Add top stories section if we have story titles
        if any(c['story_title'] for c in comments):
            lines.append("## Top Discussions")
            lines.append("")
            story_counts = [(story, len(comms)) for story, comms in by_story.items()]
            story_counts.sort(key=lambda x: x[1], reverse=True)

            for story, count in story_counts[:5]:
                lines.append(f"- **{story}** ({count} comment{'s' if count > 1 else ''})")

            lines.append("")

        # Add all comments section
        lines.append("## Comments")
        lines.append("")

        for i, comment in enumerate(comments, 1):
            text = self._clean_text(comment['text'])

            # Limit very long comments
            if len(text) > 1000:
                text = text[:1000] + "..."

            lines.append(f"### Comment {i}")
            if comment['story_title']:
                lines.append(f"**Story:** {comment['story_title']}")
            lines.append(f"**Date:** {comment['created_at'].strftime('%B %d, %Y')}")
            lines.append(f"**Link:** {comment['hn_url']}")
            lines.append("")
            lines.append("> " + text.replace('\n', '\n> '))
            lines.append("")
            lines.append("---")
            lines.append("")

        return '\n'.join(lines)

    def generate_all_posts(self):
        """Generate blog posts for all categories."""
        print(f"Generating blog posts in {self.output_dir}/...")
        print("="*70)

        generated_files = []
        self.filename_to_category = {}  # Track mapping for index generation

        for category, comments in self.categorized.items():
            if len(comments) < 2:
                print(f"  Skipping '{category}' (only {len(comments)} comment)")
                continue

            # Create filename
            filename = category.lower().replace(' & ', '_').replace(' ', '_') + '.md'
            filepath = os.path.join(self.output_dir, filename)

            # Store mapping
            self.filename_to_category[filename] = category

            # Generate post
            post_content = self._generate_post(category, comments)

            # Save to file
            with open(filepath, 'w', encoding='utf-8') as f:
                f.write(post_content)

            generated_files.append(filepath)
            print(f"  ✓ {filename:40s} ({len(comments)} comments)")

        return generated_files

    def generate_index(self, generated_files: List[str]):
        """Generate an index page linking to all posts."""
        lines = []
        lines.append("# My Hacker News Comments - Blog Posts")
        lines.append("")
        lines.append(f"*Generated on {datetime.now().strftime('%B %d, %Y')}*")
        lines.append("")
        lines.append("## Categories")
        lines.append("")

        for filepath in sorted(generated_files):
            filename = os.path.basename(filepath)
            category = self.filename_to_category.get(filename, filename.replace('_', ' ').replace('.md', '').title())
            num_comments = len(self.categorized.get(category, []))

            lines.append(f"- [{category}]({filename}) ({num_comments} comments)")

        lines.append("")
        lines.append("---")
        lines.append("")
        lines.append("These posts are automatically generated from my Hacker News comment history.")

        index_path = os.path.join(self.output_dir, 'index.md')
        with open(index_path, 'w', encoding='utf-8') as f:
            f.write('\n'.join(lines))

        print(f"\n  ✓ index.md")
        return index_path


def main():
    try:
        generator = BlogPostGenerator()
        generated_files = generator.generate_all_posts()

        if generated_files:
            generator.generate_index(generated_files)
            print("\n" + "="*70)
            print(f"✅ Success! Generated {len(generated_files)} blog posts")
            print(f"   Output directory: blog/posts/")
            print(f"   Index file: blog/posts/index.md")
        else:
            print("\n⚠️  No posts generated. Not enough comments in any category.")

    except FileNotFoundError:
        print("❌ Error: categorized_comments.json not found.")
        print("Please run categorize_comments.py first.")
        sys.exit(1)
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()
