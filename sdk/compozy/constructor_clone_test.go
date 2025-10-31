package compozy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloneConfigEmptySlices(t *testing.T) {
	assert.Empty(t, cloneWorkflowConfigs(nil))
	assert.Empty(t, cloneAgentConfigs(nil))
	assert.Empty(t, cloneToolConfigs(nil))
	assert.Empty(t, cloneKnowledgeConfigs(nil))
	assert.Empty(t, cloneMemoryConfigs(nil))
	assert.Empty(t, cloneMCPConfigs(nil))
	assert.Empty(t, cloneSchemaConfigs(nil))
	assert.Empty(t, cloneModelConfigs(nil))
	assert.Empty(t, cloneScheduleConfigs(nil))
	assert.Empty(t, cloneWebhookConfigs(nil))
}
