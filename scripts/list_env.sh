#!/bin/bash
# list-environments.sh
# Shows all running Docker Compose projects and their branches

echo "Running Docker Compose Environments:"
echo "===================================="

# Get all docker compose projects
# Check if any containers are running first
if ! docker ps -q > /dev/null 2>&1; then
    projects=""
else
    # Use direct format without table headers to avoid parsing issues
    projects=$(docker ps --format "{{.Label \"com.docker.compose.project\"}}" 2> /dev/null | grep -v "^$" | sort -u || true)
fi

if [ -z "$projects" ]; then
    echo "No active Docker Compose projects found."
    exit 0
fi

for project in $projects; do
    if [[ $project == *"_compozy" ]]; then
        branch=${project%_compozy}
        echo ""
        echo "Branch: $branch"
        echo "Project: $project"

        # Check if env file exists
        env_file=".env.${branch}"
        if [ -f "$env_file" ]; then
            echo "Env file: $env_file"
            # Source the env file to get ports
            source "$env_file"
            echo "URLs:"
            echo "  - PostgreSQL (App): localhost:${DB_PORT}"
            echo "  - Redis: localhost:${REDIS_PORT}"
            echo "  - Temporal UI: http://localhost:${TEMPORAL_UI_PORT}"
            echo "  - MCP Proxy: http://localhost:${MCP_PROXY_PORT}"
            [ -n "$PROMETHEUS_PORT" ] && echo "  - Prometheus: http://localhost:${PROMETHEUS_PORT}"
            [ -n "$GRAFANA_PORT" ] && echo "  - Grafana: http://localhost:${GRAFANA_PORT}"
        fi

        echo "Services:"
        docker ps --filter "label=com.docker.compose.project=$project" \
            --format "  - {{.Names}} ({{.Ports}})"
    fi
done

echo ""
echo "===================================="
echo "Total projects: $(echo "$projects" | wc -w)"
