#!/bin/bash

SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

CONFIG_PATH="${1:-${SCRIPTPATH}/config.json}"
if [ -n "$1" ] && [ -f "$1" ]; then
  shift
fi

cat "${SCRIPTPATH}/in.txt" | "${SCRIPTPATH}/../github-apps-trampoline" -c "${CONFIG_PATH}" --verbose "$@" get
