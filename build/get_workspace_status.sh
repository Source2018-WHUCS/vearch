#!/bin/bash
# get buildinfo from workspace with git info

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

GIT_REVISION=$(git rev-parse HEAD 2> /dev/null)
if [[  $? == 0 ]]; then
    git diff-index --quiet HEAD
    if [[  $? != 0 ]]; then
        GIT_REVISION=${GIT_REVISION}"-dirty"
    fi
else
    GIT_REVISION=unknown
fi

if [[ -z ${VERSION} ]]; then
    RELEASE_TAG=$(git describe --match '[0-9]*\.[0-9]*\.[0-9]*' --exact-match 2> /dev/null || echo "")
    VERSION="${GIT_REVISION}"
    if [[ -n "${RELEASE_TAG}" ]]; then
      VERSION="${RELEASE_TAG}"
    fi
fi

echo AppVersion       "${VERSION}"
echo GitRevision      "${GIT_REVISION}"
echo User             "$(whoami)"
echo BuildTime        $(date "+%Y-%m-%d %H:%M:%S")
echo Type             "snapshot"
