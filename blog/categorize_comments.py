#!/usr/bin/env python3
"""
Categorize and group HN comments by theme/topic using keyword analysis and AI-assisted categorization.
"""

import json
import re
from collections import defaultdict
from datetime import datetime
from typing import List, Dict, Set
import sys


class CommentCategorizer:
    def __init__(self, comments_file: str = 'blog/hn_comments.json'):
        with open(comments_file, 'r', encoding='utf-8') as f:
            self.raw_comments = json.load(f)

        # Normalize comments to standard format
        self.comments = self._normalize_comments(self.raw_comments)

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

    def categorize(self) -> Dict[str, List[Dict]]:
        """Categorize comments by analyzing keywords and topics."""
        print(f"Categorizing {len(self.comments)} comments...")

        # Define category keywords
        category_keywords = {
            'Programming & Development': [
                'code', 'programming', 'developer', 'software', 'python', 'javascript',
                'rust', 'go', 'java', 'algorithm', 'framework', 'library', 'api',
                'github', 'git', 'function', 'class', 'bug', 'debug', 'refactor'
            ],
            'AI & Machine Learning': [
                'ai', 'ml', 'machine learning', 'llm', 'gpt', 'chatgpt', 'claude',
                'model', 'training', 'neural', 'transformer', 'openai', 'anthropic',
                'deep learning', 'nlp', 'artificial intelligence'
            ],
            'Web & Frontend': [
                'web', 'frontend', 'backend', 'react', 'vue', 'angular', 'css',
                'html', 'dom', 'browser', 'javascript', 'typescript', 'node',
                'webpack', 'ui', 'ux', 'responsive'
            ],
            'DevOps & Infrastructure': [
                'docker', 'kubernetes', 'aws', 'cloud', 'deployment', 'ci/cd',
                'infrastructure', 'server', 'database', 'postgres', 'mysql',
                'redis', 'monitoring', 'scaling', 'devops', 'terraform'
            ],
            'Business & Startups': [
                'startup', 'business', 'company', 'founder', 'revenue', 'product',
                'market', 'customer', 'pricing', 'saas', 'sales', 'growth',
                'funding', 'investor', 'entrepreneurship'
            ],
            'Privacy & Security': [
                'privacy', 'security', 'encryption', 'vulnerability', 'hack',
                'breach', 'password', 'authentication', 'oauth', 'ssl', 'tls',
                'gdpr', 'tracking', 'anonymous', 'vpn'
            ],
            'Open Source': [
                'open source', 'oss', 'license', 'mit', 'gpl', 'apache',
                'contribution', 'maintainer', 'fork', 'pull request', 'community'
            ],
            'Career & Work': [
                'job', 'career', 'interview', 'hiring', 'remote', 'work',
                'salary', 'compensation', 'manager', 'team', 'culture',
                'productivity', 'burnout', 'wfh'
            ],
            'Tools & Productivity': [
                'tool', 'editor', 'vim', 'emacs', 'vscode', 'ide', 'terminal',
                'cli', 'shell', 'bash', 'workflow', 'automation', 'productivity'
            ],
            'Gaming': [
                'game', 'gaming', 'steam', 'playstation', 'xbox', 'nintendo',
                'fps', 'rpg', 'indie', 'unity', 'unreal', 'graphics'
            ],
        }

        # Categorize comments
        categorized = defaultdict(list)
        uncategorized = []

        for comment in self.comments:
            text_lower = (comment['text'] + ' ' + comment['story_title']).lower()

            # Find matching categories
            matches = []
            for category, keywords in category_keywords.items():
                score = sum(1 for keyword in keywords if keyword in text_lower)
                if score > 0:
                    matches.append((category, score))

            if matches:
                # Assign to best matching category
                matches.sort(key=lambda x: x[1], reverse=True)
                best_category = matches[0][0]
                categorized[best_category].append(comment)
            else:
                uncategorized.append(comment)

        # Add uncategorized
        if uncategorized:
            categorized['Other'] = uncategorized

        # Print summary
        print("\nCategorization Summary:")
        print("="*70)
        for category in sorted(categorized.keys()):
            count = len(categorized[category])
            print(f"  {category:30s}: {count:3d} comments")

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
        categorizer = CommentCategorizer()
        categorized = categorizer.categorize()
        categorizer.save_categorized(categorized)

        print("\n✅ Success! Comments categorized.")
        print("Next step: Run 'python3 blog/generate_blog_posts.py' to create markdown posts")

    except FileNotFoundError:
        print("❌ Error: hn_comments.json not found.")
        print("Please run download_hn_comments_v2.py first to fetch your comments.")
        sys.exit(1)
    except Exception as e:
        print(f"❌ Error: {e}")
        sys.exit(1)


if __name__ == '__main__':
    main()
