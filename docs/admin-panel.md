# Compozy Admin Panel - Website Specification

## Overview

The Compozy Admin Panel is a web application that provides a user interface for managing workflows, monitoring executions, and configuring the Compozy orchestration engine.

## Site Map

### Main Navigation

- Dashboard
- Workflows
- Executions
- Resources (Global)
- Schedules
- Monitoring
- Settings

## Pages & Routes

### 1. Dashboard (`/`)

**Purpose**: Overview of system status and recent activity

**Content**:

- System health status card
- Active executions counter
- Total workflows counter
- Success/failure rate metrics
- Recent executions list (last 10)
- Quick action buttons (Execute Workflow, View All Executions)
- Activity timeline

```md
Create a modern dashboard page for a workflow orchestration admin panel with:

Layout:

- Clean, minimal design with card-based layout
- Responsive grid: 4 columns on desktop, 2 on tablet, 1 on mobile
- White background with subtle gray borders (#e5e7eb)
- 16px spacing between cards

Top Stats Row (4 cards):

- System Health: Green/Yellow/Red status indicator with "Healthy/Warning/Critical" text
- Active Executions: Large number with pulsing animation if > 0
- Total Workflows: Number with small workflow icon
- Success Rate: Percentage with mini donut chart

Recent Executions Section:

- Card title "Recent Executions" with "View All" link
- Table with columns: Workflow, Status (with colored dot), Started, Duration
- Status colors: green (success), red (failed), blue (running), gray (pending)
- Hover effect on rows
- Max 10 rows with subtle fade on last row

Quick Actions:

- Two primary buttons: "Execute Workflow" (blue) and "View All Executions" (ghost style)
- Buttons centered with 12px gap

Activity Timeline:

- Vertical timeline on the right side
- Small dots for events with timestamp and description
- Auto-refresh every 30 seconds indicator
```

### 2. Workflows (`/workflows`)

**Purpose**: Browse and manage workflow definitions

**List View** (`/workflows`):

- Card grid or table view toggle
- Search and filter by tags
- Create new workflow button
- Workflow cards show: name, description, last modified, execution count

```md
Create a workflows list page with toggle between grid and table views:

Header:

- Page title "Workflows" with workflow icon
- "Create Workflow" primary button (blue) aligned right

Filter Bar (horizontal, below header):

- Search bar (400px width)
- View toggle buttons (Grid/Table)
- Filter dropdowns in a row:
    - Tags (multi-select dropdown)
    - Status (Active/Inactive dropdown)
    - Last Modified (date range picker)
- Clear filters link at the end

Grid View (default):

- Responsive cards: 3 columns desktop, 2 tablet, 1 mobile
- Card content:
    - Workflow name (font-weight: 600)
    - Description (2 lines max, gray text)
    - Bottom row: Last modified date | Execution count
    - Hover: subtle shadow and border color change
    - Click anywhere to navigate to detail

Table View:

- Columns: Name, Description, Tags, Last Modified, Executions, Actions
- Sortable columns with arrow indicators
- Row hover highlight
- Actions: View, Edit, Delete (icon buttons)

Empty State:

- Illustration of empty workflow
- "No workflows yet" heading
- "Create your first workflow" button
```

**Detail View** (`/workflows/:id`):

- YAML/JSON editor with syntax highlighting
- Version history
- Execution history for this workflow
- Execute button with parameter inputs
- Edit/Save/Cancel actions

```md
Create a workflow detail page with code editor:

Header:

- Breadcrumb: Workflows > [Workflow Name]
- Workflow name as editable input (click to edit)
- Status badge (Active/Inactive)
- Action buttons: Execute, Save (disabled initially), Delete

Main Layout (2 columns on desktop, stacked on mobile):

Left Column (60%):

- Code editor with syntax highlighting (Monaco editor style)
- YAML/JSON toggle
- Line numbers
- Find/Replace functionality
- Dark theme option toggle

Right Column (40%):

- Info Card:
    - Created date
    - Last modified
    - Total executions
    - Success rate with mini chart
- Execute Form:
    - "Execute Workflow" section header
    - Dynamic form inputs based on workflow parameters
    - Input validation indicators
    - "Execute Now" primary button
- Recent Executions:
    - Last 5 executions mini-list
    - Status, duration, timestamp
    - "View All Executions" link

Version History (accordion):

- Collapsible section
- List of versions with timestamp and author
- "Restore" button for each version
- Diff view on click
```

