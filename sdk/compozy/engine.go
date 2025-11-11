package compozy

import (
	"context"
	"net"
	"net/http"
	"sync"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	enginememory "github.com/compozy/compozy/engine/memory"
	engineproject "github.com/compozy/compozy/engine/project"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	"github.com/compozy/compozy/engine/resources"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetool "github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/tool/inline"
	enginewebhook "github.com/compozy/compozy/engine/webhook"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/go-chi/chi/v5"

	sdkclient "github.com/compozy/compozy/sdk/v2/client"
)

// Engine represents an instantiated Compozy SDK core.
type Engine struct {
	ctx context.Context

	mode Mode
	host string
	port int

	project        *engineproject.Config
	workflows      []*engineworkflow.Config
	agents         []*engineagent.Config
	tools          []*enginetool.Config
	knowledgeBases []*engineknowledge.BaseConfig
	memories       []*enginememory.Config
	mcps           []*enginemcp.Config
	schemas        []*engineschema.Schema
	models         []*core.ProviderConfig
	schedules      []*projectschedule.Config
	webhooks       []*enginewebhook.Config

	standaloneTemporal *StandaloneTemporalConfig
	standaloneRedis    *StandaloneRedisConfig

	resourceStore resources.ResourceStore
	router        *chi.Mux
	server        *http.Server
	listener      net.Listener
	client        *sdkclient.Client

	configSnapshot *appconfig.Config

	serverCancel context.CancelFunc
	serverWG     sync.WaitGroup

	modeCleanups  []modeCleanup
	inlineManager *inline.Manager

	stateMu sync.RWMutex
	started bool

	startMu sync.Mutex
	stopMu  sync.Mutex

	errMu     sync.Mutex
	serverErr error
	startErr  error
	stopErr   error
	baseURL   string
}
