#!/usr/bin/env python3
"""Read merged PR latency with one paginated GraphQL list, never per-PR calls."""
import argparse
import json
import math
import statistics
import subprocess
from datetime import datetime, timezone


QUERY = """
query($search: String!, $endCursor: String) {
  search(query: $search, type: ISSUE, first: 50, after: $endCursor) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on PullRequest {
        number createdAt mergedAt
        timelineItems(first: 100, itemTypes: [READY_FOR_REVIEW_EVENT,
          CONVERT_TO_DRAFT_EVENT, ADDED_TO_MERGE_QUEUE_EVENT,
          REMOVED_FROM_MERGE_QUEUE_EVENT]) {
          pageInfo { hasNextPage }
          nodes {
            __typename
            ... on ReadyForReviewEvent { createdAt }
            ... on ConvertToDraftEvent { createdAt }
            ... on AddedToMergeQueueEvent { createdAt }
            ... on RemovedFromMergeQueueEvent { createdAt }
          }
        }
      }
    }
  }
}
"""


def timestamp(value):
    """Require UTC-aware timestamps so cohort boundaries are reproducible."""
    result = datetime.fromisoformat(value.replace('Z', '+00:00'))
    if result.tzinfo is None:
        raise ValueError('timestamps must include a timezone')
    return result.astimezone(timezone.utc)


def distribution(values):
    """Report minutes, with nearest-rank p90 and an explicit sample count."""
    ordered = sorted(values)
    return {
        'n': len(ordered),
        'median_minutes': round(statistics.median(ordered) / 60, 2) if ordered else None,
        'p90_minutes': round(ordered[math.ceil(len(ordered) * .9) - 1] / 60, 2) if ordered else None,
    }


def durations(pr):
    """Use final ready-for-review and sum every observed queue residence."""
    timeline = pr['timelineItems']
    if timeline['pageInfo']['hasNextPage']:
        return None, 'timeline_truncated'
    opened, merged = timestamp(pr['createdAt']), timestamp(pr['mergedAt'])
    ready, queued, queue_seconds, entries = opened, None, 0, 0
    for event in sorted(timeline['nodes'], key=lambda item: item['createdAt']):
        at, kind = timestamp(event['createdAt']), event['__typename']
        if at > merged:
            continue
        if at < opened:
            return None, 'invalid_timeline'
        if kind == 'ConvertToDraftEvent':
            ready = None
        elif kind == 'ReadyForReviewEvent':
            ready = at
        elif kind == 'AddedToMergeQueueEvent':
            if queued is not None:
                return None, 'invalid_queue_events'
            queued = at
            entries += 1
        elif kind == 'RemovedFromMergeQueueEvent':
            if queued is None:
                return None, 'invalid_queue_events'
            queue_seconds += (at - queued).total_seconds()
            queued = None
    if ready is None or merged < ready:
        return None, 'missing_ready_event'
    if queued is not None:
        queue_seconds += (merged - queued).total_seconds()
    return {
        'time_to_merge_seconds': (merged - ready).total_seconds(),
        # No queue events means unavailable/not queued, never a zero sample.
        'queue_seconds': queue_seconds if entries else None,
        'queue_entries': entries,
    }, None


def report(pages, since, until, split=None):
    """Reject search caps and partial pages rather than silently bias a cohort."""
    if not pages or pages[-1]['data']['search']['pageInfo']['hasNextPage']:
        raise ValueError('incomplete PR pagination')
    count = pages[0]['data']['search']['issueCount']
    prs = [pr for page in pages for pr in page['data']['search']['nodes']]
    if count > 1000 or len(prs) != count or len({pr['number'] for pr in prs}) != count:
        raise ValueError('search capped, changed during pagination, or incomplete; use a smaller window')
    boundaries = [(since, split), (split, until)] if split else [(since, until)]
    cohorts = []
    for start, end in boundaries:
        selected = [pr for pr in prs if start <= timestamp(pr['mergedAt']) < end]
        measured, excluded = [], {}
        for pr in selected:
            value, reason = durations(pr)
            if reason:
                excluded[reason] = excluded.get(reason, 0) + 1
            else:
                measured.append(value)
        cohorts.append({
            'since_inclusive': start.isoformat(), 'until_exclusive': end.isoformat(),
            'merged_prs': len(selected), 'excluded': excluded,
            'time_to_merge': distribution([pr['time_to_merge_seconds'] for pr in measured]),
            'queue_time': distribution([pr['queue_seconds'] for pr in measured if pr['queue_seconds'] is not None]),
            'queue_entries': sum(pr['queue_entries'] for pr in measured),
        })
    return {'listed_prs': count, 'cohorts': cohorts}


