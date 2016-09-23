#!/bin/bash

function check_dep {
  command -v $1 >/dev/null 2>&1 || {
    echo >&2 "The required dependency program '$1' is not installed. Aborting."
    exit 1
  }
}

DEPS="curl git go strip zip"

echo "Checking for the presence of the following dependencies: ${DEPS}"

for DEP in $DEPS
do
  check_dep $DEP && echo "Found ${DEP}"
done
echo "All dependencies were successfully found, we're good to go!"
