#!/bin/bash

# QA Test Case Generator
# Interactive workflow for creating comprehensive test cases with automation annotations
# Usage: ./generate_test_cases.sh [qa-output-path/test-cases]

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}╔══════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║         QA Test Case Generator                   ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════╝${NC}"
echo ""

# Resolve output directory
OUTPUT_DIR="${1:-.}"
mkdir -p "$OUTPUT_DIR"

# Helper functions
prompt_input() {
    local prompt_text="$1"
    local var_name="$2"
    local required="$3"

    while true; do
        echo -e "${CYAN}${prompt_text}${NC}"
        read -r input

        if [ -n "$input" ]; then
            eval "$var_name=\"$input\""
            break
        elif [ "$required" != "true" ]; then
            eval "$var_name=\"\""
            break
        else
            echo -e "${RED}This field is required.${NC}"
        fi
    done
}

# Step 1: Basic Info
echo -e "${MAGENTA}━━━ Step 1: Test Case Basics ━━━${NC}"
echo ""

prompt_input "Test Case ID (e.g., TC-FUNC-001, TC-UI-045):" TC_ID true
prompt_input "Test Case Title:" TC_TITLE true

echo ""
echo "Priority:"
echo "1) P0 - Critical (blocks release)"
echo "2) P1 - High (important features)"
echo "3) P2 - Medium (nice to have)"
echo "4) P3 - Low (minor issues)"
echo ""

prompt_input "Select priority (1-4):" PRIORITY_NUM true

case $PRIORITY_NUM in
    1) PRIORITY="P0 (Critical)" ;;
    2) PRIORITY="P1 (High)" ;;
    3) PRIORITY="P2 (Medium)" ;;
    4) PRIORITY="P3 (Low)" ;;
    *) PRIORITY="P2 (Medium)" ;;
esac

echo ""
echo "Test Type:"
echo "1) Functional"
echo "2) UI/Visual"
echo "3) Integration"
echo "4) Regression"
echo "5) Performance"
echo "6) Security"
echo ""

prompt_input "Select test type (1-6):" TYPE_NUM true

case $TYPE_NUM in
    1) TEST_TYPE="Functional" ;;
    2) TEST_TYPE="UI/Visual" ;;
    3) TEST_TYPE="Integration" ;;
    4) TEST_TYPE="Regression" ;;
    5) TEST_TYPE="Performance" ;;
    6) TEST_TYPE="Security" ;;
    *) TEST_TYPE="Functional" ;;
esac

prompt_input "Estimated test time (minutes):" EST_TIME false

# Step 2: Automation Strategy
echo ""
echo -e "${MAGENTA}━━━ Step 2: Automation Strategy ━━━${NC}"
echo ""

echo "Automation Target:"
echo "1) E2E"
echo "2) Integration"
echo "3) Manual-only"
echo "4) N/A"
echo ""

prompt_input "Select automation target (1-4):" AUTOMATION_TARGET_NUM true

case $AUTOMATION_TARGET_NUM in
    1) AUTOMATION_TARGET="E2E" ;;
    2) AUTOMATION_TARGET="Integration" ;;
    3) AUTOMATION_TARGET="Manual-only" ;;
    4) AUTOMATION_TARGET="N/A" ;;
    *) AUTOMATION_TARGET="N/A" ;;
esac

if [ "$AUTOMATION_TARGET" = "Manual-only" ] || [ "$AUTOMATION_TARGET" = "N/A" ]; then
    AUTOMATION_STATUS="N/A"
    AUTOMATION_COMMAND="N/A"
else
    echo ""
    echo "Automation Status:"
    echo "1) Existing - matching coverage already exists"
    echo "2) Missing - repository supports it but coverage is absent"
    echo "3) Blocked - harness exists but prerequisites are missing"
    echo "4) N/A"
    echo ""

    prompt_input "Select automation status (1-4):" AUTOMATION_STATUS_NUM true

    case $AUTOMATION_STATUS_NUM in
        1) AUTOMATION_STATUS="Existing" ;;
        2) AUTOMATION_STATUS="Missing" ;;
        3) AUTOMATION_STATUS="Blocked" ;;
        4) AUTOMATION_STATUS="N/A" ;;
        *) AUTOMATION_STATUS="Missing" ;;
    esac

    prompt_input "Existing spec path or command (if known):" AUTOMATION_COMMAND false
fi

prompt_input "Automation notes or blocker:" AUTOMATION_NOTES false

# Step 3: Objective and Description
echo ""
echo -e "${MAGENTA}━━━ Step 3: Test Objective ━━━${NC}"
echo ""

prompt_input "What are you testing? (objective):" OBJECTIVE true
prompt_input "Why is this test important?" WHY_IMPORTANT false

# Step 4: Preconditions
echo ""
echo -e "${MAGENTA}━━━ Step 4: Preconditions ━━━${NC}"
echo ""

