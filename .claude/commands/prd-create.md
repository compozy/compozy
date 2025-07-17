You are an AI assistant tasked with creating a detailed Product Requirements Document (PRD) in Markdown format. Your goal is to produce a comprehensive PRD that focuses on user needs, functional requirements, and business goals to clearly define what to build and why.

**YOU MUST USE** --deepthink

You will be given an initial prompt describing a feature or functionality. Here is the prompt:

<initial_prompt>$ARGUMENTS</initial_prompt>

## Template Reference

<template_reference>
**ALWAYS use the standardized PRD template:** tasks/docs/\_prd-template.md
This template provides a comprehensive structure that balances requirements gathering with rollout planning, ensuring consistency across all PRDs in the project.
</template_reference>

## Process
<process_workflow>
1. **Receive Initial Prompt:** The user provides a brief description or request for a new feature or functionality.

2. **Ask Clarifying Questions:** Before writing the PRD, the AI _must_ ask clarifying questions to gather sufficient detail. Focus on understanding the "what" and "why" of the feature, user needs, and success criteria.

<mandatory_step>
3. **Create PRD Planning with Zen MCP:** Use zen's planner tool to create a comprehensive PRD development plan:
  - Analyze the enriched feature specification requirements
  - Break down PRD creation into logical planning steps
  - Identify key sections that need focused attention
  - Plan resource allocation and approach for each section
  - Document assumptions and dependencies that will guide PRD creation
</mandatory_step>

4. **Validate Planning with Consensus:** Use zen's consensus tool with o3 and gemini 2.5 models:
   - Present the detailed PRD planning approach to both expert models
   - Request critical analysis of the planning strategy
   - Gather feedback on plan completeness and effectiveness
   - Incorporate consensus recommendations into final planning approach
   - Proceed only after receiving aligned approval from both expert models

5. **Generate the PRD Document:** Using the standardized template, draft a comprehensive PRD that details user needs, business objectives, and high-level product requirements. **Exclude all low-level technical design or implementation specifics; these are reserved for the Tech Spec.**

6. **Create Feature Directory:** Direct the creation of a dedicated feature folder at `./tasks/prd-[feature-slug]/`.

7. **Store the PRD:** Save the completed PRD as `_prd.md` within the newly created feature folder.
</process_workflow>

<clarifying_questions_guidance>
## Clarifying Questions (Examples)

The AI should adapt its questions based on the prompt and template sections. Here are key areas to explore:

**Problem & Goals:**
- "What problem does this feature solve for the user?"
- "What are the specific, measurable goals we want to achieve?"
- "How will we measure success?"

**Users & Stories:**
- "Who is the primary user of this feature?"
- "Can you provide user stories? (As a [type of user], I want to [action] so that [benefit])"
- "What are the key user flows and interactions?"

**Core Functionality:**
- "What are the essential features that must be included in the MVP?"
- "Can you describe the key actions a user should be able to perform?"
- "What data does this feature need to display or manipulate?"

**Technical Constraints (acceptance criteria only – describe _what_ must be met, not _how_ to meet it):**
- "Are there any existing systems this needs to integrate with?"
- "What are the performance thresholds or security requirements? (e.g., must handle X users, must comply with Y standard)"
- "Are there any technical constraints that limit what can be built?"
- Note: Capture only acceptance-criteria level thresholds; defer solution approaches to Tech Spec

**Scope & Planning:**
- "What should this feature NOT do (non-goals)?"
- "How should development be phased for incremental delivery?"
- "What are the dependencies between different parts of this feature?"

**Risks & Challenges:**
- "What are the biggest risks or challenges you foresee?"
- "Are there any unknowns that need research before implementation?"
- "What could prevent this feature from being successful?"

**Design & Experience:**
- "Are there any design mockups or UI guidelines to follow?"
- "What accessibility requirements should be considered?"
- "How should this feature integrate with the existing user experience?"
</clarifying_questions_guidance>

## PRD Structure Requirements

<prd_structure_requirements>
The generated PRD MUST follow the template structure from @\_prd-template.md:

