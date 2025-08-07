#!/usr/bin/env bash
# publish-npm.sh - NPM Package Publishing Script
# This script handles the publishing of NPM packages from the tools directory
# with retry logic, proper error handling, and comprehensive logging.

set -euo pipefail

# Configuration
readonly SCRIPT_NAME="publish-npm"
readonly DEFAULT_TIMEOUT=1800  # 30 minutes
readonly DEFAULT_MAX_RETRIES=3
readonly DEFAULT_RETRY_DELAY=5
readonly DEFAULT_TOOLS_DIR="${GITHUB_WORKSPACE:-$(pwd)}/tools"
readonly NPM_REGISTRY="https://registry.npmjs.org/"

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly PURPLE='\033[0;35m'
readonly NC='\033[0m' # No Color

# Global variables
TOOLS_DIR="${TOOLS_DIR:-$DEFAULT_TOOLS_DIR}"
TIMEOUT="${NPM_TIMEOUT:-$DEFAULT_TIMEOUT}"
MAX_RETRIES="${NPM_MAX_RETRIES:-$DEFAULT_MAX_RETRIES}"
DRY_RUN=false
VERBOSE=false
CI_OUTPUT=false

# Arrays for tracking results
declare -a published_packages=()
declare -a failed_packages=()
declare -a skipped_packages=()

# Logging functions
log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    if [[ "$CI_OUTPUT" == "true" ]]; then
        echo "[$timestamp] [$level] $message"
    else
        case "$level" in
            ERROR)   echo -e "${RED}❌ $message${NC}" ;;
            SUCCESS) echo -e "${GREEN}✅ $message${NC}" ;;
            WARNING) echo -e "${YELLOW}⚠️  $message${NC}" ;;
            INFO)    echo -e "${BLUE}ℹ️  $message${NC}" ;;
            DEBUG)   [[ "$VERBOSE" == "true" ]] && echo -e "${PURPLE}🔍 $message${NC}" ;;
            *)       echo "$message" ;;
        esac
    fi
}

# Show usage information
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Publish NPM packages from the tools directory with retry logic and error handling.

OPTIONS:
    -h, --help              Show this help message
    -d, --dry-run          Perform a dry run without actually publishing
    -v, --verbose          Enable verbose output
    -c, --ci-output        Enable CI-friendly output format
    -t, --timeout SECONDS  Set timeout for npm operations (default: $DEFAULT_TIMEOUT)
    -r, --retries COUNT    Set maximum retry attempts (default: $DEFAULT_MAX_RETRIES)
    --tools-dir PATH       Set tools directory path (default: $DEFAULT_TOOLS_DIR)

ENVIRONMENT VARIABLES:
    NPM_TOKEN              Required: NPM authentication token
    NODE_AUTH_TOKEN        Optional: Alternative npm token (used if NPM_TOKEN not set)
    TOOLS_DIR              Optional: Tools directory path
    NPM_TIMEOUT            Optional: Timeout in seconds
    NPM_MAX_RETRIES        Optional: Maximum retry attempts

EXAMPLES:
    # Basic usage
    $0

    # Dry run with verbose output
    $0 --dry-run --verbose

    # Custom configuration
    $0 --timeout 900 --retries 5 --tools-dir ./packages

    # CI mode
    $0 --ci-output

EOF
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -d|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -c|--ci-output)
                CI_OUTPUT=true
                shift
                ;;
            -t|--timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            -r|--retries)
                MAX_RETRIES="$2"
                shift 2
                ;;
            --tools-dir)
                TOOLS_DIR="$2"
                shift 2
                ;;
            *)
                log "ERROR" "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
}

