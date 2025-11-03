#!/bin/bash
# Script to create GitHub issue for k-core decomposition fix

# Check if GitHub CLI is installed
if command -v gh &> /dev/null; then
    echo "Using GitHub CLI to create issue..."
    gh issue create \
        --title "Fix K-Core Decomposition Level Update Logic" \
        --body-file ISSUE_K_CORE_FIX.md \
        --label "bug" \
        --label "algorithms"
    echo "Issue created successfully!"
    exit 0
fi

# Check if GH_TOKEN is set
if [ -z "$GH_TOKEN" ]; then
    echo "GitHub CLI not found and GH_TOKEN not set."
    echo ""
    echo "Option 1: Install GitHub CLI and authenticate:"
    echo "  brew install gh  # macOS"
    echo "  gh auth login"
    echo "  Then run this script again"
    echo ""
    echo "Option 2: Set GH_TOKEN environment variable:"
    echo "  export GH_TOKEN=your_github_token"
    echo "  Then run this script again"
    echo ""
    echo "Option 3: Create issue manually via web:"
    echo "  1. Go to: https://github.com/mundrapranay/silhouette-db/issues/new"
    echo "  2. Copy the content from ISSUE_K_CORE_FIX.md"
    echo "  3. Paste it into the issue body"
    echo ""
    exit 1
fi

# Use GitHub API with token
echo "Creating issue using GitHub API..."

REPO="mundrapranay/silhouette-db"
TITLE="Fix K-Core Decomposition Level Update Logic"
BODY=$(cat ISSUE_K_CORE_FIX.md | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')

# Create issue via API
curl -X POST \
  -H "Authorization: token $GH_TOKEN" \
  -H "Accept: application/vnd.github.v3+json" \
  "https://api.github.com/repos/$REPO/issues" \
  -d "{\"title\":\"$TITLE\",\"body\":\"$BODY\",\"labels\":[\"bug\",\"algorithms\"]}"

echo ""
echo "Issue created successfully!"

