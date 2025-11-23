#!/usr/bin/env python3
"""
Generate blog posts from categorized HN comments using LLM analysis.
Creates markdown files with AI-generated summaries and insights.
Requires ANTHROPIC_API_KEY environment variable to be set.
"""

import json
import os
import re
from datetime import datetime
from typing import List, Dict
import sys

try:
    import anthropic
except ImportError:
    print("Error: anthropic package not found.")
    print("Install it with: pip install anthropic")
    sys.exit(1)


class LLMBlogPostGenerator:
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

        # Initialize Anthropic client
        api_key = os.environ.get('ANTHROPIC_API_KEY')
        if not api_key:
            raise ValueError("ANTHROPIC_API_KEY environment variable not set")
        self.client = anthropic.Anthropic(api_key=api_key)

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

    def _prepare_comments_for_llm(self, comments: List[Dict]) -> str:
        """Prepare comments for LLM analysis."""
        lines = []
        for i, comment in enumerate(comments, 1):
            text = self._clean_text(comment['text'])
            lines.append(f"Comment {i}:")
            if comment['story_title']:
                lines.append(f"Discussion: {comment['story_title']}")
            lines.append(f"Date: {comment['created_at'].strftime('%Y-%m-%d')}")
            lines.append(f"HN URL: {comment['hn_url']}")
            lines.append(f"Content: {text}")
            lines.append("")
        return "\n".join(lines)

    def _generate_post_with_llm(self, category: str, comments: List[Dict]) -> str:
        """Generate a blog post using Claude."""
        print(f"  Generating blog post for '{category}' with Claude...")

        comments_text = self._prepare_comments_for_llm(comments)

        prompt = f"""You are writing a blog post that summarizes and reflects on a collection of Hacker News comments.

Category: {category}
Number of comments: {len(comments)}

Here are the comments:

{comments_text}

Please write a thoughtful blog post that:
1. Starts with a compelling introduction about the theme/category
2. Identifies key patterns, insights, and perspectives from across the comments
3. Groups related ideas and discusses the main threads of conversation
4. Reflects on any interesting debates or differing viewpoints
5. Provides your analysis and synthesis of the ideas
6. Ends with concluding thoughts

Format the output as markdown with:
- A title (# heading)
- Clear section headings (## for main sections)
- Quote specific comments when relevant (using > blockquotes) with attribution like [Comment 3]
- Links to the original HN discussions where relevant
- A conversational, reflective tone

The goal is to create a readable blog post that captures the essence of these comments and provides value beyond just listing them.
Make it engaging and insightful, as if you're sharing your thoughts with readers about these interesting discussions.

Write in first person ("I commented...") since this is a personal blog reflecting on the author's own HN comments.
"""

        try:
            message = self.client.messages.create(
                model="claude-sonnet-4-20250514",
                max_tokens=8192,
                messages=[
                    {"role": "user", "content": prompt}
                ]
            )

            blog_post = message.content[0].text

            # Add metadata at the top
            metadata = f"*Generated on {datetime.now().strftime('%B %d, %Y')}*\n"
            metadata += f"*{len(comments)} comments from {category}*\n\n"
            metadata += "---\n\n"

            return metadata + blog_post

        except Exception as e:
            print(f"  Error generating post with LLM: {e}")
            print(f"  Falling back to simple format...")
            return self._generate_post_simple(category, comments)

    def _generate_post_simple(self, category: str, comments: List[Dict]) -> str:
        """Fallback: Generate a simple formatted post without LLM."""
        lines = []
        lines.append(f"# {category}")
        lines.append("")
        lines.append(f"*Generated on {datetime.now().strftime('%B %d, %Y')}*")
        lines.append("")
        lines.append(f"## Overview")
        lines.append("")
        lines.append(f"A collection of {len(comments)} comments on {category.lower()} topics.")
        lines.append("")
        lines.append("## Comments")
        lines.append("")

        for i, comment in enumerate(comments, 1):
            text = self._clean_text(comment['text'])
            lines.append(f"### Comment {i}")
            if comment['story_title']:
                lines.append(f"**Discussion:** {comment['story_title']}")
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
        self.filename_to_category = {}

        for category, comments in self.categorized.items():
            if len(comments) < 2:
                print(f"  Skipping '{category}' (only {len(comments)} comment)")
                continue

            # Create filename
            filename = category.lower().replace(' & ', '_').replace(' ', '_').replace('/', '_') + '.md'
            filepath = os.path.join(self.output_dir, filename)

            # Store mapping
            self.filename_to_category[filename] = category

            # Generate post with LLM
            post_content = self._generate_post_with_llm(category, comments)

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
        lines.append("A collection of blog posts synthesizing my Hacker News comment history,")
        lines.append("organized by theme and written with AI assistance to identify patterns")
        lines.append("and insights across discussions.")
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
        lines.append("*These posts are generated using AI analysis of my HN comment history.*")
        lines.append("*The AI helps identify themes and synthesize ideas, but the underlying comments are all mine.*")

        index_path = os.path.join(self.output_dir, 'index.md')
        with open(index_path, 'w', encoding='utf-8') as f:
            f.write('\n'.join(lines))

        print(f"\n  ✓ index.md")
        return index_path


def main():
    try:
        generator = LLMBlogPostGenerator()
        generated_files = generator.generate_all_posts()

        if generated_files:
            generator.generate_index(generated_files)
            print("\n" + "="*70)
            print(f"✅ Success! Generated {len(generated_files)} blog posts with LLM analysis")
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
