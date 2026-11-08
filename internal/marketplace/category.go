package marketplace

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	CategoryProductivity   = "productivity"
	CategoryCommunication  = "communication"
	CategoryDesign         = "design"
	CategoryDevelopment    = "development"
	CategoryData           = "data"
	CategoryCloud          = "cloud"
	CategoryFinance        = "finance"
	CategorySalesMarketing = "sales-marketing"
	CategoryResearch       = "research"
	CategorySecurity       = "security"
	CategoryAiAgents       = "ai-agents"
	CategoryMore           = "more"
	CategoryFeatured       = "featured"
)

type CategoryID string
type Category struct {
	ID    CategoryID
	Label string
}

var ErrUnknownCategory = errors.New("marketplace_category_unknown")
var taxonomy = []Category{
	{CategoryProductivity, "Productivity"}, {CategoryCommunication, "Communication"},
	{CategoryDesign, "Design"}, {CategoryDevelopment, "Development"}, {CategoryData, "Data & Analytics"},
	{CategoryCloud, "Cloud & Infrastructure"}, {CategoryFinance, "Finance & Payments"},
	{CategorySalesMarketing, "Sales & Marketing"}, {CategoryResearch, "Research & Learning"},
	{CategorySecurity, "Security"}, {CategoryAiAgents, "AI & Agents"}, {CategoryMore, "More"},
}

func Categories() []Category { return slices.Clone(taxonomy) }
func ParseCategoryID(value string) (CategoryID, error) {
	for _, category := range taxonomy {
		if value == string(category.ID) {
			return category.ID, nil
		}
	}
	return "", fmt.Errorf("%w: unknown category %q", ErrUnknownCategory, value)
}
func ResolveCategory(explicit, sourceCategory string) CategoryID {
	if id, err := ParseCategoryID(explicit); err == nil {
		return id
	}
	normalized := strings.Join(
		strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(strings.ToLower(sourceCategory))),
		" ",
	)
	for id, aliases := range categoryAliases {
		if slices.Contains(aliases, normalized) {
			return id
		}
	}
	return CategoryMore
}

var categoryAliases = map[CategoryID][]string{
	CategoryProductivity: {
		CategoryProductivity,
		"documents and files",
		"scheduling",
		"automation",
		"knowledge",
		"notes",
	},
	CategoryCommunication:  {CategoryCommunication, "inbox and collaboration", "customer support", "chat", "email"},
	CategoryDesign:         {CategoryDesign, "canvas", "creativity"},
	CategoryDevelopment:    {CategoryDevelopment, "developer tools", "testing", "migration", "code"},
	CategoryData:           {CategoryData, "database", "data analytics", "data and analytics", "analytics"},
	CategoryCloud:          {"infrastructure", "deployment", "monitoring", CategoryCloud, "devops"},
	CategoryFinance:        {CategoryFinance, "payments", "finance and legal"},
	CategorySalesMarketing: {"sales marketing", "sales", "marketing"},
	CategoryResearch:       {CategoryResearch, "learning", "education and research", "math"},
	CategorySecurity:       {CategorySecurity},
	CategoryAiAgents:       {"ai agents", "agent orchestration", "ai", "agents"},
	CategoryMore:           {CategoryMore, "location"},
}

func categoryFor(id CategoryID) Category {
	for _, c := range taxonomy {
		if c.ID == id {
			return c
		}
	}
	return taxonomy[len(taxonomy)-1]
}
func curatedSource(name string) bool {
	return name == CatalogSource || name == "compozy-plugins"
}

func curatedIdentity(name, ref string) bool {
	return curatedSource(name) && ((name == CatalogSource && ref == CompozyCatalogRef) ||
		(name == "compozy-plugins" && ref == "github:compozy/plugins"))
}