# Validate environment and prerequisites
validate_environment() {
    log "INFO" "Validating environment..."
    
    # Check for required commands
    local required_commands=("npm")
    for cmd in "${required_commands[@]}"; do
        if ! command -v "$cmd" &> /dev/null; then
            log "ERROR" "Required command not found: $cmd"
            return 1
        fi
    done
    
    # Check for timeout command (different on Linux vs macOS)
    if ! command -v "timeout" &> /dev/null && ! command -v "gtimeout" &> /dev/null; then
        log "ERROR" "Timeout command not found (install coreutils with: brew install coreutils)"
        return 1
    fi
    
    # Check for realpath command (different on Linux vs macOS)
    if ! command -v "realpath" &> /dev/null && ! command -v "grealpath" &> /dev/null; then
        log "ERROR" "Realpath command not found (install coreutils with: brew install coreutils)"
        return 1
    fi
    
    # Validate NPM authentication
    if [[ -z "${NPM_TOKEN:-}" && -z "${NODE_AUTH_TOKEN:-}" ]]; then
        log "ERROR" "NPM authentication token is required (NPM_TOKEN or NODE_AUTH_TOKEN)"
        return 1
    fi
    
    # Validate tools directory
    if [[ ! -d "$TOOLS_DIR" ]]; then
        log "ERROR" "Tools directory not found: $TOOLS_DIR"
        return 1
    fi
    
    # Resolve absolute path for tools directory (use grealpath on macOS if available)
    if command -v "realpath" &> /dev/null; then
        TOOLS_DIR=$(realpath "$TOOLS_DIR")
    elif command -v "grealpath" &> /dev/null; then
        TOOLS_DIR=$(grealpath "$TOOLS_DIR")
    else
        # Fallback to basic path resolution
        TOOLS_DIR=$(cd "$TOOLS_DIR" && pwd)
    fi
    log "DEBUG" "Using tools directory: $TOOLS_DIR"
    
    # Validate numeric arguments
    if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]] || [[ "$TIMEOUT" -le 0 ]]; then
        log "ERROR" "Invalid timeout value: $TIMEOUT (must be positive integer)"
        return 1
    fi
    
    if ! [[ "$MAX_RETRIES" =~ ^[0-9]+$ ]] || [[ "$MAX_RETRIES" -le 0 ]]; then
        log "ERROR" "Invalid max retries value: $MAX_RETRIES (must be positive integer)"
        return 1
    fi
    
    log "SUCCESS" "Environment validation passed"
    return 0
}

# Check if a directory contains a valid NPM package
is_npm_package() {
    local package_dir="$1"
    local package_json="$package_dir/package.json"
    
    if [[ ! -f "$package_json" ]]; then
        log "DEBUG" "No package.json found in: $package_dir"
        return 1
    fi
    
    # Validate package.json syntax using node/jq instead of npm list
    if ! node -e "JSON.parse(require('fs').readFileSync('$package_json', 'utf8'))" &> /dev/null; then
        log "DEBUG" "Invalid JSON in package.json: $package_dir"
        return 1
    fi
    
    # Check if package has required fields for publishing
    local name
    name=$(node -e "console.log(JSON.parse(require('fs').readFileSync('$package_json', 'utf8')).name || '')" 2>/dev/null)
    if [[ -z "$name" ]]; then
        log "DEBUG" "No name field in package.json: $package_dir"
        return 1
    fi
    
    # Check if package is marked as private
    local is_private
    is_private=$(node -e "console.log(JSON.parse(require('fs').readFileSync('$package_json', 'utf8')).private || false)" 2>/dev/null)
    if [[ "$is_private" == "true" ]]; then
        log "DEBUG" "Package marked as private: $package_dir"
        return 1
    fi
    
    log "DEBUG" "Valid NPM package found: $package_dir (name: $name)"
    return 0
}

# Publish a single NPM package with retry logic
publish_package() {
    local package_dir="$1"
    local package_name
    package_name=$(basename "$package_dir")
    local attempt=1
    local retry_delay=$DEFAULT_RETRY_DELAY
    
    log "INFO" "Publishing NPM package: $package_name (from: $package_dir)"
    
    # Validate package
    if ! is_npm_package "$package_dir"; then
        log "WARNING" "Skipping $package_name: not a valid NPM package"
        skipped_packages+=("$package_name")
        return 0
    fi
    
    # Change to package directory for npm operations
    pushd "$package_dir" > /dev/null || {
        log "ERROR" "Failed to change to directory: $package_dir"
        return 1
    }
    
    # Retry loop
    while [[ $attempt -le $MAX_RETRIES ]]; do
        log "INFO" "Attempt $attempt of $MAX_RETRIES for $package_name"
        
        # Prepare npm command with appropriate timeout command
        local timeout_cmd="timeout"
        if ! command -v "timeout" &> /dev/null && command -v "gtimeout" &> /dev/null; then
            timeout_cmd="gtimeout"
        fi
        
        local npm_cmd=(
            "$timeout_cmd" "$TIMEOUT"
            "npm" "publish"
            "--access" "public"
            "--registry" "$NPM_REGISTRY"
        )
        
        # Add dry-run flag if requested
        if [[ "$DRY_RUN" == "true" ]]; then
            npm_cmd+=("--dry-run")
        fi
        
        # Set up environment for npm authentication
        local npm_env=()
        if [[ -n "${NPM_TOKEN:-}" ]]; then
            npm_env+=("NPM_TOKEN=$NPM_TOKEN")
        fi
        if [[ -n "${NODE_AUTH_TOKEN:-}" ]]; then
            npm_env+=("NODE_AUTH_TOKEN=$NODE_AUTH_TOKEN")
        elif [[ -n "${NPM_TOKEN:-}" ]]; then
            # Use NPM_TOKEN as NODE_AUTH_TOKEN for GitHub Actions compatibility
            npm_env+=("NODE_AUTH_TOKEN=$NPM_TOKEN")
        fi
        
        # Execute npm publish
        if env "${npm_env[@]}" "${npm_cmd[@]}" 2>&1; then
            local action="published"
            [[ "$DRY_RUN" == "true" ]] && action="validated (dry-run)"
            
            log "SUCCESS" "Successfully $action package: $package_name"
            published_packages+=("$package_name")
            popd > /dev/null
            return 0
        else
            local exit_code=$?
            log "ERROR" "Attempt $attempt failed for $package_name (exit code: $exit_code)"
            
            # Check if this is a timeout
            if [[ $exit_code -eq 124 ]]; then
                log "WARNING" "Operation timed out after ${TIMEOUT}s"
            fi
            
            # Check if we should retry
            if [[ $attempt -lt $MAX_RETRIES ]]; then
                log "INFO" "Waiting ${retry_delay}s before retry..."
                sleep "$retry_delay"
                retry_delay=$((retry_delay * 2))  # Exponential backoff
            fi
        fi
        
        attempt=$((attempt + 1))
    done
    
    log "ERROR" "Failed to publish $package_name after $MAX_RETRIES attempts"
    failed_packages+=("$package_name")
    popd > /dev/null
    return 1
}