### 3. Executions (`/executions`)

**Purpose**: Monitor and manage workflow executions

**List View** (`/executions`):

- Real-time updates via WebSocket
- Filter by status, workflow, date range
- Bulk actions (cancel, retry)
- Status indicators with colors

```md
Create an executions monitoring page with real-time updates:

Header:

- Page title "Executions" with activity icon
- Auto-refresh indicator (pulsing dot when active)
- Bulk action buttons: Cancel Selected, Retry Failed

Filters Bar (horizontal):

- Status pills: All, Running, Success, Failed, Pending (with counts)
- Workflow dropdown selector
- Date range picker
- Clear filters button
- All filters in a single horizontal row

Executions Table:

- Columns: □ (checkbox), Status, Workflow, Started, Duration, Progress
- Status column:
    - Colored dot + text
    - Running: blue with spinning animation
    - Success: green check
    - Failed: red X
    - Pending: gray clock
- Progress column: progress bar for running executions
- Real-time updates without page jump
- Sticky header on scroll

Row Actions (on hover):

- View Details
- Cancel (if running)
- Retry (if failed)
- Copy execution ID

Pagination:

- Items per page selector (10, 25, 50)
- Page numbers with current page highlighted
- Keyboard shortcuts hint (← →)

WebSocket Status:

- Small indicator bottom right
- Green: connected, Red: disconnected
- Click to reconnect
```

**Detail View** (`/executions/:id`):

- Execution timeline/graph
- Task-level status and logs
- Input/Output data viewer
- Performance metrics
- Cancel/Retry actions

```md
Create an execution detail page with timeline visualization:

Header:

- Breadcrumb: Executions > [Execution ID]
- Status badge with large icon
- Copy ID button
- Action buttons: Cancel (if running), Retry (if failed)

Execution Timeline:

- Horizontal timeline with tasks as nodes
- Node colors match status (green/blue/red/gray)
- Animated progress line for running tasks
- Click node to see task details
- Zoom controls (+/-/fit)

Info Cards Row:

- Start Time card with calendar icon
- Duration card with timer (running timer if active)
- Task Progress card (X of Y completed)
- Resource Usage card (CPU/Memory if available)

Task Details Section:

- Collapsible task list
- For each task:
    - Name with status icon
    - Duration
    - Logs viewer (expandable)
    - Input/Output JSON viewers (side by side)

Logs Panel:

- Full-width terminal-style log viewer
- Black background with color-coded output
- Search/filter within logs
- Download logs button
- Auto-scroll toggle

Error Details (if failed):

- Red error card with error message
- Stack trace in collapsible section
- "Retry with same input" button
```

### 4. Resources (`/resources`)

**Purpose**: Manage global resources (agents, tools, tasks, MCP servers) loaded via autoload

**Main View** (`/resources`):

- Tab navigation for resource types
- Global resources only (autoload feature)
- Search across all resource types

```md
Create a resources management page with tabs:

Header:

- Page title "Global Resources" with cube icon
- Subtitle: "Resources loaded via autoload configuration"
- Refresh resources button

Tab Navigation:

- Horizontal tabs: Agents | Tools | Tasks | MCP Servers
- Active tab with blue underline
- Tab badges showing count (e.g., "Agents (12)")
- Smooth slide animation between tabs

Common Filter Bar (below tabs, above content):

- Search bar that searches within active tab
- Status filter dropdown (Active/Inactive)
- Tags/Categories dropdown (content changes per tab)
- Clear filters link
- All filters in horizontal layout

Empty State (per tab):

- Icon representing resource type
- "No [resource type] configured"
- "Learn about autoload configuration" link
```

**Agents Tab** (`/resources?tab=agents`):

