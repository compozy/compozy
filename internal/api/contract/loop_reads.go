package contract

import looppkg "github.com/compozy/compozy/internal/loop"

// LoopRunNodesResponse is the public computed roster page.
type LoopRunNodesResponse = looppkg.RosterPage

// LoopBriefingResponse is the public server-owned run verdict.
type LoopBriefingResponse = looppkg.Briefing

// LoopTimelineResponse is the public durable timeline page.
type LoopTimelineResponse = looppkg.TimelinePage
