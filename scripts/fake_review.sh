#!/usr/bin/env bash
set -euo pipefail

WAIT_VALUE="${WAIT:-${LRC_FAKE_REVIEW_WAIT:-30s}}"
export LRC_FAKE_REVIEW_WAIT="$WAIT_VALUE"

echo "[lrc fake-review] mode=fake wait=${LRC_FAKE_REVIEW_WAIT}"
exec lrc review "$@"
