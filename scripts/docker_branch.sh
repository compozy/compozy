#!/bin/bash
# docker-branch-manager.sh
# Complete Docker environment isolation for different git branches

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get current branch name and sanitize it
BRANCH_NAME=$(git branch --show-current | sed 's/[^a-zA-Z0-9]/_/g')
if [ -z "$BRANCH_NAME" ]; then
    echo -e "${RED}Error: Not in a git repository or detached HEAD state${NC}"
    exit 1
fi

# Configuration
BASE_ENV_FILE=".env"
BRANCH_ENV_FILE=".env.${BRANCH_NAME}"
COMPOSE_PROJECT="${BRANCH_NAME}_compozy"
COMPOSE_FILE="cluster/docker-compose.yml"

# Function to generate a consistent port offset from branch name
get_port_offset() {
    # Generate a number between 1-99 from branch name
    echo $((($(echo "$1" | cksum | cut -d' ' -f1) % 99) + 1))
}

# Function to check if port is available
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t > /dev/null 2>&1; then
        return 1
    else
        return 0
    fi
}

# Function to find next available port
find_available_port() {
    local base_port=$1
    local max_attempts=100
    local port=$base_port

    for i in $(seq 1 $max_attempts); do
        if check_port $port; then
            echo $port
            return 0
        fi
        port=$((port + 1))
    done

    echo -e "${RED}Error: Could not find available port starting from $base_port${NC}" >&2
    return 1
}

# Create branch-specific environment file
create_env_file() {
    echo -e "${GREEN}Creating environment for branch: ${YELLOW}$BRANCH_NAME${NC}"

    # Copy base environment file
    if [ -f "$BASE_ENV_FILE" ]; then
        cp "$BASE_ENV_FILE" "$BRANCH_ENV_FILE"
    elif [ -f "cluster/$BASE_ENV_FILE" ]; then
        # Try cluster directory if not in root
        cp "cluster/$BASE_ENV_FILE" "$BRANCH_ENV_FILE"
    else
        echo -e "${RED}Error: Base .env file not found in root or cluster directory${NC}"
        exit 1
    fi

    # Get port offset
    PORT_OFFSET=$(get_port_offset "$BRANCH_NAME")

    # Update ports with available ones
    echo -e "${GREEN}Finding available ports...${NC}"

    # Application database
    DB_PORT=$(find_available_port $((5432 + PORT_OFFSET)))
    sed -i.bak "s/^DB_PORT=.*/DB_PORT=$DB_PORT/" "$BRANCH_ENV_FILE"
    echo "  - PostgreSQL (App): $DB_PORT"

    # Redis
    REDIS_PORT=$(find_available_port $((6379 + PORT_OFFSET)))
    sed -i.bak "s/^REDIS_PORT=.*/REDIS_PORT=$REDIS_PORT/" "$BRANCH_ENV_FILE"
    echo "  - Redis: $REDIS_PORT"

    # Temporal database
    TEMPORAL_DB_PORT=$(find_available_port $((5433 + PORT_OFFSET)))
    sed -i.bak "s/^TEMPORAL_DB_PORT=.*/TEMPORAL_DB_PORT=$TEMPORAL_DB_PORT/" "$BRANCH_ENV_FILE"
    echo "  - PostgreSQL (Temporal): $TEMPORAL_DB_PORT"

    # Test database
    TEST_DB_PORT=$(find_available_port $((5434 + PORT_OFFSET)))
    sed -i.bak "s/^TEST_DB_PORT=.*/TEST_DB_PORT=$TEST_DB_PORT/" "$BRANCH_ENV_FILE"
    echo "  - PostgreSQL (Test): $TEST_DB_PORT"

    # Temporal
    TEMPORAL_PORT=$(find_available_port $((7233 + PORT_OFFSET)))
    sed -i.bak "s/^TEMPORAL_PORT=.*/TEMPORAL_PORT=$TEMPORAL_PORT/" "$BRANCH_ENV_FILE"
    echo "  - Temporal: $TEMPORAL_PORT"

    # Temporal UI
    TEMPORAL_UI_PORT=$(find_available_port $((8080 + PORT_OFFSET)))
    sed -i.bak "s/^TEMPORAL_UI_PORT=.*/TEMPORAL_UI_PORT=$TEMPORAL_UI_PORT/" "$BRANCH_ENV_FILE"
    echo "  - Temporal UI: $TEMPORAL_UI_PORT"

    # MCP Proxy
    MCP_PROXY_PORT=$(find_available_port $((8081 + PORT_OFFSET)))
    sed -i.bak "s/^MCP_PROXY_PORT=.*/MCP_PROXY_PORT=$MCP_PROXY_PORT/" "$BRANCH_ENV_FILE"
    sed -i.bak "s|^MCP_PROXY_URL=.*|MCP_PROXY_URL=http://localhost:$MCP_PROXY_PORT|" "$BRANCH_ENV_FILE"
    sed -i.bak "s|^MCP_PROXY_BASE_URL=.*|MCP_PROXY_BASE_URL=http://localhost:$MCP_PROXY_PORT|" "$BRANCH_ENV_FILE"
    echo "  - MCP Proxy: $MCP_PROXY_PORT"

    # Prometheus
    PROMETHEUS_PORT=$(find_available_port $((9090 + PORT_OFFSET)))
    if ! grep -q "^PROMETHEUS_PORT=" "$BRANCH_ENV_FILE"; then
        echo "PROMETHEUS_PORT=$PROMETHEUS_PORT" >> "$BRANCH_ENV_FILE"
    else
        sed -i.bak "s/^PROMETHEUS_PORT=.*/PROMETHEUS_PORT=$PROMETHEUS_PORT/" "$BRANCH_ENV_FILE"
    fi
    echo "  - Prometheus: $PROMETHEUS_PORT"

    # Grafana
    GRAFANA_PORT=$(find_available_port $((3000 + PORT_OFFSET)))
    if ! grep -q "^GRAFANA_PORT=" "$BRANCH_ENV_FILE"; then
        echo "GRAFANA_PORT=$GRAFANA_PORT" >> "$BRANCH_ENV_FILE"
    else
        sed -i.bak "s/^GRAFANA_PORT=.*/GRAFANA_PORT=$GRAFANA_PORT/" "$BRANCH_ENV_FILE"
    fi
    echo "  - Grafana: $GRAFANA_PORT"

    # Update database names to include branch suffix
    sed -i.bak "s/^DB_NAME=.*/DB_NAME=compozy_${BRANCH_NAME}/" "$BRANCH_ENV_FILE"
    sed -i.bak "s/^TEST_DB_NAME=.*/TEST_DB_NAME=compozy_test_${BRANCH_NAME}/" "$BRANCH_ENV_FILE"
    sed -i.bak "s/^TEMPORAL_DB_NAME=.*/TEMPORAL_DB_NAME=temporal_${BRANCH_NAME}/" "$BRANCH_ENV_FILE"

    # Add compose project name
    if ! grep -q "^COMPOSE_PROJECT_NAME=" "$BRANCH_ENV_FILE"; then
        echo "COMPOSE_PROJECT_NAME=$COMPOSE_PROJECT" >> "$BRANCH_ENV_FILE"
    else
        sed -i.bak "s/^COMPOSE_PROJECT_NAME=.*/COMPOSE_PROJECT_NAME=$COMPOSE_PROJECT/" "$BRANCH_ENV_FILE"
    fi

    # Clean up backup files
    rm -f "${BRANCH_ENV_FILE}.bak"

    echo -e "${GREEN}Environment file created: ${YELLOW}$BRANCH_ENV_FILE${NC}"
}

