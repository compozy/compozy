package nats

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func Test_parseSubject(t *testing.T) {
	t.Run("Valid command subject", func(t *testing.T) {
		data, typeValue, err := parseSubject("compozy.corr123.workflow.cmds.wf456.execute", SegmentCmd)
		assert.NoError(t, err)
		assert.Equal(t, ComponentType("workflow"), data.CompType)
		assert.Equal(t, common.ID("wf456"), data.ExecID)
		assert.Equal(t, common.ID("corr123"), data.CorrID)
		assert.Equal(t, "execute", typeValue)
	})

	t.Run("Valid event subject", func(t *testing.T) {
		data, typeValue, err := parseSubject("compozy.corr123.task.evts.task456.started", SegmentEvent)
		assert.NoError(t, err)
		assert.Equal(t, ComponentType("task"), data.CompType)
		assert.Equal(t, common.ID("task456"), data.ExecID)
		assert.Equal(t, common.ID("corr123"), data.CorrID)
		assert.Equal(t, "started", typeValue)
	})

	t.Run("Valid log subject", func(t *testing.T) {
		data, typeValue, err := parseSubject("compozy.corr123.agent.logs.agent456.info", SegmentLog)
		assert.NoError(t, err)
		assert.Equal(t, ComponentType("agent"), data.CompType)
		assert.Equal(t, common.ID("agent456"), data.ExecID)
		assert.Equal(t, common.ID("corr123"), data.CorrID)
		assert.Equal(t, "info", typeValue)
	})

	t.Run("Too few parts", func(t *testing.T) {
		_, _, err := parseSubject("compozy.corr123.task.evts.task456", SegmentEvent)
		assert.Error(t, err)
		assert.Equal(t, "invalid subject format: compozy.corr123.task.evts.task456, expected at least 6 parts", err.Error())
	})

	t.Run("Invalid prefix", func(t *testing.T) {
		_, _, err := parseSubject("invalidprefix.corr123.task.evts.task456.started", SegmentEvent)
		assert.Error(t, err)
		assert.Equal(t, "invalid subject prefix: invalidprefix, expected \"compozy\"", err.Error())
	})

	t.Run("Invalid segment", func(t *testing.T) {
		_, _, err := parseSubject("compozy.corr123.task.invalid.task456.started", SegmentEvent)
		assert.Error(t, err)
		assert.Equal(t, "invalid segment type: invalid, expected \"evts\"", err.Error())
	})
}

func Test_ParseEventSubject(t *testing.T) {
	t.Run("Valid event subject", func(t *testing.T) {
		evtSubject, err := ParseEvtSubject("compozy.corr123.task.evts.task456.started")
		assert.NoError(t, err)
		assert.NotNil(t, evtSubject)
		assert.Equal(t, ComponentType("task"), evtSubject.CompType)
		assert.Equal(t, common.ID("task456"), evtSubject.ExecID)
		assert.Equal(t, common.ID("corr123"), evtSubject.CorrID)
		assert.Equal(t, EvtType("started"), evtSubject.EventType)
	})

	t.Run("Invalid segment type", func(t *testing.T) {
		_, err := ParseEvtSubject("compozy.corr123.task.cmds.task456.execute")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected \"evts\"")
	})

	t.Run("Too few parts", func(t *testing.T) {
		_, err := ParseEvtSubject("compozy.corr123.task.evts.task456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected at least 6 parts")
	})
}

func Test_ParseCommandSubject(t *testing.T) {
	t.Run("Valid command subject", func(t *testing.T) {
		cmdSubject, err := ParseCmdSubject("compozy.corr123.workflow.cmds.wf456.execute")
		assert.NoError(t, err)
		assert.NotNil(t, cmdSubject)
		assert.Equal(t, ComponentType("workflow"), cmdSubject.CompType)
		assert.Equal(t, common.ID("wf456"), cmdSubject.ExecID)
		assert.Equal(t, common.ID("corr123"), cmdSubject.CorrID)
		assert.Equal(t, CmdType("execute"), cmdSubject.CommandType)
	})

	t.Run("Invalid segment type", func(t *testing.T) {
		_, err := ParseCmdSubject("compozy.corr123.workflow.evts.wf456.started")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected \"cmds\"")
	})

	t.Run("Too few parts", func(t *testing.T) {
		_, err := ParseCmdSubject("compozy.corr123.workflow.cmds.wf456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected at least 6 parts")
	})
}

