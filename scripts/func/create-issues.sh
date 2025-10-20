#!/usr/bin/env bash
set -euo pipefail

# Script to create issue files from function length violations
# Usage: ./scripts/func/create-issues.sh [output_dir]

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
readonly MAX_LINES=50

# Default output directory
OUTPUT_DIR="${1:-ai-docs/func-length-issues}"
ISSUES_DIR="${OUTPUT_DIR}/issues"

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[0;33m'
readonly BLUE='\033[0;36m'
readonly NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}ℹ${NC} $*"
}

log_success() {
    echo -e "${GREEN}✓${NC} $*"
}

log_error() {
    echo -e "${RED}✗${NC} $*" >&2
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $*"
}

# Create output directories
create_directories() {
    log_info "Creating output directories..."
    mkdir -p "${ISSUES_DIR}"
    mkdir -p "${OUTPUT_DIR}/grouped"
}

# Run the function length checker and capture output
run_checker() {
    log_info "Running function length checker..."
    cd "${PROJECT_ROOT}"

    # Run and capture output, ignoring exit code
    local output
    output=$(go run scripts/func/check-function-length.go 2>&1 || true)

    echo "${output}"
}

# Parse checker output and group by file
parse_violations() {
    local output="$1"
    local temp_file="${OUTPUT_DIR}/.violations.tmp"

    # Save output to temp file first
    local raw_output="${OUTPUT_DIR}/.raw_output.tmp"
    echo "${output}" > "${raw_output}"

    # Extract violations line by line
    local file="" linenum="" func="" lines="" excess=""
    while IFS= read -r line; do
        if [[ "${line}" =~ ^📄[[:space:]](.+):([0-9]+)$ ]]; then
            file="${BASH_REMATCH[1]}"
            linenum="${BASH_REMATCH[2]}"
            func=""
            lines=""
            excess=""
        elif [[ "${line}" =~ ^[[:space:]]*Function:[[:space:]](.+)$ ]]; then
            func="${BASH_REMATCH[1]}"
        elif [[ "${line}" =~ ^[[:space:]]*Lines:[[:space:]]([0-9]+)[[:space:]]\(exceeds[[:space:]]limit[[:space:]]by[[:space:]]([0-9]+)\)$ ]]; then
            lines="${BASH_REMATCH[1]}"
            excess="${BASH_REMATCH[2]}"

            if [[ -n "${file}" && -n "${func}" && -n "${lines}" ]]; then
                echo "${file}|${linenum}|${func}|${lines}|${excess}" >> "${temp_file}"
            fi
        fi
    done < "${raw_output}"

    rm -f "${raw_output}"
    echo "${temp_file}"
}

# Group violations by file
group_by_file() {
    local violations_file="$1"
    local grouped_file="${OUTPUT_DIR}/.grouped.tmp"

    # Sort by file, then by line number
    sort -t'|' -k1,1 -k2,2n "${violations_file}" > "${grouped_file}"

    echo "${grouped_file}"
}