# Main publishing workflow
main_publish() {
    log "INFO" "Starting NPM package publishing workflow"
    log "INFO" "Tools directory: $TOOLS_DIR"
    log "INFO" "Timeout: ${TIMEOUT}s, Max retries: $MAX_RETRIES"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log "INFO" "DRY RUN MODE - No packages will be actually published"
    fi
    
    # Find all potential NPM packages
    local package_dirs=()
    while IFS= read -r -d '' dir; do
        if [[ -d "$dir" && "$dir" != "$TOOLS_DIR" ]]; then
            package_dirs+=("$dir")
        fi
    done < <(find "$TOOLS_DIR" -maxdepth 1 -type d -print0)
    
    if [[ ${#package_dirs[@]} -eq 0 ]]; then
        log "WARNING" "No directories found in tools directory"
        return 0
    fi
    
    log "INFO" "Found ${#package_dirs[@]} potential package directories"
    
    # Process each package directory
    local failed_count=0
    for dir in "${package_dirs[@]}"; do
        if ! publish_package "$dir"; then
            failed_count=$((failed_count + 1))
        fi
    done
    
    # Return failure if any packages failed
    [[ $failed_count -eq 0 ]]
}

# Generate comprehensive summary report
generate_report() {
    local total_processed=$((${#published_packages[@]} + ${#failed_packages[@]} + ${#skipped_packages[@]}))
    
    echo
    log "INFO" "📊 Publishing Summary Report"
    echo "============================================="
    log "INFO" "Total directories processed: $total_processed"
    
    if [[ ${#published_packages[@]} -gt 0 ]]; then
        local action="published"
        [[ "$DRY_RUN" == "true" ]] && action="validated (dry-run)"
        
        log "SUCCESS" "Successfully $action: ${#published_packages[@]} packages"
        for package in "${published_packages[@]}"; do
            echo "   - $package"
        done
    fi
    
    if [[ ${#skipped_packages[@]} -gt 0 ]]; then
        log "WARNING" "Skipped: ${#skipped_packages[@]} packages"
        for package in "${skipped_packages[@]}"; do
            echo "   - $package"
        done
    fi
    
    if [[ ${#failed_packages[@]} -gt 0 ]]; then
        log "ERROR" "Failed: ${#failed_packages[@]} packages"
        for package in "${failed_packages[@]}"; do
            echo "   - $package"
        done
        echo "============================================="
        return 1
    fi
    
    echo "============================================="
    local final_message="All NPM packages processed successfully!"
    [[ "$DRY_RUN" == "true" ]] && final_message="Dry run completed successfully!"
    
    log "SUCCESS" "🎉 $final_message"
    return 0
}

# Cleanup and signal handlers
cleanup() {
    local exit_code=$?
    if [[ $exit_code -ne 0 ]]; then
        log "ERROR" "Script terminated with exit code: $exit_code"
    fi
    exit $exit_code
}

# Set up signal handlers
trap cleanup EXIT
trap 'log "ERROR" "Script interrupted by user"; exit 130' INT TERM

# Main execution
main() {
    log "INFO" "Starting $SCRIPT_NAME script"
    
    # Parse command line arguments
    parse_arguments "$@"
    
    # Validate environment and prerequisites
    if ! validate_environment; then
        log "ERROR" "Environment validation failed"
        return 1
    fi
    
    # Execute main publishing workflow
    if ! main_publish; then
        log "ERROR" "Publishing workflow failed"
        generate_report
        return 1
    fi
    
    # Generate final report
    if ! generate_report; then
        return 1
    fi
    
    log "SUCCESS" "NPM publishing completed successfully"
    return 0
}

# Execute main function with all arguments
main "$@"
