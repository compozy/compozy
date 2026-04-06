package kinds

import "time"

// TaskFileUpdatedPayload describes a rewritten task file.
type TaskFileUpdatedPayload struct {
	TasksDir  string `json:"tasks_dir"`
	TaskName  string `json:"task_name"`
	FilePath  string `json:"file_path"`
	OldStatus string `json:"old_status,omitempty"`
	NewStatus string `json:"new_status,omitempty"`
}

// TaskMetadataRefreshedPayload describes refreshed task workflow metadata.
type TaskMetadataRefreshedPayload struct {
	TasksDir  string    `json:"tasks_dir"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
	Total     int       `json:"total,omitempty"`
	Completed int       `json:"completed,omitempty"`
	Pending   int       `json:"pending,omitempty"`
}