# Docker Compose wrapper
docker_compose() {
    docker-compose -f "$COMPOSE_FILE" --env-file "$BRANCH_ENV_FILE" -p "$COMPOSE_PROJECT" "$@"
}

# Show current configuration
show_config() {
    echo -e "${GREEN}Current branch configuration:${NC}"
    echo -e "  Branch: ${YELLOW}$BRANCH_NAME${NC}"
    echo -e "  Project: ${YELLOW}$COMPOSE_PROJECT${NC}"
    echo -e "  Env file: ${YELLOW}$BRANCH_ENV_FILE${NC}"

    if [ -f "$BRANCH_ENV_FILE" ]; then
        echo -e "\n${GREEN}Service URLs:${NC}"
        source "$BRANCH_ENV_FILE"
        echo "  - PostgreSQL (App): localhost:${DB_PORT}"
        echo "  - PostgreSQL (Test): localhost:${TEST_DB_PORT}"
        echo "  - PostgreSQL (Temporal): localhost:${TEMPORAL_DB_PORT}"
        echo "  - Redis: localhost:${REDIS_PORT}"
        echo "  - Temporal: localhost:${TEMPORAL_PORT}"
        echo "  - Temporal UI: http://localhost:${TEMPORAL_UI_PORT}"
        echo "  - MCP Proxy: http://localhost:${MCP_PROXY_PORT}"
        echo "  - Prometheus: http://localhost:${PROMETHEUS_PORT:-9090}"
        echo "  - Grafana: http://localhost:${GRAFANA_PORT:-3000}"
    else
        echo -e "${YELLOW}Environment not initialized. Run '$0 init' first.${NC}"
    fi
}

