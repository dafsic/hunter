#!/usr/bin/env bash

RELEASE=${RELEASE:-$2}
PREVIOUS_RELEASE=${PREVIOUS_RELEASE:-$1}

## Ensure Correct Usage
if [[ -z "${PREVIOUS_RELEASE}" || -z "${RELEASE}" ]]; then
  echo Usage:
  echo ./scripts/release-notes.sh v3.0.0 v3.1.0
  echo or
  echo PREVIOUS_RELEASE=v3.0.0
  echo RELEASE=v3.1.0
  echo ./scripts/release-notes.sh
  exit 1
fi

## validate git tags
for tag in $RELEASE $PREVIOUS_RELEASE; do
  OK=$(git tag -l ${tag} | wc -l)
  if [[ "$OK" == "0" ]]; then
    echo ${tag} is not a valid release version
    exit 1
  fi
done


## Generate CHANGELOG from git log
CHANGELOG=$(git log --no-merges --pretty=format:'- %s %H (%aN)' ${PREVIOUS_RELEASE}..${RELEASE})
if [[ ! $? -eq 0 ]]; then
  echo "Error creating changelog"
  echo "try running \`git log --no-merges --pretty=format:'- %s %H (%aN)' ${PREVIOUS_RELEASE}..${RELEASE}\`"
  exit 1
fi

## guess at MAJOR / MINOR / PATCH versions
MAJOR=$(echo ${RELEASE} | sed 's/^v//' | cut -f1 -d.)
MINOR=$(echo ${RELEASE} | sed 's/^v//' | cut -f2 -d.)
PATCH=$(echo ${RELEASE} | sed 's/^v//' | cut -f3 -d.)

## Print release notes to stdout
cat <<EOF
## ${RELEASE}

ProductionManagement ${RELEASE} is a feature release. This release, we focused on <insert focal point>. Users are encouraged to upgrade for the best experience.

## Notable Changes

- Add list of
- notable changes here

## User's Guide

The [install guide](http://192.168.3.40/mgs/cloud/platform/production_management) will get you going from there. 

## What's Next

- ${MAJOR}.${MINOR}.$(expr ${PATCH} + 1) will contain only bug fixes.
- ${MAJOR}.$(expr ${MINOR} + 1).${PATCH} is the next feature release. This release will focus on ...

## Changelog

${CHANGELOG}
EOF