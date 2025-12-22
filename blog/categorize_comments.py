#!/usr/bin/env python3
"""
Categorize and group HN comments by theme/topic using LLM analysis.
Requires ANTHROPIC_API_KEY environment variable to be set.
"""

import json
import os
from datetime import datetime
from typing import List, Dict
import sys

try:
    import anthropic
except ImportError:
    print("Error: anthropic package not found.")
    print("Install it with: pip install anthropic")
    sys.exit(1)


class LLMCommentCategorizer:
    def __init__(self, comments_file: str = 'blog/hn_comments.json'):
        with open(comments_file, 'r', encoding='utf-8') as f:
            self.raw_comments = json.load(f)

        # Normalize comments to standard format
        self.comments = self._normalize_comments(self.raw_comments)

        # Initialize Anthropic client
        api_key = os.environ.get('ANTHROPIC_API_KEY')
        if not api_key:
            raise ValueError("ANTHROPIC_API_KEY environment variable not set")
        self.client = anthropic.Anthropic(api_key=api_key)

    def _normalize_comments(self, comments: List[Dict]) -> List[Dict]:
        """Normalize different API formats to standard format."""
        normalized = []

        for comment in comments:
            # Detect format
            if 'comment_text' in comment:
                # Algolia format
                normalized.append({
                    'id': comment.get('objectID', comment.get('id', '')),
                    'text': comment.get('comment_text', ''),
                    'story_title': comment.get('story_title', ''),
                    'story_url': comment.get('story_url', ''),
                    'created_at': datetime.fromtimestamp(comment.get('created_at_i', 0)),
                    'points': comment.get('points', 0),
                    'parent_id': comment.get('parent_id'),
                    'hn_url': f"https://news.ycombinator.com/item?id={comment.get('objectID', '')}",
                })
            else:
                # Firebase format
                normalized.append({
                    'id': comment.get('id', ''),
                    'text': comment.get('text', ''),
                    'story_title': '',  # Not available in Firebase format
                    'story_url': '',
                    'created_at': datetime.fromtimestamp(comment.get('time', 0)),
                    'points': 0,
                    'parent_id': comment.get('parent'),
                    'hn_url': f"https://news.ycombinator.com/item?id={comment.get('id', '')}",
                })

        return normalized

    def _prepare_comments_for_llm(self) -> str:
        """Prepare comments in a format suitable for LLM analysis."""
        lines = []
        for i, comment in enumerate(self.comments, 1):
            lines.append(f"Comment {i} (ID: {comment['id']}):")
            if comment['story_title']:
                lines.append(f"Story: {comment['story_title']}")
            lines.append(f"Date: {comment['created_at'].strftime('%Y-%m-%d')}")
            lines.append(f"Text: {comment['text'][:500]}")  # Limit length
            lines.append("")
        return "\n".join(lines)

    def categorize(self) -> Dict[str, List[Dict]]:
        """Categorize comments using LLM analysis."""
        print(f"Analyzing {len(self.comments)} comments with Claude...")
        print("This may take a moment...")

        # Prepare prompt
        comments_text = self._prepare_comments_for_llm()

        prompt = f"""Analyze these Hacker News comments and group them into thematic categories.

For each comment, identify its main topic/theme and assign it to an appropriate category.
Create category names that are descriptive and meaningful.

Comments:
{comments_text}

Please respond with a JSON object where:
- Keys are category names (e.g., "AI & Machine Learning", "Software Development", etc.)
- Values are arrays of comment IDs that belong to that category

Example format:
{{
  "AI & Machine Learning": ["40123456", "40123789"],
  "Software Architecture": ["40123567"],
  ...
}}

Focus on creating 5-10 meaningful categories that capture the main themes.
Each comment should be assigned to exactly one category that best represents its content.
"""

        try:
            # Call Claude API
            message = self.client.messages.create(
                model="claude-sonnet-4-20250514",
                max_tokens=4096,
                messages=[
                    {"role": "user", "content": prompt}
                ]
            )

            # Parse response
            response_text = message.content[0].text

            # Extract JSON from response (handle markdown code blocks)
            if "```json" in response_text:
                response_text = response_text.split("```json")[1].split("```")[0].strip()
            elif "```" in response_text:
                response_text = response_text.split("```")[1].split("```")[0].strip()

            category_mapping = json.loads(response_text)

            # Build categorized dict
            categorized = {}
            comment_id_map = {c['id']: c for c in self.comments}

            for category, comment_ids in category_mapping.items():
                categorized[category] = []
                for comment_id in comment_ids:
                    if comment_id in comment_id_map:
                        categorized[category].append(comment_id_map[comment_id])

            # Print summary
            print("\nCategorization Summary:")
            print("="*70)
            for category in sorted(categorized.keys()):
                count = len(categorized[category])
                print(f"  {category:40s}: {count:3d} comments")

            return categorized

        except Exception as e:
            print(f"Error calling Claude API: {e}")
            print("\nFalling back to keyword-based categorization...")
            return self._fallback_categorize()

    def _fallback_categorize(self) -> Dict[str, List[Dict]]:
        """Fallback to simple keyword-based categorization."""
        from collections import defaultdict

        category_keywords = {
            'Programming & Development': ['code', 'programming', 'developer', 'software', 'python', 'rust', 'go'],
            'AI & Machine Learning': ['ai', 'ml', 'llm', 'gpt', 'model', 'training'],
            'Web Development': ['web', 'frontend', 'backend', 'react', 'javascript'],
            'Other': []  # Default category
        }

        categorized = defaultdict(list)

        for comment in self.comments:
            text_lower = (comment['text'] + ' ' + comment['story_title']).lower()
            matched = False

            for category, keywords in category_keywords.items():
                if category == 'Other':
                    continue
                if any(kw in text_lower for kw in keywords):
                    categorized[category].append(comment)
                    matched = True
                    break

            if not matched:
                categorized['Other'].append(comment)

        return dict(categorized)

    def save_categorized(self, categorized: Dict[str, List[Dict]], output_file: str = 'blog/categorized_comments.json'):
        """Save categorized comments to file."""
        # Convert datetime objects to strings for JSON serialization
        serializable = {}
        for category, comments in categorized.items():
            serializable[category] = [
                {**comment, 'created_at': comment['created_at'].isoformat()}
                for comment in comments
            ]

        with open(output_file, 'w', encoding='utf-8') as f:
            json.dump(serializable, f, indent=2, ensure_ascii=False)

        print(f"\nCategorized comments saved to {output_file}")


def main():
    try:
        categorizer = LLMCommentCategorizer()
        categorized = categorizer.categorize()
        categorizer.save_categorized(categorized)

        print("\n✅ Success! Comments categorized using LLM analysis.")
        print("Next step: Run 'python3 blog/generate_blog_posts.py' to create markdown posts")

    except FileNotFoundError:
        print("❌ Error: hn_comments.json not found.")
        print("Please run download_hn_comments.py first to fetch your comments.")
        sys.exit(1)
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()