# Generate issue markdown for a single file's violations
generate_issue_markdown() {
    local issue_num="$1"
    local file_path="$2"
    shift 2
    local violations=("$@")

    local sanitized_file
    local sanitized_file="${file_path//\//_}"
    sanitized_file="${sanitized_file//:/_}"
    sanitized_file="${sanitized_file// /_}"
    local issue_file="${ISSUES_DIR}/$(printf '%03d' "${issue_num}")-${sanitized_file}.md"
    local total_funcs="${#violations[@]}"

    cat > "${issue_file}" << EOF
# Issue ${issue_num} - Function Length Violations

**File:** \`${file_path}\`
**Total Functions:** ${total_funcs}
**Status:** - [ ] UNRESOLVED

## Summary

This file contains ${total_funcs} function(s) that exceed the 50-line limit. Each function should be refactored into smaller, more focused functions following the Single Responsibility Principle.

## Functions Requiring Refactoring

EOF

    # Add each violation
    local idx=1
    for violation in "${violations[@]}"; do
        IFS='|' read -r _ line func lines excess <<< "${violation}"

        cat >> "${issue_file}" << EOF
### ${idx}. \`${func}\` (Line ${line})

- **Current Length:** ${lines} lines
- **Exceeds Limit By:** ${excess} lines
- **Target:** ≤ ${MAX_LINES} lines

EOF
        ((idx++))
    done

    cat >> "${issue_file}" << EOF

## Refactoring Guidelines

- Break down into smaller, focused functions
- Extract complex logic into helper functions
- Follow Single Responsibility Principle
- Maintain clear function names that describe their purpose

## Resolution Steps

1. **Analyze** each function to identify logical segments
2. **Extract** complex logic into separate helper functions
3. **Refactor** to reduce cyclomatic complexity
4. **Test** thoroughly after refactoring
5. **Verify** with \`make lint\` and \`make test\`

## Project Standards

- Functions must not exceed ${MAX_LINES} lines
- Follow patterns in \`.cursor/rules/go-coding-standards.mdc\`
- Maintain backward compatibility unless in alpha/breaking change phase
- Update tests if function signatures change

---
*Generated from function length analysis - Compozy Code Quality*
EOF

    log_success "Created issue file: ${issue_file}" >&2
    echo "${issue_file}"
}

# Create grouped summary file
create_grouped_summary() {
    local violations_file="$1"
    local grouped_file="${OUTPUT_DIR}/grouped/all-violations.md"

    cat > "${grouped_file}" << 'EOF'
# Function Length Violations - Grouped Summary

This document contains all functions exceeding the 50-line limit, grouped by file.

## Overview

EOF

    # Count total violations and files
    local total_violations
    total_violations=$(wc -l < "${violations_file}")
    local total_files
    total_files=$(cut -d'|' -f1 "${violations_file}" | sort -u | wc -l)

    cat >> "${grouped_file}" << EOF
- **Total Violations:** ${total_violations}
- **Files Affected:** ${total_files}

## Files

EOF

    # Group violations by file
    local current_file=""
    while IFS='|' read -r file line func lines excess; do
        if [[ "${file}" != "${current_file}" ]]; then
            if [[ -n "${current_file}" ]]; then
                echo "" >> "${grouped_file}"
            fi
            current_file="${file}"
            echo "### \`${file}\`" >> "${grouped_file}"
            echo "" >> "${grouped_file}"
        fi

        echo "- **${func}** (line ${line}): ${lines} lines (exceeds by ${excess})" >> "${grouped_file}"
    done < "${violations_file}"

    log_success "Created grouped summary: ${grouped_file}"
}

# Create summary file
create_summary() {
    local issue_files=("$@")
    local summary_file="${OUTPUT_DIR}/_summary.md"
    local total="${#issue_files[@]}"

    cat > "${summary_file}" << EOF
# Function Length Issues - Summary

This folder contains function length violation issues for the Compozy codebase.

## Summary

- **Total Issues:** ${total}
- **Status:** All Unresolved
- **Limit:** ${MAX_LINES} lines per function

## Issues

EOF

    for issue_file in "${issue_files[@]}"; do
        local issue_basename
        issue_basename=$(basename "${issue_file}")

        local issue_num_part
        issue_num_part=${issue_basename%%-*}
        local issue_num="${issue_num_part}"
        if [[ ${issue_num_part} =~ ^[0-9]+$ ]]; then
            # shellcheck disable=SC2004
            issue_num=$((10#${issue_num_part}))
        fi

        local file_path
        file_path=$(grep -m1 '^\*\*File:\*\*' "${issue_file}" | sed 's/^.*`\([^`]*\)`.*/\1/')

        echo "- [ ] [Issue ${issue_num}](issues/${issue_basename}) - ${file_path}" >> "${summary_file}"
    done

    cat >> "${summary_file}" << 'EOF'

## Usage

These issues can be processed with the `solve_issues.go` script:

```bash
go run scripts/issues/solve_issues.go \
  --issues-dir ai-docs/func-length-issues/issues \
  --ide codex \
  --concurrent 4 \
  --batch-size 3
```

Or use the Makefile target:

```bash
make solve-func-length
```

## Resolution Process

1. Each issue file contains functions from a single file that need refactoring
2. Functions should be broken down following SOLID principles
4. Ensure `make lint` and `make test` pass
5. Commit changes with descriptive message

## Guidelines

- **Never** sacrifice code quality for line count
- Extract meaningful helper functions with clear names
- Follow patterns from `.cursor/rules/go-coding-standards.mdc`
- Maintain test coverage when refactoring
- Document complex logic appropriately

---
*Generated on $(date '+%Y-%m-%d %H:%M:%S')*
EOF

    log_success "Created summary file: ${summary_file}"
}

# Main execution
main() {
    log_info "Starting function length issue generation..."
    echo ""

    create_directories

    # Run checker and parse violations
    local checker_output
    checker_output=$(run_checker)

    # Check if any violations found
    if ! echo "${checker_output}" | grep -q "^📄"; then
        log_success "No function length violations found!"
        exit 0
    fi

    # Parse and group violations
    local violations_file
    violations_file=$(parse_violations "${checker_output}")

    # Check if violations file has content
    if [[ ! -s "${violations_file}" ]]; then
        log_success "No violations to process!"
        rm -f "${violations_file}"
        exit 0
    fi

    local grouped_file
    grouped_file=$(group_by_file "${violations_file}")

    # Create issue files grouped by source file
    local issue_num=0
    local current_file=""
    local current_violations=()
    declare -a issue_files

    while IFS='|' read -r file line func lines excess; do
        if [[ "${file}" != "${current_file}" ]]; then
            # Generate issue for previous file if exists
            if [[ -n "${current_file}" ]] && [[ ${#current_violations[@]} -gt 0 ]]; then
                ((issue_num++))
                local issue_file
                issue_file=$(generate_issue_markdown "${issue_num}" "${current_file}" "${current_violations[@]}")
                issue_files+=("${issue_file}")
            fi

            # Start new file
            current_file="${file}"
            current_violations=("${file}|${line}|${func}|${lines}|${excess}")
        else
            current_violations+=("${file}|${line}|${func}|${lines}|${excess}")
        fi
    done < "${grouped_file}"

    # Generate last issue
    if [[ -n "${current_file}" ]] && [[ ${#current_violations[@]} -gt 0 ]]; then
        ((issue_num++))
        local issue_file
        issue_file=$(generate_issue_markdown "${issue_num}" "${current_file}" "${current_violations[@]}")
        issue_files+=("${issue_file}")
    fi

    # Create summary and grouped files only if we have issues
    if [[ ${#issue_files[@]} -gt 0 ]]; then
        create_summary "${issue_files[@]}"
        create_grouped_summary "${violations_file}"
    fi

    # Cleanup temp files
    rm -f "${violations_file}" "${grouped_file}"

    echo ""
    log_success "Generated ${issue_num} issue files in ${ISSUES_DIR}"
    log_info "Summary available at: ${OUTPUT_DIR}/_summary.md"
    echo ""
    echo "Next steps:"
    echo "  1. Review issues: cat ${OUTPUT_DIR}/_summary.md"
    echo "  2. Process with solve_issues.go:"
    echo "     go run scripts/issues/solve_issues.go --issues-dir ${ISSUES_DIR} --ide codex --concurrent 4"
}

# Run main function
main "$@"
