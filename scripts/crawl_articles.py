#!/usr/bin/env python3
"""
Crawl4AI Article Downloader

This script uses crawl4ai to download content from a list of URLs and save them
as markdown files in the ai-docs/articles directory.

Usage:
    python scripts/crawl_articles.py <url1> <url2> <url3> ...
    python scripts/crawl_articles.py --file urls.txt

Examples:
    python scripts/crawl_articles.py https://example.com/article1 https://example.com/article2
    python scripts/crawl_articles.py --file urls.txt
"""

import asyncio
import sys
import argparse
from pathlib import Path
from typing import List
import re
from urllib.parse import urlparse
import hashlib

try:
    from crawl4ai import AsyncWebCrawler
except ImportError:
    print("Error: crawl4ai is not installed.")
    print("Install it with: pip install crawl4ai")
    print("Then run: crawl4ai-setup")
    sys.exit(1)


def sanitize_filename(url: str) -> str:
    """Create a safe filename from a URL."""
    parsed = urlparse(url)
    path = parsed.path.strip("/")

    if not path:
        path = parsed.netloc

    safe_name = re.sub(r'[^\w\s-]', '_', path)
    safe_name = re.sub(r'[-\s]+', '-', safe_name)
    safe_name = safe_name[:100]

    url_hash = hashlib.md5(url.encode()).hexdigest()[:8]

    return f"{safe_name}-{url_hash}.md"


async def download_article(crawler: AsyncWebCrawler, url: str, output_dir: Path) -> bool:
    """Download a single article and save it to the output directory."""
    try:
        print(f"Crawling: {url}")
        result = await crawler.arun(url=url)

        if not result.success:
            print(f"  ❌ Failed to crawl {url}: {result.error_message}")
            return False

        filename = sanitize_filename(url)
        filepath = output_dir / filename

        content = f"# Source: {url}\n\n"
        content += f"# Title: {result.metadata.get('title', 'Untitled')}\n\n"
        content += "---\n\n"
        content += result.markdown

        filepath.write_text(content, encoding="utf-8")
        print(f"  ✅ Saved to: {filepath.name}")
        return True

    except Exception as e:
        print(f"  ❌ Error processing {url}: {str(e)}")
        return False


async def download_articles(urls: List[str], output_dir: Path):
    """Download multiple articles concurrently."""
    output_dir.mkdir(parents=True, exist_ok=True)

    print(f"\nDownloading {len(urls)} articles to {output_dir}")
    print("=" * 60)

    async with AsyncWebCrawler() as crawler:
        tasks = [download_article(crawler, url, output_dir) for url in urls]
        results = await asyncio.gather(*tasks)

    successful = sum(results)
    failed = len(results) - successful

    print("\n" + "=" * 60)
    print(f"✅ Successfully downloaded: {successful}")
    print(f"❌ Failed: {failed}")


def read_urls_from_file(filepath: str) -> List[str]:
    """Read URLs from a text file (one URL per line)."""
    try:
        with open(filepath, 'r') as f:
            urls = [line.strip() for line in f if line.strip() and not line.startswith('#')]
        return urls
    except FileNotFoundError:
        print(f"Error: File not found: {filepath}")
        sys.exit(1)
    except Exception as e:
        print(f"Error reading file: {e}")
        sys.exit(1)


def main():
    parser = argparse.ArgumentParser(
        description="Download articles using crawl4ai",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s https://example.com/article1 https://example.com/article2
  %(prog)s --file urls.txt
  %(prog)s --file urls.txt --output custom-folder
        """
    )

    parser.add_argument(
        'urls',
        nargs='*',
        help='URLs to download (can provide multiple URLs separated by spaces)'
    )

    parser.add_argument(
        '--file', '-f',
        dest='file',
        help='Read URLs from a text file (one URL per line)'
    )

    parser.add_argument(
        '--output', '-o',
        dest='output',
        default='ai-docs/articles',
        help='Output directory (default: ai-docs/articles)'
    )

    args = parser.parse_args()

    urls = []
    if args.file:
        urls = read_urls_from_file(args.file)
    elif args.urls:
        urls = args.urls
    else:
        parser.print_help()
        sys.exit(1)

    if not urls:
        print("Error: No URLs provided")
        sys.exit(1)

    for url in urls:
        if not url.startswith(('http://', 'https://')):
            print(f"Error: Invalid URL: {url}")
            sys.exit(1)

    output_dir = Path(args.output)

    try:
        asyncio.run(download_articles(urls, output_dir))
    except KeyboardInterrupt:
        print("\n\nDownload interrupted by user")
        sys.exit(1)


if __name__ == "__main__":
    main()
