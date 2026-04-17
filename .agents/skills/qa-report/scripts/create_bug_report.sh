#!/bin/bash

# Bug Report Generator
# Create structured, reproducible bug reports using the unified issue template
# Usage: ./create_bug_report.sh [qa-output-path/issues]

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${RED}╔══════════════════════════════════════════════════╗${NC}"
echo -e "${RED}║           Bug Report Generator                   ║${NC}"
echo -e "${RED}╚══════════════════════════════════════════════════╝${NC}"
echo ""

# Resolve output directory
OUTPUT_DIR="${1:-.}"
mkdir -p "$OUTPUT_DIR"

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

# Bug ID
BUG_ID="BUG-$(date +%Y%m%d%H%M%S)"
echo -e "${YELLOW}Auto-generated Bug ID: $BUG_ID${NC}"
echo ""

# Basic Info
prompt_input "Bug title (clear, specific — e.g., '[Login] Password reset email not sent'):" BUG_TITLE true

echo ""
echo "Severity:"
echo "1) Critical - System crash, data loss, security breach"
echo "2) High - Major feature broken, no workaround"
echo "3) Medium - Feature partially broken, workaround exists"
echo "4) Low - Cosmetic, minor inconvenience"
echo ""

prompt_input "Select severity (1-4):" SEVERITY_NUM true

case $SEVERITY_NUM in
    1) SEVERITY="Critical" ;;
    2) SEVERITY="High" ;;
    3) SEVERITY="Medium" ;;
    4) SEVERITY="Low" ;;
    *) SEVERITY="Medium" ;;
esac

echo ""
echo "Priority:"
echo "1) P0 - Blocks release"
echo "2) P1 - Fix before release"
echo "3) P2 - Fix in next release"
echo "4) P3 - Fix when possible"
echo ""

prompt_input "Select priority (1-4):" PRIORITY_NUM true

case $PRIORITY_NUM in
    1) PRIORITY="P0" ;;
    2) PRIORITY="P1" ;;
    3) PRIORITY="P2" ;;
    4) PRIORITY="P3" ;;
    *) PRIORITY="P2" ;;
esac

echo ""
echo "Bug type:"
echo "1) Functional"
echo "2) UI"
echo "3) Performance"
echo "4) Security"
echo "5) Data"
echo "6) Crash"
echo ""

prompt_input "Select type (1-6):" TYPE_NUM true

case $TYPE_NUM in
    1) BUG_TYPE="Functional" ;;
    2) BUG_TYPE="UI" ;;
    3) BUG_TYPE="Performance" ;;
    4) BUG_TYPE="Security" ;;
    5) BUG_TYPE="Data" ;;
    6) BUG_TYPE="Crash" ;;
    *) BUG_TYPE="Functional" ;;
esac

# Environment
echo ""
echo -e "${MAGENTA}━━━ Environment Details ━━━${NC}"
echo ""

prompt_input "Build/Version (e.g., v2.5.0 or commit hash):" BUILD true
prompt_input "Operating System (e.g., macOS 14, Ubuntu 22.04):" OS false
prompt_input "Browser & Version (e.g., Chrome 120) — skip if not Web UI:" BROWSER false
prompt_input "URL or page where bug occurs:" URL false

# Bug Description
echo ""
echo -e "${MAGENTA}━━━ Bug Description ━━━${NC}"
echo ""

prompt_input "Brief description of the issue:" DESCRIPTION true

# Reproduction Steps
echo ""
echo -e "${MAGENTA}━━━ Steps to Reproduce ━━━${NC}"
echo ""

echo "Enter reproduction steps (one per line, press Enter twice when done):"
REPRO_STEPS=""
STEP_NUM=1
while true; do
    read -r line
    if [ -z "$line" ]; then
        break
    fi
    REPRO_STEPS="${REPRO_STEPS}${STEP_NUM}. ${line}"$'\n'
    ((STEP_NUM++))
done

# Expected vs Actual
echo ""
prompt_input "Expected behavior:" EXPECTED true
prompt_input "Actual behavior:" ACTUAL true

# Impact
echo ""
echo -e "${MAGENTA}━━━ Impact ━━━${NC}"
echo ""

