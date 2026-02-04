#!/bin/bash

SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

CONFIG_PATH="${1:-${SCRIPTPATH}/config-cli.json}"
if [ -n "$1" ] && [ -f "$1" ]; then
  shift
fi

${SCRIPTPATH}/../github-apps-trampoline --cli -c "${CONFIG_PATH}" --verbose "$@"
