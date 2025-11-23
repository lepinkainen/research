# Hacker News Comment Blog Generator

This toolkit downloads your Hacker News comments and transforms them into thoughtful blog posts using AI analysis to identify themes, patterns, and insights.

## Features

- **Downloads comments** from Hacker News (Algolia and Firebase APIs)
- **LLM-powered categorization** - Claude analyzes your comments to identify natural themes
- **AI-generated blog posts** - Creates engaging, reflective posts that synthesize your ideas
- **Preserves context** - Links back to original HN discussions
- **Thoughtful summaries** - Goes beyond simple listing to find patterns and insights

## Requirements

```bash
pip install anthropic requests
```

Set your Anthropic API key:
```bash
export ANTHROPIC_API_KEY='your-api-key-here'
```

## Quick Start

```bash
# 1. Download your HN comments (run locally, no API key needed)
python3 blog/download_hn_comments.py

# 2. Categorize comments using AI analysis (requires ANTHROPIC_API_KEY)
python3 blog/categorize_comments.py

# 3. Generate thoughtful blog posts with AI (requires ANTHROPIC_API_KEY)
python3 blog/generate_blog_posts.py
```

## How It Works

### 1. Download Comments (`download_hn_comments.py`)

- Fetches your HN comments from the last 6 months (configurable)
- Tries Algolia HN API first (most complete data with story titles)
- Falls back to Firebase API if needed
- Saves to `blog/hn_comments.json`
- **No API key required** - run this locally on your machine

### 2. Categorize with AI (`categorize_comments.py`)

- Uses Claude to analyze comment content and identify natural themes
- Creates meaningful category names based on actual content
- Groups related discussions together
- Typically creates 5-10 thematic categories
- Saves to `blog/categorized_comments.json`
- **Requires ANTHROPIC_API_KEY**

### 3. Generate Blog Posts with AI (`generate_blog_posts.py`)

- Uses Claude to write thoughtful blog posts for each category
- Identifies patterns and insights across comments
- Synthesizes ideas and provides analysis
- Quotes relevant comments with attribution
- Creates engaging, readable posts in first person
- Generates index page
- Output: `blog/posts/*.md`
- **Requires ANTHROPIC_API_KEY**

## Configuration

### Download Script

Edit `download_hn_comments.py`:
- **Username**: Change `username = 'theshrike79'` (line 174)
- **Time range**: Modify `days_back = 180` (line 175)

### AI Analysis

Both categorization and blog generation use:
- Model: `claude-sonnet-4-20250514`
- The scripts include fallback to simpler methods if API calls fail

## Output Structure

```
blog/
├── hn_comments.json              # Raw downloaded comments
├── categorized_comments.json     # AI-categorized comments
├── posts/
│   ├── index.md                  # Index page
│   ├── ai_machine_learning.md    # AI-generated post
│   ├── software_development.md   # AI-generated post
│   └── ...
└── README.md
```

## Example Workflow

1. **User runs locally:**
   ```bash
   python3 blog/download_hn_comments.py
   git add blog/hn_comments.json
   git commit -m "Add HN comments data"
   git push
   ```

2. **Claude processes with AI:**
   ```bash
   export ANTHROPIC_API_KEY='sk-...'
   python3 blog/categorize_comments.py
   python3 blog/generate_blog_posts.py
   ```

3. **Result:** Thoughtful blog posts with AI-driven insights

## Why LLM-Based?

Traditional keyword matching misses nuance. The LLM approach:

- **Understands context** - Knows a comment about "Python decorators" belongs in "Software Development"
- **Finds patterns** - Identifies recurring themes across unrelated discussions
- **Writes naturally** - Creates engaging prose instead of just formatting
- **Synthesizes ideas** - Connects dots between comments to provide insights
- **Captures your voice** - Writes in first person based on your actual comments

## Troubleshooting

**Network errors (download)**:
- Run the download script on your local machine
- The script includes retry logic and API fallbacks

**API errors (categorize/generate)**:
- Check that `ANTHROPIC_API_KEY` is set correctly
- Scripts fall back to simpler methods if LLM calls fail
- Categorization uses keyword matching as fallback
- Generation uses simple formatting as fallback

**No comments found**:
- Check username is correct (case-sensitive)
- Verify time range includes your comment activity
- Ensure APIs are accessible from your network

## Cost Estimate

Using Claude Sonnet 4:
- Categorization: ~$0.10-0.50 per 100 comments
- Blog generation: ~$0.50-2.00 per category (depends on comment count)

For a typical 6-month period with 50-100 comments:
- Total cost: ~$2-5

## License

Public domain - use as you wish!