1. **Overview:** Problem statement, target users, and value proposition
2. **Goals:** Specific, measurable objectives and business outcomes
3. **User Stories:** Detailed narratives covering primary and edge case scenarios
4. **Core Features:** Main functionality with detailed functional requirements
5. **User Experience:** User journeys, flows, UI/UX considerations, and accessibility
6. **High-Level Technical Constraints:** Integration points, compliance mandates, performance thresholds (avoid architectural diagrams or code-level solutions)
7. **Non-Goals (Out of Scope):** Clear boundaries and excluded features
8. **Phased Rollout Plan:** User-facing milestones with MVP and enhancement stages
9. **Success Metrics:** Measurable outcomes for user engagement and business impact
10. **Risks and Mitigations:** Potential challenges and response strategies
11. **Open Questions:** Unresolved items requiring further clarification
12. **Appendix:** Supporting materials, research, and reference documentation
</prd_structure_requirements>

## Content Guidelines

<content_guidelines>
**Target Audience:** Assume readers include both **junior developers** and **project stakeholders**. Requirements should be:

- Explicit and unambiguous
- Detailed enough for implementation
- Strategic enough for decision-making
- Avoid technical jargon without explanation

**Functional Requirements:** Use clear, actionable language:
- "The system must allow users to..."
- "Users should be able to..."
- Number requirements for easy reference

**Delivery Considerations (high-level only):** Balance user needs with practical rollout:
- Consider MVP vs. full feature scope from a user value perspective
- Plan for incremental delivery of user value
- Identify which features deliver the most user/business value first
- Note: Detailed technical sequencing and dependencies belong in the Tech Spec

**Separation of Concerns:**
- Keep the PRD centered on _what_ the product should achieve, not _how_ it will be built
- Capture only high-level technical constraints (e.g., required throughput, compliance mandates); defer architectural or code-level solutions to the Tech Spec
- If detailed design ideas arise, note them as TODOs for the Tech Spec author or move them to an appendix reference rather than the main PRD body

**Document Size Guidelines:**
- Target a maximum length of ~3,000 words (≈7–8 pages)
- Prefer concise bullet points, tables, and links to external references over verbose narrative paragraphs
- Offload large data sets, research studies, or extended examples to appendices or separate reference documents to keep the core PRD lightweight
</content_guidelines>

<output_specification>
- **Format:** Markdown (`.md`)
- **Location:** `./tasks/prd-[feature-slug]/`
- **Filename:** `_prd.md`
- **Template:** Use @\_prd-template.md structure
</output_specification>

<workflow_instructions>
## Workflow Instructions

1. **ALWAYS** ask clarifying questions first to gather comprehensive information
2. **MUST THEN** use zen's planner tool to create comprehensive PRD development plan
3. **MUST VALIDATE** planning approach using zen's consensus tool with o3 and gemini 2.5 models
4. **DO NOT** proceed to PRD creation without completing both mandatory planning steps
5. **USE** the standardized template structure for consistency
6. **FOCUS** on comprehensive user and business requirements; defer implementation planning to the Tech Spec
7. **ENSURE** the PRD is actionable for both development and project management
8. **ITERATE** on the PRD based on user feedback and additional clarification
</workflow_instructions>

<quality_checklist>
## Quality Checklist

Before finalizing the PRD, ensure:

**Planning Validation:**
- [ ] Used zen's planner tool to create comprehensive PRD development plan
- [ ] Validated planning approach with zen's consensus tool using o3 and gemini 2.5 models
- [ ] Incorporated consensus recommendations into final planning approach
- [ ] Received aligned approval from both expert models before proceeding

**PRD Content Quality:**
- [ ] All template sections are completed with relevant information
- [ ] User stories cover primary flows and edge cases
- [ ] Functional requirements are numbered and specific
- [ ] High-level constraints (e.g., required integrations, compliance) are defined
- [ ] Phased rollout plan shows clear user-facing milestones and product-level dependencies
- [ ] Success metrics are measurable and relevant
- [ ] Risks are identified with mitigation strategies
- [ ] Open questions capture any remaining uncertainties
- [ ] Document is ≤ 3,000 words (core sections) – move overflow to Appendix
- [ ] No technical implementation details or architecture decisions included
</quality_checklist>
