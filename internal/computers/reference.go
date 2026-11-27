package computers

import (
	"context"
	"net/http"
	"slices"
	"strings"
)

func ResolveReference(ctx context.Context, list func(context.Context) ([]View, error), ref string) (View, error) {
	ref = strings.TrimSpace(ref)
	views, err := list(ctx)
	if err != nil {
		return View{}, err
	}
	if i := slices.IndexFunc(views, func(item View) bool { return string(item.ID) == ref }); i >= 0 {
		return views[i], nil
	}
	var found View
	for _, item := range views {
		if item.Name != ref {
			continue
		}
		if found.ID != "" {
			return View{}, &Failure{
				Status:  http.StatusConflict,
				Code:    "computer_ambiguous",
				Message: "More than one computer has this name. Use its ID.",
			}
		}
		found = item
	}
	if found.ID == "" {
		return View{}, ErrNotFound
	}
	return found, nil
}
