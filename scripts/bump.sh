#!/usr/bin/env bash
# Usage: scripts/bump.sh <patch|minor|major>
set -euo pipefail

PART=${1:?usage: bump.sh <patch|minor|major>}
REPO="https://github.com/baniol/docs-mcp"
CHANGELOG="CHANGELOG.md"

last=$(git tag --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || true)
last=${last:-v0.0.0}

major=$(echo "$last" | cut -d. -f1 | tr -d v)
minor=$(echo "$last" | cut -d. -f2)
patch=$(echo "$last" | cut -d. -f3)

case "$PART" in
  major) next="v$((major+1)).0.0" ;;
  minor) next="v${major}.$((minor+1)).0" ;;
  patch) next="v${major}.${minor}.$((patch+1))" ;;
  *)     echo "error: unknown part '$PART'"; exit 1 ;;
esac

echo "${last} → ${next}"

# Update CHANGELOG.md
today=$(date +%Y-%m-%d)

if ! grep -q '^## \[Unreleased\]' "$CHANGELOG"; then
  echo "error: no [Unreleased] section in $CHANGELOG"
  exit 1
fi

# Replace [Unreleased] header with versioned header, insert new empty [Unreleased]
sed -i '' "s|^## \[Unreleased\]|## [Unreleased]\n\n## [${next}] - ${today}|" "$CHANGELOG"

# Update comparison links at the bottom
sed -i '' "s|\[Unreleased\]:.*|[Unreleased]: ${REPO}/compare/${next}...HEAD\n[${next}]: ${REPO}/compare/${last}...${next}|" "$CHANGELOG"

# Commit, tag
git add "$CHANGELOG"
git commit -m "chore: release ${next}"
git tag -a "${next}" -m "Release ${next}"

echo "Tagged ${next}. Push with: git push && git push origin ${next}"