```md
Agents tab content:

Filter Bar (specific to agents):

- Search agents
- Model filter dropdown (GPT-4, Claude, etc.)
- Capabilities multi-select
- Status (Active/Inactive)

Agent Cards Grid:

- 2 columns on desktop, 1 on mobile
- Card content:
    - Agent name with robot icon
    - Model badge (GPT-4, Claude, etc.)
    - Description (2 lines)
    - Capabilities tags
    - Last used timestamp
    - "View Config" link

Agent Detail Modal:

- YAML configuration viewer
- Model settings section
- Token limits display
- System prompt preview
- Test agent button
```

**Tools Tab** (`/resources?tab=tools`):

```md
Tools tab content:

Filter Bar (specific to tools):

- Search tools
- Type filter (TypeScript, Python, API)
- Category dropdown
- Usage (Most used, Recently added)

Tools Table:

- Columns: Name, Type, Description, Parameters, Usage Count
- Type badges: TypeScript, Python, API, etc.
- Expandable row for parameter details
- Code preview on hover

Tool Detail Modal:

- Source code viewer with syntax highlighting
- Parameter schema documentation
- Input/Output examples
- Recent usage statistics
- Test tool interface
```

**Tasks Tab** (`/resources?tab=tasks`):

```md
Tasks tab content:

Filter Bar (specific to tasks):

- Search tasks
- Type filter (Basic, Composite, Parallel, Router)
- Workflow/Category dropdown
- Dependencies filter

Task List:

- Grouped by category/workflow
- Collapsible groups
- Task cards within groups:
    - Task name with type icon
    - Type badge (Basic, Composite, Parallel, Router)
    - Dependencies list
    - "View Details" button

Task Detail Modal:

- Configuration viewer
- Dependency graph visualization
- Input/Output schemas
- Performance metrics
```

**MCP Servers Tab** (`/resources?tab=mcp`):

```md
MCP Servers tab content:

Filter Bar (specific to MCP):

- Search servers
- Connection status (Connected/Disconnected)
- Server type dropdown

MCP Server Cards:

- Server name with connection status
- URL/endpoint display
- Available tools count
- Connection health indicator
- Last ping timestamp

Server Actions:

- Test connection button
- View available tools
- Refresh tools list
- Enable/Disable toggle

Server Detail Modal:

- Connection configuration
- Available tools list with descriptions
- Request/Response logs
- Performance metrics
```

### 5. Schedules (`/schedules`)

**Purpose**: Manage scheduled workflow executions

**List View** (`/schedules`):

- Calendar view option
- List of schedules with cron expressions
- Next run time calculation
- Enable/disable toggles

```md
Create a schedules management page:

Header:

- Page title "Schedules" with calendar icon
- View toggle: List / Calendar
- "Create Schedule" button

Filter Bar (horizontal):

- Search schedules
- Workflow filter dropdown
- Status (Active/Inactive)
- Next run date range

Schedule List View:

- Table columns: □, Status, Workflow, Schedule, Next Run, Last Run, Actions
- Status: toggle switch (enabled/disabled)
- Schedule: human-readable + cron expression on hover
- Next Run: relative time (in 2 hours) with exact time on hover
- Actions: Edit, Delete, Run Now

Calendar View:

- Monthly calendar with scheduled runs as dots
- Click date to see schedules for that day
- Different colors for different workflows
- Today highlighted

Create/Edit Schedule Modal:

- Workflow selector dropdown
- Cron expression builder:
    - Quick presets (Hourly, Daily, Weekly, Monthly)
    - Custom cron input with validation
    - Next 5 runs preview
- Timezone selector
- Active/Inactive toggle
- Save/Cancel buttons

Schedule Detail:

- Execution history for this schedule
- Success/failure rate chart
- Average duration trend
- Modify or disable schedule
```

### 6. Monitoring (`/monitoring`)

**Purpose**: System health, metrics, and performance monitoring

**Main View** (`/monitoring`):

- System health dashboard
- Real-time metrics
- Performance charts
- Alert configuration

