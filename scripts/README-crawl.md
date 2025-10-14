# Article Crawler Script

This script uses [crawl4ai](https://github.com/unclecode/crawl4ai) to download web content and save it as markdown files.

## Installation

1. Install crawl4ai and its dependencies:

```bash
pip install -r scripts/requirements-crawl.txt
crawl4ai-setup
```

2. Verify the installation:

```bash
crawl4ai-doctor
```

## Usage

### Download from command-line URLs

```bash
python scripts/crawl_articles.py https://example.com/article1 https://example.com/article2
```

### Download from a file

Create a text file with URLs (one per line):

```bash
# urls.txt
https://example.com/article1
https://example.com/article2
https://example.com/article3
```

Then run:

```bash
python scripts/crawl_articles.py --file urls.txt
```

### Custom output directory

```bash
python scripts/crawl_articles.py --file urls.txt --output custom-folder
```

## Output

Articles are saved as markdown files in `ai-docs/articles/` (or custom directory) with:

- Sanitized filenames based on the URL
- Source URL in the header
- Page title
- Full content in markdown format

## Features

- ✅ Async concurrent downloading for speed
- ✅ Automatic filename sanitization
- ✅ Markdown format with metadata
- ✅ Progress reporting
- ✅ Error handling and reporting
- ✅ Duplicate URL handling via hash

## Example Output

Each article file will have this structure:

```markdown
# Source: https://example.com/article

# Title: Article Title

---

[Article content in markdown format]
```