# Main command handler
case "${1:-help}" in
    init)
        if [ ! -f "$COMPOSE_FILE" ]; then
            echo -e "${RED}Error: Docker Compose file not found at $COMPOSE_FILE${NC}"
            exit 1
        fi
        create_env_file
        echo -e "\n${GREEN}To start services, run: ${YELLOW}$0 up${NC}"
        ;;

    up)
        if [ ! -f "$COMPOSE_FILE" ]; then
            echo -e "${RED}Error: Docker Compose file not found at $COMPOSE_FILE${NC}"
            exit 1
        fi
        if [ ! -f "$BRANCH_ENV_FILE" ]; then
            create_env_file
        fi
        shift
        docker_compose up -d "$@"
        echo -e "\n${GREEN}Services started!${NC}"
        show_config
        ;;

    down)
        shift
        docker_compose down "$@"
        ;;

    stop)
        shift
        docker_compose stop "$@"
        ;;

    restart)
        shift
        docker_compose restart "$@"
        ;;

    logs)
        shift
        docker_compose logs "$@"
        ;;

    ps)
        shift
        docker_compose ps "$@"
        ;;

    exec)
        shift
        docker_compose exec "$@"
        ;;

    config)
        show_config
        ;;

    clean)
        echo -e "${YELLOW}This will remove all containers, volumes, and networks for branch: $BRANCH_NAME${NC}"
        read -p "Are you sure? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            docker_compose down -v --remove-orphans
            rm -f "$BRANCH_ENV_FILE"
            echo -e "${GREEN}Cleaned up branch environment${NC}"
        fi
        ;;

    reset-db)
        echo -e "${YELLOW}This will reset all databases for branch: $BRANCH_NAME${NC}"
        echo -e "${RED}WARNING: All database data will be lost!${NC}"
        read -p "Are you sure? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            echo -e "${GREEN}Resetting Docker services...${NC}"
            docker_compose down --volumes
            docker_compose up -d
            echo -e "${GREEN}Waiting for databases to be ready...${NC}"
            sleep 10
            echo -e "${GREEN}Running database migrations...${NC}"
            if [ -f "$BRANCH_ENV_FILE" ]; then
                source "$BRANCH_ENV_FILE"
                GOOSE_DBSTRING="postgres://${DB_USER:-postgres}:${DB_PASSWORD}@${DB_HOST:-localhost}:${DB_PORT}/${DB_NAME}?sslmode=disable"
                GOOSE_DRIVER=postgres GOOSE_DBSTRING=${GOOSE_DBSTRING} goose -dir ./engine/infra/store/migrations up
                echo -e "${GREEN}Database reset and migrations complete!${NC}"
            else
                echo -e "${YELLOW}Environment file not found. Please run migrations manually.${NC}"
            fi
        fi
        ;;

    migrate-status)
        if [ -f "$BRANCH_ENV_FILE" ]; then
            source "$BRANCH_ENV_FILE"
            echo -e "${GREEN}Checking migration status for branch: ${YELLOW}$BRANCH_NAME${NC}"
            GOOSE_DBSTRING="postgres://${DB_USER:-postgres}:${DB_PASSWORD}@${DB_HOST:-localhost}:${DB_PORT}/${DB_NAME}?sslmode=disable"
            GOOSE_DRIVER=postgres GOOSE_DBSTRING=${GOOSE_DBSTRING} goose -dir ./engine/infra/store/migrations status
        else
            echo -e "${RED}Error: Environment file not found. Run '$0 init' first.${NC}"
            exit 1
        fi
        ;;

    migrate-up)
        if [ -f "$BRANCH_ENV_FILE" ]; then
            source "$BRANCH_ENV_FILE"
            echo -e "${GREEN}Running migrations up for branch: ${YELLOW}$BRANCH_NAME${NC}"
            GOOSE_DBSTRING="postgres://${DB_USER:-postgres}:${DB_PASSWORD}@${DB_HOST:-localhost}:${DB_PORT}/${DB_NAME}?sslmode=disable"
            GOOSE_DRIVER=postgres GOOSE_DBSTRING=${GOOSE_DBSTRING} goose -dir ./engine/infra/store/migrations up
            echo -e "${GREEN}Migrations completed!${NC}"
        else
            echo -e "${RED}Error: Environment file not found. Run '$0 init' first.${NC}"
            exit 1
        fi
        ;;

    migrate-down)
        if [ -f "$BRANCH_ENV_FILE" ]; then
            source "$BRANCH_ENV_FILE"
            echo -e "${YELLOW}Rolling back one migration for branch: $BRANCH_NAME${NC}"
            read -p "Are you sure? (y/N) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                GOOSE_DBSTRING="postgres://${DB_USER:-postgres}:${DB_PASSWORD}@${DB_HOST:-localhost}:${DB_PORT}/${DB_NAME}?sslmode=disable"
                GOOSE_DRIVER=postgres GOOSE_DBSTRING=${GOOSE_DBSTRING} goose -dir ./engine/infra/store/migrations down
                echo -e "${GREEN}Migration rolled back!${NC}"
            fi
        else
            echo -e "${RED}Error: Environment file not found. Run '$0 init' first.${NC}"
            exit 1
        fi
        ;;

    migrate-create)
        if [ -z "$2" ]; then
            echo -e "${RED}Error: Migration name required${NC}"
            echo "Usage: $0 migrate-create <migration_name>"
            exit 1
        fi
        migration_name="$2"
        echo -e "${GREEN}Creating new migration: ${YELLOW}$migration_name${NC}"
        GOOSE_DRIVER=postgres goose -dir ./engine/infra/store/migrations create "$migration_name" sql
        echo -e "${GREEN}Migration created!${NC}"
        ;;

    migrate-validate)
        echo -e "${GREEN}Validating migrations...${NC}"
        GOOSE_DRIVER=postgres goose -dir ./engine/infra/store/migrations validate
        echo -e "${GREEN}Validation complete!${NC}"
        ;;

    migrate-reset)
        if [ -f "$BRANCH_ENV_FILE" ]; then
            source "$BRANCH_ENV_FILE"
            echo -e "${YELLOW}This will reset all migrations for branch: $BRANCH_NAME${NC}"
            echo -e "${RED}WARNING: All database data will be lost!${NC}"
            read -p "Are you sure? (y/N) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                GOOSE_DBSTRING="postgres://${DB_USER:-postgres}:${DB_PASSWORD}@${DB_HOST:-localhost}:${DB_PORT}/${DB_NAME}?sslmode=disable"
                GOOSE_DRIVER=postgres GOOSE_DBSTRING=${GOOSE_DBSTRING} goose -dir ./engine/infra/store/migrations reset
                echo -e "${GREEN}Migrations reset!${NC}"
            fi
        else
            echo -e "${RED}Error: Environment file not found. Run '$0 init' first.${NC}"
            exit 1
        fi
        ;;

    switch)
        # Show all available branch environments
        echo -e "${GREEN}Available branch environments:${NC}"
        for env_file in .env.*; do
            if [[ $env_file =~ \.env\.([a-zA-Z0-9_]+)$ ]]; then
                branch="${BASH_REMATCH[1]}"
                if [ "$branch" = "$BRANCH_NAME" ]; then
                    echo -e "  * ${YELLOW}$branch${NC} (current)"
                else
                    echo "    $branch"
                fi
            fi
        done
        ;;

    help | *)
        echo -e "${GREEN}Docker Branch Manager${NC}"
        echo -e "Manages isolated Docker environments per git branch\n"
        echo -e "${YELLOW}Usage:${NC} $0 <command> [options]\n"
        echo -e "${YELLOW}Commands:${NC}"
        echo "  init       Initialize environment for current branch"
        echo "  up         Start all services (runs init if needed)"
        echo "  down       Stop and remove all containers"
        echo "  stop       Stop all services"
        echo "  restart    Restart all services"
        echo "  logs       View logs (use -f for follow)"
        echo "  ps         List running containers"
        echo "  exec       Execute command in container"
        echo "  config     Show current configuration"
        echo "  clean      Remove all resources for current branch"
        echo "  reset-db   Reset all databases (removes all data!)"
        echo "  migrate-status   Check migration status"
        echo "  migrate-up     Run migrations up"
        echo "  migrate-down   Roll back one migration"
        echo "  migrate-create Create a new migration"
        echo "  migrate-validate Validate migrations"
        echo "  migrate-reset Reset all migrations"
        echo "  switch     List all branch environments"
        echo "  help       Show this help message"
        echo -e "\n${YELLOW}Examples:${NC}"
        echo "  $0 init                    # Initialize current branch"
        echo "  $0 up                      # Start all services"
        echo "  $0 logs temporal -f        # Follow temporal logs"
        echo "  $0 exec app-postgresql psql -U postgres  # Connect to database"
        echo "  $0 clean                   # Clean up current branch"
        ;;
esac
