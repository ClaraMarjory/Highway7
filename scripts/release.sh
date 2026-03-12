#!/bin/bash
# Highway7 Release Script
# Usage: bash scripts/release.sh v0.2.1 "release notes"
set -e
TAG=$1; NOTE=$2
[ -z "$TAG" ] && { echo "Usage: bash scripts/release.sh vX.Y.Z \"notes\""; exit 1; }
[ -z "$NOTE" ] && NOTE="$TAG"

TOKEN=${H7_TOKEN:?"set H7_TOKEN first"}
REPO=ClaraMarjory/Highway7

echo "=== Build ==="
make build
./highway -version

echo "=== Git Push ==="
git add -A
git commit -m "$TAG: $NOTE" || echo "nothing to commit"
git push

echo "=== Tag ==="
git tag -d $TAG 2>/dev/null || true
git push origin :refs/tags/$TAG 2>/dev/null || true
git tag $TAG
git push origin $TAG

echo "=== Release ==="
# 删旧release如果存在
OLD_ID=$(curl -s "https://api.github.com/repos/$REPO/releases/tags/$TAG" -H "Authorization: token $TOKEN" | grep '"id"' | head -1 | grep -o '[0-9]*')
[ -n "$OLD_ID" ] && curl -s -X DELETE "https://api.github.com/repos/$REPO/releases/$OLD_ID" -H "Authorization: token $TOKEN"

# 创建新release
RESP=$(curl -s -X POST "https://api.github.com/repos/$REPO/releases" \
  -H "Authorization: token $TOKEN" -H "Content-Type: application/json" \
  -d "{\"tag_name\":\"$TAG\",\"name\":\"$TAG\",\"body\":\"$NOTE\"}")
RELEASE_ID=$(echo "$RESP" | grep '"id"' | head -1 | grep -o '[0-9]*')
echo "Release ID: $RELEASE_ID"

# 上传binary
echo "=== Upload Binary ==="
DL=$(curl -s -X POST "https://uploads.github.com/repos/$REPO/releases/$RELEASE_ID/assets?name=highway-linux-amd64" \
  -H "Authorization: token $TOKEN" -H "Content-Type: application/octet-stream" \
  --data-binary @./highway | grep -o '"browser_download_url":"[^"]*"' | cut -d'"' -f4)
echo "Download: $DL"

echo ""
echo "=== Done ==="
echo "HK-IEPL deploy:"
echo "  bash <(curl -sL https://raw.githubusercontent.com/$REPO/main/scripts/update.sh)"
echo "  or fresh install:"
echo "  bash <(curl -sL https://raw.githubusercontent.com/$REPO/main/scripts/install.sh)"
