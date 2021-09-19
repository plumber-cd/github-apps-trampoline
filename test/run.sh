#!/bin/bash

SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cat ${SCRIPTPATH}/in.txt | ${SCRIPTPATH}/../github-apps-trampoline -c ${SCRIPTPATH}/config.json --verbose get