func Test_ParseLogSubject(t *testing.T) {
	t.Run("Valid log subject - debug", func(t *testing.T) {
		logSubject, err := ParseLogSubject("compozy.corr123.agent.logs.agent456.debug")
		assert.NoError(t, err)
		assert.NotNil(t, logSubject)
		assert.Equal(t, ComponentType("agent"), logSubject.CompType)
		assert.Equal(t, common.ID("agent456"), logSubject.ExecID)
		assert.Equal(t, common.ID("corr123"), logSubject.CorrID)
		assert.Equal(t, logger.DebugLevel, logSubject.LogLevel)
	})

	t.Run("Valid log subject - uppercase INFO", func(t *testing.T) {
		logSubject, err := ParseLogSubject("compozy.corr123.agent.logs.agent456.INFO")
		assert.NoError(t, err)
		assert.NotNil(t, logSubject)
		assert.Equal(t, logger.InfoLevel, logSubject.LogLevel)
	})

	t.Run("Invalid log level - defaults to info", func(t *testing.T) {
		logSubject, err := ParseLogSubject("compozy.corr123.agent.logs.agent456.invalid")
		assert.NoError(t, err)
		assert.NotNil(t, logSubject)
		assert.Equal(t, logger.InfoLevel, logSubject.LogLevel)
	})

	t.Run("Invalid segment type", func(t *testing.T) {
		_, err := ParseLogSubject("compozy.corr123.agent.evts.agent456.started")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected \"logs\"")
	})

	t.Run("Too few parts", func(t *testing.T) {
		_, err := ParseLogSubject("compozy.corr123.agent.logs.agent456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected at least 6 parts")
	})
}

func Test_BuildEventSubject(t *testing.T) {
	t.Run("Workflow started event", func(t *testing.T) {
		subject := BuildEvtSubject(ComponentType("workflow"), common.ID("corr123"), common.ID("wf456"), EvtType("started"))
		assert.Equal(t, "compozy.corr123.workflow.evts.wf456.started", subject)
	})

	t.Run("Task success event", func(t *testing.T) {
		subject := BuildEvtSubject(ComponentType("task"), common.ID("corr456"), common.ID("task789"), EvtType("success"))
		assert.Equal(t, "compozy.corr456.task.evts.task789.success", subject)
	})

	t.Run("Agent failed event", func(t *testing.T) {
		subject := BuildEvtSubject(ComponentType("agent"), common.ID("corr789"), common.ID("agent123"), EvtType("failed"))
		assert.Equal(t, "compozy.corr789.agent.evts.agent123.failed", subject)
	})
}

func Test_BuildCommandSubject(t *testing.T) {
	t.Run("Workflow execute command", func(t *testing.T) {
		subject := BuildCmdSubject(ComponentType("workflow"), common.ID("corr123"), common.ID("wf456"), CmdType("execute"))
		assert.Equal(t, "compozy.corr123.workflow.cmds.wf456.execute", subject)
	})

	t.Run("Task cancel command", func(t *testing.T) {
		subject := BuildCmdSubject(ComponentType("task"), common.ID("corr456"), common.ID("task789"), CmdType("cancel"))
		assert.Equal(t, "compozy.corr456.task.cmds.task789.cancel", subject)
	})

	t.Run("Tool trigger command", func(t *testing.T) {
		subject := BuildCmdSubject(ComponentType("tool"), common.ID("corr789"), common.ID("tool123"), CmdType("trigger"))
		assert.Equal(t, "compozy.corr789.tool.cmds.tool123.trigger", subject)
	})
}

func Test_BuildLogSubject(t *testing.T) {
	t.Run("Workflow debug log", func(t *testing.T) {
		subject := BuildLogSubject(ComponentType("workflow"), common.ID("corr123"), common.ID("wf456"), logger.DebugLevel)
		assert.Equal(t, "compozy.corr123.workflow.logs.wf456.debug", subject)
	})

	t.Run("Task info log", func(t *testing.T) {
		subject := BuildLogSubject(ComponentType("task"), common.ID("corr456"), common.ID("task789"), logger.InfoLevel)
		assert.Equal(t, "compozy.corr456.task.logs.task789.info", subject)
	})

	t.Run("Agent error log", func(t *testing.T) {
		subject := BuildLogSubject(ComponentType("agent"), common.ID("corr789"), common.ID("agent123"), logger.ErrorLevel)
		assert.Equal(t, "compozy.corr789.agent.logs.agent123.error", subject)
	})
}
