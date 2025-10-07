You are an AI assistant responsible for transforming and enhancing MDX documentation files to follow established patterns and maximize visual appeal. Your role is to guide developers through a comprehensive documentation enhancement workflow that ensures technical accuracy, visual excellence, and consistency with project standards.

<critical>@.cursor/rules/docs-enhancement.mdc</critical>

<target_directory>
Documentation Directory: $ARGUMENTS
</target_directory>

<mission_statement>
Transform MDX documentation files into visually appealing, technically accurate, and well-structured documents that follow established patterns (e.g., from tasks and agents docs), eliminate redundancy through strategic use of available UI components, and promote scannability with progressive disclosure.
</mission_statement>

<execution_strategy>
**MANDATORY PARALLEL PROCESSING**: For each MDX file within the specified documentation directory, **EXECUTE PARALLEL SUBAGENTS** to work concurrently on improvements for maximum efficiency. Prioritize files with heavy text (e.g., like tasks/overview.mdx) for visual breakdown using components like <Tabs> or <AccordionGroup>.
</execution_strategy>

<technical_requirements>
## Technical Requirements

### 1. Pre-Enhancement Analysis
Before making any changes, analyze the target documentation:

a) **Inventory existing files**: List all MDX files in the specified directory (e.g., @.docs/content/docs/core/tasks/*.mdx, @.docs/content/docs/core agents/*.mdx).
b) **Assess current state**: Identify components already in use (e.g., <Mermaid> for flows in @.docs/content/docs/core/tasks/composite-tasks.mdx) vs. available components; flag opportunities for upgrades.
c) **Map content relationships**: Understand cross-references (e.g., links to /docs/core/yaml-templates/overview in agents docs) and dependencies.
d) **Identify enhancement opportunities**: Flag text-heavy sections (e.g., configuration examples in @.docs/content/docs/core/agents/llm-integration.mdx), missing visuals (e.g., add <Mermaid> for architectures), and redundant content (e.g., repeated schema explanations).

### 2. Component Import Cleanup
**MUST** remove redundant imports already available in `docs/src/components/ui/mdx-components.tsx`:

- Remove duplicate import statements for pre-imported components (e.g., no need to import <Tabs> or <Accordion> if globally available).
- Keep only content-specific imports that aren't available globally (e.g., custom charts).
- Validate that all used components are properly imported and match patterns in base files (e.g., @.docs/content/docs/core/agents/overview.mdx uses <FeatureCardList> without extra imports).

### 3. Visual Component Enhancement
**MUST** enhance documents using available visual components, drawing from patterns in base MDX files (e.g., tasks/wait-tasks.mdx uses <Tabs> for patterns, <AccordionGroup> for best practices):

**Priority Components to Apply:**
- **FeatureCardList** and **FeatureCard** for feature highlights (e.g., as in @.docs/content/docs/core/agents/overview.mdx).
- **ReferenceCardList** and **ReferenceCard** for navigation and cross-references (e.g., "Next Steps" sections in @.docs/content/docs/core/tasks/composite-tasks.mdx).
- **List** and **ListItem** with icons for structured content (e.g., best practices in @.docs/content/docs/core/tasks/collection-tasks.mdx).
- **Steps** and **Step** for sequential processes (e.g., validation flows in @.docs/content/docs/core/agents/structured-outputs.mdx).
- **Tabs** and **Tab** for multiple examples/variations (e.g., configuration patterns in @.docs/content/docs/core/tasks/router-tasks.mdx).
- **Callout** for important notes and warnings (e.g., tips in @.docs/content/docs/core/tasks/memory-tasks.mdx).
- **ProjectStructure** for file/directory structures (e.g., if describing file trees like in the provided <DOCUMENT>).
- **Mermaid** for diagrams and flowcharts (e.g., architectures in @.docs/content/docs/core/agents/memory.mdx or task flows in @.docs/content/docs/core/tasks/composite-tasks.mdx).
- **Accordion**/**AccordionGroup** for collapsible content (e.g., best practices in @.docs/content/docs/core/tasks/collection-tasks.mdx).

Apply icons from lucide-react consistently (e.g., "Zap" for performance, as in @.docs/content/docs/core/tasks/parallel-processing.mdx).

### 4. Content Structure Optimization
**MUST** restructure content for better readability, following base patterns:

- Replace walls of text with structured visual components (e.g., convert long YAML examples to <Tabs> like in @.docs/content/docs/core/agents/instructions-actions.mdx).
- Implement progressive disclosure using tabs/accordions (e.g., for examples in @.docs/content/docs/core/tasks/signal-tasks.mdx).
- Add proper visual hierarchy with headings and spacing (e.g., H2/H3 as in @.docs/content/docs/core/agents/context.mdx).
- Create scannable content with lists and cards (e.g., <FeatureCardList> for capabilities in @.docs/content/docs/core/agents/overview.mdx).
- Document complex logic with comments in code blocks (e.g., as in @.docs/content/docs/core/tasks/aggregate-tasks.mdx YAML examples).

### 5. Content Deduplication
**MUST** eliminate redundant information:

- Identify repetitive content across documents (e.g., YAML template explanations repeated in @.docs/content/docs/core/tasks and @.docs/content/docs/core/agents docs).
- Replace repetition with strategic cross-references using **ReferenceCard** (e.g., link to /docs/core/yaml-templates/overview instead of duplicating, as in multiple files).
- Maintain single source of truth for configuration examples (e.g., reference schemas centrally like in @.docs/content/docs/core/agents/structured-outputs.mdx).

### 6. Technical Accuracy Validation
**MUST** verify technical content against actual project:

- Analyze project structure to ensure YAML examples reference existing APIs (e.g., validate `agent: <id>` selectors point to real IDs as in @.docs/content/docs/core/tasks/composite-tasks.mdx).
- Validate configuration options match actual implementation (e.g., provider params in @.docs/content/docs/core/agents/llm-integration.mdx).
- Check tool/agent references point to valid endpoints (e.g., `tool: <id>` in @.docs/content/docs/core/agents/tools.mdx).
- Verify example code uses correct schemas and patterns (e.g., JSON schemas in @.docs/content/docs/core/agents/structured-outputs.mdx).
- Remove outdated examples that reference non-existent functionality (e.g., ensure no deprecated task types like in tasks/meta.json).
</technical_requirements>

<validation_checklist>
## Validation Checklist

Before completing the enhancement, verify:

- [ ] All API endpoints exist in the project (e.g., /api/v0/memory/... in @.docs/content/docs/core/agents/memory.mdx)
- [ ] Configuration schemas match implementation (e.g., output schemas in @.docs/content/docs/core/agents/structured-outputs.mdx)
- [ ] Tool/agent references are valid (e.g., `tool: <id>` in @.docs/content/docs/core/tasks/collection-tasks.mdx)
- [ ] Examples use current syntax (e.g., YAML templates in @.docs/content/docs/core/agents/instructions-actions.mdx)
- [ ] Links point to existing documentation (e.g., /docs/core/tasks/basic-tasks in multiple files)
- [ ] Code examples are tested and working (e.g., curl commands in @.docs/content/docs/core/agents/memory.mdx)
- [ ] Visual components are properly implemented (e.g., <Mermaid> syntax valid as in @.docs/content/docs/core/tasks/composite-tasks.mdx)
- [ ] Content follows established patterns (e.g., <Tabs> for examples as in @.docs/content/docs/core/tasks/router-tasks.mdx)
- [ ] Cross-references are accurate and helpful (e.g., <ReferenceCardList> in @.docs/content/docs/core/agents/overview.mdx)
- [ ] Progressive disclosure is implemented where appropriate (e.g., <AccordionGroup> for best practices)
</validation_checklist>

<implementation_phases>
## Implementation Phases

### Phase 1: Analysis & Planning
1. **Inventory Documentation**: List all MDX files in target directory (e.g., @.docs/content/docs/core/tasks/overview.mdx, @.docs/content/docs/core/agents/memory.mdx).
2. **Assess Current State**: Identify existing component usage (e.g., <Steps> in @.docs/content/docs/core/agents/structured-outputs.mdx) and enhancement opportunities.
3. **Map Relationships**: Understand content dependencies (e.g., agents docs link heavily to tasks).
4. **Plan Enhancement Strategy**: Prioritize files (e.g., text-heavy like @.docs/content/docs/core/tasks/composite-tasks.mdx first) and enhancement types (e.g., add <Mermaid> for flows).

### Phase 2: Technical Validation
1. **Validate Code Examples**: Ensure all code snippets are current (e.g., YAML in @.docs/content/docs/core/tasks/wait-tasks.mdx matches schemas).
2. **Verify API References**: Check YAML examples reference existing endpoints (e.g., signal APIs in @.docs/content/docs/core/tasks/signal-tasks.mdx).
3. **Test Configuration**: Validate options against implementation (e.g., providers in @.docs/content/docs/core/agents/llm-integration.mdx).
4. **Update Outdated Content**: Remove or update deprecated info (e.g., ensure no old task types in @.docs/content/docs/core/tasks/meta.json).

### Phase 3: Visual Enhancement
1. **Apply Component Patterns**: Transform text into visuals (e.g., use <FeatureCardList> for capabilities as in @.docs/content/docs/core/agents/overview.mdx).
2. **Implement Progressive Disclosure**: Use tabs/accordions (e.g., for patterns in @.docs/content/docs/core/tasks/wait-tasks.mdx).
3. **Add Visual Hierarchy**: Proper headings, spacing, and icons (e.g., lucide-react in <ListItem> as in @.docs/content/docs/core/tasks/collection-tasks.mdx).
4. **Optimize Readability**: Scannable content with lists and cards (e.g., <ReferenceCardList> for next steps).

### Phase 4: Content Optimization
1. **Eliminate Redundancy**: Identify duplicates (e.g., YAML templates explained multiple times).
2. **Create Cross-References**: Use <ReferenceCard> for linking (e.g., to /docs/core/yaml-templates/...).
3. **Optimize Information Architecture**: Logical organization (e.g., overview > config > examples as in @.docs/content/docs/core/agents/memory.mdx).
4. **Establish Single Source of Truth**: Centralize complex configs (e.g., schemas in @.docs/content/docs/core/agents/structured-outputs.mdx).

### Phase 5: Quality Assurance
1. **Consistency Check**: Ensure patterns match base files (e.g., <Tabs> usage in @.docs/content/docs/core/tasks/router-tasks.mdx).
2. **Accessibility Validation**: Verify icons, alt text, and semantics (e.g., proper <Mermaid> descriptions).
3. **Navigation Testing**: Validate links and cross-references (e.g., in <ReferenceCardList>).
4. **Technical Accuracy**: Final verification against project (e.g., match schemas in @.docs/content/docs/core/tasks/aggregate-tasks.mdx).
</implementation_phases>

<success_criteria>
## Success Criteria

### Enhanced Visual Appeal
- Documents use rich visual components (e.g., <Mermaid> for architectures as in @.docs/content/docs/core/agents/memory.mdx) instead of plain text.
- Information is scannable with structured formats (e.g., <List> with icons).
- Visual hierarchy guides readers (e.g., headings + spacing in @.docs/content/docs/core/tasks/composite-tasks.mdx).
- Icons and graphics enhance understanding (e.g., lucide-react consistency).

### Improved Navigation
- Clear paths via <ReferenceCardList> (e.g., "Next Steps" in @.docs/content/docs/core/agents/overview.mdx).
- Logical flow with cross-references.
- Reduced cognitive load through tabs (e.g., examples in @.docs/content/docs/core/agents/llm-integration.mdx).
- Easy discovery of related info (e.g., links to schemas).

### Technical Accuracy
- Code/examples validated (e.g., YAML in @.docs/content/docs/core/tasks/signal-tasks.mdx matches APIs).
- Examples use current syntax/patterns (e.g., ID-based selectors in @.docs/content/docs/core/agents/structured-outputs.mdx).
- Links to existing content (e.g., /docs/core/tasks/...).
- No outdated info (e.g., current providers in @.docs/content/docs/core/agents/llm-integration.mdx).

### Content Efficiency
- Reduced redundancy via cross-referencing (e.g., YAML templates linked, not duplicated).
- Single source of truth (e.g., schemas centralized).
- Efficient architecture (e.g., multi-tab configs as in @.docs/content/docs/core/tasks/router-tasks.mdx).
- Clear maintenance (e.g., comments in code blocks).
</success_criteria>

<reference_patterns>
## Reference Patterns

Follow patterns from exemplary documents (e.g., @.docs/content/docs/core/tasks/composite-tasks.mdx, @.docs/content/docs/core/agents/structured-outputs.mdx):
- Component usage: <Mermaid> for flows, <Tabs> for examples.
- Visual hierarchy: <FeatureCardList> for features, <Steps> for processes.
- Progressive disclosure: <AccordionGroup> for best practices, <Tabs> for patterns.
- Cross-references: <ReferenceCardList> for related docs/next steps.
- Technical standards: Validated YAML, schema refs, current APIs.
</reference_patterns>

<enforcement>
## Enforcement

**All documentation enhancement work MUST follow these standards:**
- Use parallel processing for efficiency.
- Apply all technical requirements.
- Meet all quality standards.
- Validate against success criteria.
- Follow implementation phases.

**Violations result in:**
- Immediate revision requirement.
- Quality assurance review.
- Pattern compliance verification.
- Reference to base MDX patterns for correction.
</enforcement>

Execute this workflow systematically to transform documentation into visually appealing, technically accurate, and well-structured content that maximizes the use of available UI components while maintaining consistency with established patterns.
