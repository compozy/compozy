package agentinstance

import (
	"context"
	"time"
)

type ActivityQuery struct {
	ProfileID   string
	AgentName   string
	WorkspaceID string
	Since       time.Time
}

type Activity struct {
	ReportsByDay  []ActivityDay  `json:"reports_by_day"`
	CostByProject []ActivityCost `json:"cost_by_project"`
	BusyByHour    []ActivityHour `json:"busy_by_hour"`
}

type ActivityDay struct {
	Day       string `json:"day"`
	Quiet     int    `json:"quiet"`
	Told      int    `json:"told"`
	NeededYou int    `json:"needed_you"`
}

type ActivityCost struct {
	WorkspaceID string  `json:"workspace_id"`
	Scope       string  `json:"scope"`
	USDEstimate float64 `json:"usd_estimate"`
	Tokens      int64   `json:"tokens"`
}

type ActivityHour struct {
	Weekday int `json:"weekday"`
	Hour    int `json:"hour"`
	Events  int `json:"events"`
}

type ActivityReader interface {
	ReadActivity(context.Context, ActivityQuery) (Activity, error)
}