echo "Enter preconditions (one per line, press Enter twice when done):"
PRECONDITIONS=""
while true; do
    read -r line
    if [ -z "$line" ]; then
        break
    fi
    PRECONDITIONS="${PRECONDITIONS}- ${line}"$'\n'
done

# Step 5: Test Steps
echo ""
echo -e "${MAGENTA}━━━ Step 5: Test Steps ━━━${NC}"
echo ""

echo "Enter test steps (format: action | expected result)"
echo "Type 'done' when finished"
echo ""

TEST_STEPS=""
STEP_NUM=1

while true; do
    echo -e "${YELLOW}Step $STEP_NUM:${NC}"
    prompt_input "Action:" ACTION false

    if [ "$ACTION" = "done" ] || [ -z "$ACTION" ]; then
        break
    fi

    prompt_input "Expected result:" EXPECTED true

    TEST_STEPS="${TEST_STEPS}${STEP_NUM}. ${ACTION}"$'\n'"   **Expected:** ${EXPECTED}"$'\n'$'\n'
    ((STEP_NUM++))
done

# Step 6: Test Data
echo ""
echo -e "${MAGENTA}━━━ Step 6: Test Data ━━━${NC}"
echo ""

prompt_input "Test data required (e.g., user credentials, sample data):" TEST_DATA false

# Step 7: Figma Design (if UI test)
echo ""
if [ "$TEST_TYPE" = "UI/Visual" ]; then
    echo -e "${MAGENTA}━━━ Step 7: Figma Design Validation ━━━${NC}"
    echo ""

    prompt_input "Figma design URL (if applicable):" FIGMA_URL false
    prompt_input "Visual elements to validate:" VISUAL_CHECKS false
fi

# Step 8: Edge Cases
echo ""
echo -e "${MAGENTA}━━━ Step 8: Additional Info ━━━${NC}"
echo ""

prompt_input "Edge cases or variations to consider:" EDGE_CASES false
prompt_input "Related test cases (IDs):" RELATED_TCS false
prompt_input "Notes or comments:" NOTES false

if [ -z "$PRECONDITIONS" ]; then
    PRECONDITIONS="- [No special preconditions documented]"
fi

if [ -z "$TEST_STEPS" ]; then
    echo -e "${RED}At least one test step is required.${NC}" >&2
    exit 1
fi

if [ -z "$AUTOMATION_COMMAND" ]; then
    AUTOMATION_COMMAND="N/A"
fi

# Generate filename
FILENAME="${TC_ID}.md"
FILENAME="${FILENAME//[^a-zA-Z0-9._-]/}"

OUTPUT_FILE="$OUTPUT_DIR/$FILENAME"

# Generate test case
echo ""
echo -e "${BLUE}Generating test case...${NC}"
echo ""

cat > "$OUTPUT_FILE" << EOF
# ${TC_ID}: ${TC_TITLE}

**Priority:** ${PRIORITY}
**Type:** ${TEST_TYPE}
**Status:** Not Run
**Estimated Time:** ${EST_TIME:-TBD} minutes
**Created:** $(date +%Y-%m-%d)
**Last Updated:** $(date +%Y-%m-%d)
**Automation Target:** ${AUTOMATION_TARGET}
**Automation Status:** ${AUTOMATION_STATUS}
**Automation Command/Spec:** ${AUTOMATION_COMMAND}
**Automation Notes:** ${AUTOMATION_NOTES:-None}

---

## Objective

${OBJECTIVE}

${WHY_IMPORTANT:+**Why this matters:** ${WHY_IMPORTANT}}

---

## Preconditions

${PRECONDITIONS}

---

## Test Steps

${TEST_STEPS}

---

## Test Data

${TEST_DATA:-No specific test data required}

---

EOF

# Add Figma section if UI test
if [ "$TEST_TYPE" = "UI/Visual" ] && [ -n "$FIGMA_URL" ]; then
    cat >> "$OUTPUT_FILE" << EOF
## Visual Validation (Figma)

**Design Reference:** ${FIGMA_URL}

**Elements to validate:**
${VISUAL_CHECKS}

**Verification checklist (viewports: 375px, 768px, 1280px):**
- [ ] Layout matches Figma design
- [ ] Spacing (padding/margins) accurate
- [ ] Typography (font, size, weight, color) correct
- [ ] Colors match design system
- [ ] Component states (hover, active, disabled) implemented
- [ ] Responsive behavior as designed

---

EOF
fi

cat >> "$OUTPUT_FILE" << EOF
## Post-conditions

- [Describe system state after test execution]
- [Any cleanup required]

---

## Edge Cases & Variations

${EDGE_CASES:-Consider boundary values, null inputs, special characters, concurrent users}

---

## Related Test Cases

${RELATED_TCS:-None documented}

---

## Execution History

| Date | Tester | Build | Result | Bug ID | Notes |
|------|--------|-------|--------|--------|-------|
| | | | Not Run | | |

---

## Notes

${NOTES:-None}

EOF

echo -e "${GREEN}Test case generated: ${BLUE}$OUTPUT_FILE${NC}" >&2
echo "$OUTPUT_FILE"
