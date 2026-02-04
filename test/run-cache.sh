#!/bin/bash

set -euo pipefail

SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

CONFIG_PATH="${1:-${SCRIPTPATH}/config-current-owner.json}"
if [ -n "${1:-}" ] && [ -f "$1" ]; then
  shift
fi

CACHE_DIR="${SCRIPTPATH}/.cache"
LOG_FILE="${SCRIPTPATH}/cache-test.log"

rm -rf "${CACHE_DIR}"
rm -f "${LOG_FILE}"

${SCRIPTPATH}/run.sh "${CONFIG_PATH}" --cache --cache-dir "${CACHE_DIR}" --log-file "${LOG_FILE}" "$@"
${SCRIPTPATH}/run.sh "${CONFIG_PATH}" --cache --cache-dir "${CACHE_DIR}" --log-file "${LOG_FILE}" "$@"

MISS_COUNT=$(rg -c "cache miss" "${LOG_FILE}")
HIT_COUNT=$(rg -c "cache hit" "${LOG_FILE}")

if [ "${MISS_COUNT}" -lt 1 ]; then
  echo "Expected at least 1 cache miss, got ${MISS_COUNT}"
  exit 1
fi

if [ "${HIT_COUNT}" -lt 1 ]; then
  echo "Expected at least 1 cache hit, got ${HIT_COUNT}"
  exit 1
fi

echo "Cache test ok: misses=${MISS_COUNT} hits=${HIT_COUNT}"
