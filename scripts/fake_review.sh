#!/usr/bin/env bash
set -euo pipefail

WAIT_VALUE="${WAIT:-${LRC_FAKE_REVIEW_WAIT:-30s}}"
export LRC_FAKE_REVIEW_WAIT="$WAIT_VALUE"

TMP_REPO_PATH="${TMP_REPO:-/tmp/lrc-fake-review-repo}"

if [[ -d "$TMP_REPO_PATH" ]]; then
	rm -rf "$TMP_REPO_PATH"
fi

mkdir -p "$TMP_REPO_PATH"
cd "$TMP_REPO_PATH"

git init -q
git config user.name "lrc fake reviewer"
git config user.email "lrc-fake@example.local"

cat > README.md <<'EOF'
# lrc fake review sandbox

This repository is auto-generated for fake E2E review testing.
EOF
git add README.md
LRC_SKIP_REVIEW=1 git commit -q --no-verify -m "chore: initial fake repo state"

seed="$(date +%s)-$RANDOM"
mkdir -p src
cat > src/fake_${seed}.txt <<EOF
fake review seed: ${seed}
timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

echo "run: ${seed}" >> README.md
git add README.md src/fake_${seed}.txt

# Ensure no stale hook state from setup phase leaks into fake review UX.
rm -f .git/livereview_state \
	.git/livereview_state.lock \
	.git/livereview_commit_message \
	.git/__LRC_COMMIT_MESSAGE_FILE__ \
	.git/livereview_push_request \
	.git/livereview_initial_message.* 2>/dev/null || true

echo "[lrc fake-review] mode=fake wait=${LRC_FAKE_REVIEW_WAIT} repo=${TMP_REPO_PATH}"
echo "[lrc fake-review] created and staged fake changes:"
git status --short

exec lrc review --staged "$@"
