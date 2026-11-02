package github

import (
	"context"
	"errors"
	"strings"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

// reviewsQuery reads the newest submitted reviews and pages unresolved threads. Review bodies stay out
// of it on purpose: the agent reads them itself.
const reviewsQuery = `query($owner:String!,$name:String!,$number:Int!,$after:String){repository(owner:$owner,name:$name){pullRequest(number:$number){reviews(last:100){nodes{id state submittedAt author{__typename login} commit{oid}} pageInfo{hasPreviousPage}} reviewThreads(first:100,after:$after){nodes{isResolved} pageInfo{hasNextPage endCursor}}}}}`

// maxReviewThreadPages bounds thread paging; a longer PR reports a truncated count.
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
		Nodes    []struct{ IsResolved bool }
		PageInfo struct {
			HasNextPage bool
			EndCursor   string
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
		result, err := p.graphRequest(ctx, token, reviewsQuery, map[string]any{
			"owner": repo.owner, "name": repo.name, "number": number, graphAfterVariable: cursor,
		})
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
		for _, thread := range activity.ReviewThreads.Nodes {
			if !thread.IsResolved {
				reviews.UnresolvedThreads++
			}
		}
		info := activity.ReviewThreads.PageInfo
		if !info.HasNextPage || info.EndCursor == "" {
			return reviews, nil
		}
		cursor = &info.EndCursor
	}
	reviews.Truncated = true
	return reviews, nil
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
