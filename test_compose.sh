#!/bin/bash
# test_compose.sh — Verifies docker-compose brings up Redis + App correctly
# Run from the chat/ directory: bash test_compose.sh

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "=== Docker Compose Integration Test ==="
echo ""

# 1. Start services
echo "[1/5] Starting services..."
docker-compose up -d --build
echo ""

# 2. Wait for app to be ready
echo "[2/5] Waiting for app health endpoint..."
MAX_WAIT=30
WAITED=0
until curl -sf http://localhost:8080/health > /dev/null 2>&1; do
    sleep 1
    WAITED=$((WAITED + 1))
    if [ $WAITED -ge $MAX_WAIT ]; then
        echo -e "${RED}FAIL: App did not become healthy within ${MAX_WAIT}s${NC}"
        docker-compose logs app
        docker-compose down
        exit 1
    fi
done
echo -e "${GREEN}  App is healthy (took ${WAITED}s)${NC}"
echo ""

# 3. Verify Redis is reachable from host
echo "[3/5] Checking Redis connectivity..."
if docker-compose exec redis redis-cli ping | grep -q "PONG"; then
    echo -e "${GREEN}  Redis PONG received${NC}"
else
    echo -e "${RED}FAIL: Redis not responding${NC}"
    docker-compose down
    exit 1
fi
echo ""

# 4. Verify app can talk to Redis (check logs for "Redis ready")
echo "[4/5] Verifying app connected to Redis..."
if docker-compose logs app 2>&1 | grep -q "Redis ready"; then
    echo -e "${GREEN}  App confirmed Redis connection${NC}"
else
    echo -e "${RED}FAIL: App did not confirm Redis connection${NC}"
    docker-compose logs app
    docker-compose down
    exit 1
fi
echo ""

# 5. Cleanup
echo "[5/5] Tearing down..."
docker-compose down
echo ""

echo -e "${GREEN}=== All checks passed ===${NC}"
