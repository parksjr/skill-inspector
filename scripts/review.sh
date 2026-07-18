#!/bin/bash
# Review pipeline — run from the review agent worktree
# Usage: scripts/review.sh <agent-branch> [--deep]
#
# 1. Fetches latest main
# 2. Checks out agent branch
# 3. Runs automated gates (go vet, go test, gofmt)
# 4. Produces diff against main
# 5. Outputs review-ready summary for liza-code-review

set -euo pipefail

BRANCH="${1:-}"
MODE="${2:-standard}"

if [ -z "$BRANCH" ]; then
    echo "Usage: scripts/review.sh <agent-branch> [--deep]"
    echo "  agent-branch: e.g. agents/security, agents/cli-tui"
    echo "  --deep:       deep review mode (default: standard)"
    exit 1
fi

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Review Pipeline: $BRANCH"
echo "Mode: $MODE"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 1. Fetch and verify branch exists
echo ""
echo "▶ Fetching latest..."
git fetch origin main 2>/dev/null || git fetch origin master 2>/dev/null || true

if ! git rev-parse --verify "origin/$BRANCH" >/dev/null 2>&1; then
    if ! git rev-parse --verify "$BRANCH" >/dev/null 2>&1; then
        echo -e "${RED}✗ Branch '$BRANCH' not found${NC}"
        exit 1
    fi
    BASE="$BRANCH"
else
    BASE="origin/$BRANCH"
fi

# Determine main branch
if git rev-parse --verify origin/main >/dev/null 2>&1; then
    MAIN="origin/main"
elif git rev-parse --verify origin/master >/dev/null 2>&1; then
    MAIN="origin/master"
else
    MAIN="HEAD~1"
fi

echo "  Agent branch: $BASE"
echo "  Base:         $MAIN"

# 2. Automated gates
echo ""
echo "━━━ Automated Gates ━━━"

echo "▶ go vet ./..."
if go vet ./... 2>&1; then
    echo -e "  ${GREEN}✓ go vet passed${NC}"
else
    echo -e "  ${RED}✗ go vet FAILED${NC}"
    GATES_FAILED=1
fi

echo "▶ gofmt -d ."
unformatted=$(gofmt -l . 2>/dev/null)
if [ -z "$unformatted" ]; then
    echo -e "  ${GREEN}✓ gofmt passed${NC}"
else
    echo -e "  ${RED}✗ gofmt FAILED — unformatted:${NC}"
    echo "$unformatted"
    GATES_FAILED=1
fi

echo "▶ go test ./..."
if go test ./... 2>&1; then
    echo -e "  ${GREEN}✓ go test passed${NC}"
else
    echo -e "  ${RED}✗ go test FAILED${NC}"
    GATES_FAILED=1
fi

# 3. Diff stats
echo ""
echo "━━━ Diff Summary ━━━"
DIFF_FILES=$(git diff --name-only "$MAIN"..."$BASE" 2>/dev/null | wc -l | tr -d ' ')
DIFF_LINES=$(git diff --stat "$MAIN"..."$BASE" 2>/dev/null | tail -1)
echo "  Files changed: $DIFF_FILES"
echo "  $DIFF_LINES"

# List changed files
echo ""
echo "Changed files:"
git diff --name-only "$MAIN"..."$BASE" 2>/dev/null | while read -r f; do
    echo "  $f"
done

# 4. Produce full diff for review
echo ""
echo "━━━ Full Diff (for liza-code-review) ━━━"
echo "────────────────────────────────────────"
git diff "$MAIN"..."$BASE" 2>/dev/null
echo "────────────────────────────────────────"

# 5. Summary
echo ""
echo "━━━ Review Verdict ━━━"
if [ "${GATES_FAILED:-0}" -eq 1 ]; then
    echo -e "${RED}BLOCKED: Automated gates failed${NC}"
    exit 1
fi

if [ "$MODE" = "--deep" ]; then
    echo "Deep review required — run liza-code-review Deep on the diff above"
else
    echo "Standard review — run liza-code-review Standard on the diff above"
fi

echo ""
echo "Next: Apply liza-code-review protocol to the diff above."
echo "  Review skill: ~/.agents/skills/liza-code-review/SKILL.md"
