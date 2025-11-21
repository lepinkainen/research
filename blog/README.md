# Hacker News Comment Blog Generator

This toolkit downloads your Hacker News comments and transforms them into organized blog posts grouped by theme.

## Quick Start

```bash
# 1. Download your HN comments
python3 blog/download_hn_comments.py

# 2. Categorize comments by theme
python3 blog/categorize_comments.py

# 3. Generate markdown blog posts
python3 blog/generate_blog_posts.py
```

## How It Works

### 1. Download Comments (`download_hn_comments.py`)

- Fetches your HN comments from the last 6 months (configurable)
- Tries multiple APIs (Algolia, Firebase) for reliability
- Saves to `blog/hn_comments.json`

### 2. Categorize (`categorize_comments.py`)

- Groups comments by topic using keyword analysis
- Categories include:
  - Programming & Development
  - AI & Machine Learning
  - Web & Frontend
  - DevOps & Infrastructure
  - Business & Startups
  - Privacy & Security
  - Open Source
  - Career & Work
  - Tools & Productivity
  - Gaming
  - Other
- Saves to `blog/categorized_comments.json`

### 3. Generate Blog Posts (`generate_blog_posts.py`)

- Creates markdown files for each category
- Includes:
  - Overview and summary
  - Statistics
  - Top discussions
  - All comments with links back to HN
- Generates index page
- Output: `blog/posts/*.md`

## Configuration

Edit the scripts to customize:

- **Username**: Change `username = 'theshrike79'` in `download_hn_comments.py` (line 153)
- **Time range**: Modify `days_back = 180` for different periods (line 154)
- **Categories**: Add keywords to `category_keywords` in `categorize_comments.py`
- **Post format**: Customize templates in `generate_blog_posts.py`

## Output Structure

```
blog/
├── hn_comments.json              # Raw downloaded comments
├── categorized_comments.json     # Comments grouped by theme
├── posts/
│   ├── index.md                  # Index page
│   ├── programming_development.md
│   ├── ai_machine_learning.md
│   ├── web_frontend.md
│   └── ...
└── README.md
```

## Troubleshooting

**Network errors**: The scripts include retry logic and multiple API fallbacks:
1. Tries Algolia HN API first (most complete data)
2. Falls back to Firebase API if Algolia fails

**No comments found**: Check that:
- Username is correct (case-sensitive)
- Time range includes your comment activity
- APIs are accessible from your network

## Next Steps

1. Run the download script to fetch your comments
2. Review the raw data in `hn_comments.json`
3. Run categorization to group by theme
4. Generate blog posts
5. Review and customize the generated markdown
6. Publish to your blog or website

## License

Public domain - use as you wish!