def self_test():
    """Own the metric invariant here, alongside the repository's CI self-tests."""
    def pr(events, truncated=False):
        return {'number': 1, 'createdAt': '2026-11-04T00:00:00Z',
                'mergedAt': '2026-11-04T01:00:00Z', 'timelineItems': {
                    'pageInfo': {'hasNextPage': truncated}, 'nodes': [
                        {'__typename': kind, 'createdAt': f'2026-11-04T00:{minute:02}:00Z'}
                        for kind, minute in events]}}
    assert durations(pr([]))[0]['time_to_merge_seconds'] == 3600
    assert durations(pr([]))[0]['queue_seconds'] is None
    measured, reason = durations(pr([
        ('ReadyForReviewEvent', 10), ('ConvertToDraftEvent', 15),
        ('ReadyForReviewEvent', 20), ('AddedToMergeQueueEvent', 30),
        ('RemovedFromMergeQueueEvent', 35), ('AddedToMergeQueueEvent', 50)]))
    assert reason is None and measured == {
        'time_to_merge_seconds': 2400, 'queue_seconds': 900, 'queue_entries': 2}
    assert durations(pr([], True))[1] == 'timeline_truncated'
    assert durations(pr([('ConvertToDraftEvent', 10)]))[1] == 'missing_ready_event'
    assert durations(pr([('RemovedFromMergeQueueEvent', 10)]))[1] == 'invalid_queue_events'
    assert distribution(list(range(1, 11))) == {'n': 10, 'median_minutes': .09, 'p90_minutes': .15}
    assert distribution([])['median_minutes'] is None
    pages = [{'data': {'search': {'issueCount': 1, 'pageInfo': {'hasNextPage': False}, 'nodes': [pr([])]}}}]
    since, split, until = map(timestamp, ['2026-11-04T00:00:00Z', '2026-11-04T01:00:00Z', '2026-11-05T00:00:00Z'])
    cohorts = report(pages, since, until, split)['cohorts']
    assert [cohort['merged_prs'] for cohort in cohorts] == [0, 1]
    pages[0]['data']['search']['issueCount'] = 1001
    try:
        report(pages, since, until)
        raise AssertionError('search cap accepted')
    except ValueError:
        pass
    print('ci-merge-latency self-test: readiness, queue retries, quantiles and completeness hold')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', default='compozy/compozy')
    parser.add_argument('--since', help='inclusive ISO timestamp')
    parser.add_argument('--until', default=datetime.now(timezone.utc).isoformat(), help='exclusive ISO timestamp; default now')
    parser.add_argument('--split-at', help='optional boundary for before/after cohorts from the same list')
    parser.add_argument('--input', help='replay a saved gh api graphql --paginate --slurp response')
    parser.add_argument('--self-test', action='store_true')
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return
    if not args.since:
        parser.error('--since is required')
    since, until = timestamp(args.since), timestamp(args.until)
    split = timestamp(args.split_at) if args.split_at else None
    if since >= until or (split and not since < split < until):
        parser.error('require since < split-at < until, or since < until')
    if args.input:
        with open(args.input) as source:
            pages = json.load(source)
    else:
        # Search dates intentionally overfetch boundary days; report filters exact timestamps.
        search = f'repo:{args.repo} is:pr is:merged base:main merged:{since.date()}..{until.date()}'
        response = subprocess.run(['gh', 'api', 'graphql', '--paginate', '--slurp',
                                   '-f', f'query={QUERY}', '-f', f'search={search}'],
                                  check=True, capture_output=True, text=True)
        pages = json.loads(response.stdout)
    print(json.dumps(report(pages, since, until, split), indent=2))


if __name__ == '__main__':
    main()
