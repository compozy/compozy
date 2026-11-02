package github

import (
	"context"
	"errors"
	"strings"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

// reviewsQuery reads the newest submitted reviews and pages threads from the newest end, so a long pull
// request still reports its latest ones. Review bodies stay out of it on purpose: the agent reads them
// itself. A thread's commit comes from originalCommit, because a
// comment's commit follows the head while its line is unchanged.
const reviewsQuery = `query($owner:String!,$name:String!,$number:Int!,$before:String){repository(owner:$owner,name:$name){pullRequest(number:$number){reviews(last:100){nodes{id state submittedAt author{__typename login} commit{oid}} pageInfo{hasPreviousPage}} reviewThreads(last:100,before:$before){nodes{id isResolved comments(first:1){nodes{originalCommit{oid}}}} pageInfo{hasPreviousPage startCursor}}}}}`

// maxReviewThreadPages bounds thread paging; a longer PR reports a truncated count of its newest threads.
const maxReviewThreadPages = 3

var errGraphReviewsUnavailable = errors.New("github forge: review activity unavailable")

type graphReviewActivity struct {
	Reviews struct {
		Nodes []struct {
			ID          string
			State       string
			SubmittedAt *time.Time
			Author      *struct {
				Typename string `json:"__typename"`
				Login    string
			}
			Commit *struct{ OID string }
		}
		PageInfo struct{ HasPreviousPage bool }
	}
	ReviewThreads struct {
		Nodes []struct {
			ID         string
			IsResolved bool
			Comments   struct {
				Nodes []struct {
					OriginalCommit *struct{ OID string }
				}
			}
		}
		PageInfo struct {
			HasPreviousPage bool
			StartCursor     string
		}
	}
}

func (p *Provider) readGraphReviews(
	ctx context.Context,
	repo repository,
	number int,
	token string,
) (*extensioncontract.ForgeReviews, error) {
	reviews := &extensioncontract.ForgeReviews{Items: []extensioncontract.ForgeReview{}}
	var cursor *string
	for page := range maxReviewThreadPages {
		variables := map[string]any{"owner": repo.owner, "name": repo.name, "number": number, "before": cursor}
		result, err := p.graphRequest(ctx, token, reviewsQuery, variables)
		if err != nil {
			return nil, err
		}
		activity := result.Data.Repository.PullRequest
		if activity == nil {
			return nil, errGraphReviewsUnavailable
		}
		if page == 0 {
			reviews.Items = graphReviewItems(activity)
			reviews.Truncated = activity.Reviews.PageInfo.HasPreviousPage
		}
		prependGraphThreads(reviews, activity)
		info := activity.ReviewThreads.PageInfo
		if !info.HasPreviousPage {
			return boundGraphThreads(reviews), nil
		}
		if info.StartCursor == "" {
			break
		}
		cursor = &info.StartCursor
	}
	reviews.Truncated = true
	return boundGraphThreads(reviews), nil
}

// prependGraphThreads adds one older page, keeping the threads oldest first.
func prependGraphThreads(reviews *extensioncontract.ForgeReviews, activity *graphReviewActivity) {
	page := make([]extensioncontract.ForgeReviewThread, 0, len(activity.ReviewThreads.Nodes))
	for i := range activity.ReviewThreads.Nodes {
		node := &activity.ReviewThreads.Nodes[i]
		if node.IsResolved || node.ID == "" {
			continue
		}
		thread := extensioncontract.ForgeReviewThread{ID: node.ID}
		if comments := node.Comments.Nodes; len(comments) > 0 && comments[0].OriginalCommit != nil {
			thread.Commit = comments[0].OriginalCommit.OID
		}
		page = append(page, thread)
		reviews.UnresolvedThreads++
	}
	reviews.Threads = append(page, reviews.Threads...)
}

// boundGraphThreads keeps the newest threads, which are the ones a watcher has not seen yet.
func boundGraphThreads(reviews *extensioncontract.ForgeReviews) *extensioncontract.ForgeReviews {
	if extra := len(reviews.Threads) - extensioncontract.MaxForgeThreads; extra > 0 {
		reviews.Threads, reviews.Truncated = reviews.Threads[extra:], true
	}
	return reviews
}

func graphReviewItems(activity *graphReviewActivity) []extensioncontract.ForgeReview {
	items := make([]extensioncontract.ForgeReview, 0, len(activity.Reviews.Nodes))
	for i := range activity.Reviews.Nodes {
		node := &activity.Reviews.Nodes[i]
		state := strings.ToLower(node.State)
		// A pending review is the viewer's unsubmitted draft, not review activity.
		if node.ID == "" || !submittedReviewState(state) {
			continue
		}
		item := extensioncontract.ForgeReview{ID: node.ID, State: state, SubmittedAt: node.SubmittedAt}
		if node.Author != nil {
			item.Author, item.IsBot = node.Author.Login, node.Author.Typename == "Bot"
		}
		if node.Commit != nil {
			item.Commit = node.Commit.OID
		}
		items = append(items, item)
	}
	return items
}

func submittedReviewState(state string) bool {
	switch state {
	case extensioncontract.ForgeReviewApproved, extensioncontract.ForgeReviewChangesRequested,
		extensioncontract.ForgeReviewCommented, extensioncontract.ForgeReviewDismissed:
		return true
	default:
		return false
	}
}
