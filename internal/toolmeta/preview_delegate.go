package toolmeta

import (
	"fmt"
	"strings"
)

func delegatePreview(input map[string]any) string {
	rawTasks, ok := input["tasks"].([]any)
	if !ok || len(rawTasks) == 0 {
		return ""
	}
	labels := make([]string, 0, len(rawTasks))
	for _, rawTask := range rawTasks {
		task, ok := rawTask.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"goal", "prompt", "description", "name"} {
			if label := oneLine(scalarString(task[key])); label != "" {
				labels = append(labels, label)
				break
			}
		}
	}
	if len(labels) == 0 {
		return fmt.Sprintf("%d tasks", len(rawTasks))
	}
	return fmt.Sprintf("%d tasks: %s", len(rawTasks), strings.Join(labels, "; "))
}
