#!/bin/bash
set -e

echo "🔍 Running local checks (same as CI)..."
echo ""

# Set USE_ACT=1 to run with act (e.g., USE_ACT=1 make check)
if [ "$USE_ACT" = "1" ] && command -v act &> /dev/null; then
  echo "🎬 Running GitHub Actions PR checks locally with act..."
  # Only run PR checks workflow, not deploy workflow
  act pull_request -j test -W .github/workflows/ci.yml --container-architecture linux/amd64
else
  if [ "$USE_ACT" = "1" ]; then
    echo -e "${YELLOW}⚠️  USE_ACT=1 but 'act' is not installed. Install with: brew install act${NC}"
    echo -e "📦 Falling back to manual checks..."
    echo ""
  fi
  
  # Run Go tests (fast, independent)
  echo "📦 Running Go tests..."
  make test
  
  # Run E2E tests (slower, depends on built assets)
  echo "🎭 Running E2E tests..."
  make test-e2e
  
  echo ""
  echo "💡 Tip: Run with act for exact CI simulation: USE_ACT=1 make check"
  echo "⚡ Speed: Go tests took ~10-20s, E2E tests took ~30-60s"
fi

echo ""
echo "✅ All checks passed! Safe to push."