```md
Create a monitoring dashboard:

System Health Header:

- Large status indicator (Green/Yellow/Red)
- Overall health score (e.g., 98.5%)
- Last check timestamp
- "Run Health Check" button

Metrics Grid (2x2):

- CPU Usage: Gauge chart with percentage
- Memory Usage: Gauge chart with GB used/total
- Active Workers: Number with trend arrow
- Queue Depth: Number with mini sparkline

Performance Charts:

- Execution Duration Trend (line chart, last 24h)
- Success Rate (area chart with green/red)
- Throughput (bar chart, executions per hour)
- API Response Time (line chart with P50/P95/P99)

Alerts Section:

- Active alerts list with severity badges
- Alert rules configuration table
- Add alert rule button
- Test alert button

Logs Preview:

- Recent error logs (last 10)
- Log level filter buttons
- "View All Logs" link to full logs page
```

### 7. Settings (`/settings`)

**Purpose**: System configuration and user preferences

**Categories**:

- General settings
- API configuration
- Authentication
- Notifications
- Backup/Restore

```md
Create a settings page with sections:

Sidebar Navigation:

- Vertical menu with icons
- Sections: General, API, Authentication, Notifications, Backup
- Active section highlighted

General Settings:

- System name input
- Timezone selector
- Date format preference
- Language selector
- Theme toggle (Light/Dark)

API Configuration:

- API endpoint URL
- Timeout settings
- Rate limiting configuration
- CORS allowed origins
- API key management table

Authentication:

- Auth provider selector
- SSO configuration
- Session timeout
- Password policies
- Two-factor auth toggle

Notifications:

- Email notification settings
- Webhook configurations
- Alert thresholds
- Notification channels table

Backup/Restore:

- Backup schedule configuration
- Manual backup button
- Restore from backup interface
- Export/Import configurations
- Data retention policies

Save Bar:

- Sticky bottom bar
- "Save Changes" and "Cancel" buttons
- Unsaved changes indicator
```

## User Flows

### Execute a Workflow

1. Dashboard → Click "Execute Workflow" or navigate to Workflows
2. Select workflow from list
3. Fill in execution parameters
4. Click "Execute"
5. Redirected to execution detail page
6. Monitor real-time progress

### Create a Schedule

1. Navigate to Schedules
2. Click "Create Schedule"
3. Select workflow
4. Configure cron expression or use preset
5. Set timezone and active status
6. Save schedule
7. View in schedules list

### Debug Failed Execution

1. Go to Executions or see alert on Dashboard
2. Filter by "Failed" status
3. Click on failed execution
4. Review error logs and task details
5. Check input/output data
6. Click "Retry" with same or modified input

### Monitor System Health

1. Navigate to Monitoring
2. Check overall system health
3. Review performance metrics
4. Set up alerts for thresholds
5. View recent error logs
6. Take action based on alerts

## UI Components

### Common Components

- **Navigation Bar**: Fixed top bar with logo, main nav, user menu
- **Status Badges**: Consistent colors - green (success), blue (running), red (failed), gray (pending)
- **Action Buttons**: Primary (blue), Secondary (gray), Danger (red)
- **Data Tables**: Sortable columns, pagination, bulk actions
- **Code Editors**: Syntax highlighting for YAML/JSON
- **Real-time Indicators**: WebSocket connection status, auto-refresh
- **Empty States**: Helpful illustrations and CTAs
- **Loading States**: Skeleton screens and progress indicators

### Responsive Design

- **Mobile**: Single column, collapsible navigation, touch-friendly
- **Tablet**: 2-column layouts, sidebar navigation
- **Desktop**: Full multi-column layouts, persistent sidebars

### Theme

- Clean, modern design
- Primary color: Blue (#3B82F6)
- Success: Green (#10B981)
- Error: Red (#EF4444)
- Warning: Yellow (#F59E0B)
- Neutral grays for UI elements

## Error Handling

- User-friendly error messages
- Retry options where applicable
- Link to documentation
- Contact support option

## Future Enhancements

- Visual workflow builder
- Collaboration features
- Version control integration
- API playground
- Custom dashboards
- Mobile app
