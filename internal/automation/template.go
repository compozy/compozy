package automation

import (
	"text/template"

	modelpkg "github.com/compozy/agh/internal/automation/model"
)

// ParseTriggerPromptTemplate parses a trigger prompt template with strict activation-envelope validation.
func ParseTriggerPromptTemplate(prompt string) (*template.Template, error) {
	return modelpkg.ParseTriggerPromptTemplate(prompt)
}

// ValidateTriggerPromptTemplate validates a trigger prompt template against the normalized activation-envelope model.
func ValidateTriggerPromptTemplate(prompt string) error {
	return modelpkg.ValidateTriggerPromptTemplate(prompt)
}