prompt_input "Frequency (Always/Sometimes/Rarely):" FREQUENCY false
prompt_input "Users affected (all/subset/specific role):" USER_IMPACT false
prompt_input "Workaround available? (describe if yes):" WORKAROUND false

# Related
echo ""
echo -e "${MAGENTA}━━━ Related Items ━━━${NC}"
echo ""

prompt_input "Related test case ID (e.g., TC-FUNC-001):" TEST_CASE false
prompt_input "Figma design link (if UI bug):" FIGMA_LINK false

# Automation follow-up
echo ""
echo -e "${MAGENTA}━━━ Automation Follow-up ━━━${NC}"
echo ""

echo "Automation Status:"
echo "1) Added - automation was added or updated"
echo "2) Pending - automation should be added later"
echo "3) Blocked - automation is expected but prerequisites are missing"
echo "4) N/A - no automation follow-up applies"
echo ""

prompt_input "Select automation status (1-4):" AUTOMATION_STATUS_NUM true

case $AUTOMATION_STATUS_NUM in
    1) AUTOMATION_STATUS="Added"; AUTOMATION_REQUIRED="Yes" ;;
    2) AUTOMATION_STATUS="Pending"; AUTOMATION_REQUIRED="Yes" ;;
    3) AUTOMATION_STATUS="Blocked"; AUTOMATION_REQUIRED="Yes" ;;
    4) AUTOMATION_STATUS="N/A"; AUTOMATION_REQUIRED="No" ;;
    *) AUTOMATION_STATUS="N/A"; AUTOMATION_REQUIRED="No" ;;
esac

if [ "$AUTOMATION_STATUS" = "N/A" ]; then
    AUTOMATION_SPEC="N/A"
else
    prompt_input "Automation spec path or command:" AUTOMATION_SPEC false
fi

prompt_input "Automation notes or blocker:" AUTOMATION_NOTES false

if [ -z "$AUTOMATION_SPEC" ]; then
    AUTOMATION_SPEC="N/A"
fi

RELATED_ITEMS="- None"
if [ -n "$TEST_CASE" ] || [ -n "$FIGMA_LINK" ]; then
    RELATED_ITEMS=""
    if [ -n "$TEST_CASE" ]; then
        RELATED_ITEMS="${RELATED_ITEMS}- Test Case: ${TEST_CASE}"$'\n'
    fi
    if [ -n "$FIGMA_LINK" ]; then
        RELATED_ITEMS="${RELATED_ITEMS}- Figma Design: ${FIGMA_LINK}"$'\n'
    fi
fi

FILENAME="${BUG_ID}.md"
OUTPUT_FILE="$OUTPUT_DIR/$FILENAME"

echo ""
echo -e "${BLUE}Generating bug report...${NC}"
echo ""

cat > "$OUTPUT_FILE" << EOF
# ${BUG_ID}: ${BUG_TITLE}

**Severity:** ${SEVERITY}
**Priority:** ${PRIORITY}
**Type:** ${BUG_TYPE}
**Status:** Open

## Environment

- **Build:** ${BUILD}
- **OS:** ${OS:-N/A}
- **Browser:** ${BROWSER:-N/A}
- **URL:** ${URL:-N/A}

## Summary

${DESCRIPTION}

## Reproduction

\`\`\`bash
${REPRO_STEPS}
\`\`\`

Observed before the fix:

- ${ACTUAL}

## Expected

${EXPECTED}

## Root cause

[To be filled after investigation]

## Fix

[To be filled after fix is applied]

## Verification

- [ ] Narrow reproduction rerun
- [ ] Broader regression or full gate rerun

## Impact

- **Users Affected:** ${USER_IMPACT:-Unknown}
- **Frequency:** ${FREQUENCY:-Unknown}
- **Workaround:** ${WORKAROUND:-None}

## Automation Follow-up

- **Required:** ${AUTOMATION_REQUIRED}
- **Status:** ${AUTOMATION_STATUS}
- **Spec / Command:** ${AUTOMATION_SPEC}
- **Notes:** ${AUTOMATION_NOTES:-None}

## Related

${RELATED_ITEMS}
EOF

echo -e "${GREEN}Bug report generated: ${BLUE}$OUTPUT_FILE${NC}" >&2
echo "$OUTPUT_FILE"
