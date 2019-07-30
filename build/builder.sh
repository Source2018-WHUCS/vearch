#!/bin/bash
# This script builds and link stamps the output
set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILDPATH=${1:?"path to build"}
BUILDINFO_PACKAGE=${2:?"version go package"}
BUILDOUT=${3:?"output path"}
BUILDINFO=${BUILDINFO:-""}
VERBOSE=${VERBOSE:-"0"}
V=""
if [[ "${VERBOSE}" == "1" ]];then
    V="-x"
    set -x
fi

GOOS=${GOOS:-linux}
GOARCH=${GOARCH:-amd64}
GOBINARY=${GOBINARY:-go}
GOPKG="$GOPATH/pkg"
GOBUILDFLAGS=${GOBUILDFLAGS:-""}
GCFLAGS=${GCFLAGS:-}
STATIC=${STATIC:-""}
LDFLAGS="-extldflags \"-lm -lstdc++ -static\""
if [[ "${STATIC}" !=  "1" ]];then
    LDFLAGS=""
    export CGO_ENABLED=1
else
    export CGO_ENABLED=0
fi

if [[ -z ${BUILDINFO} ]];then
    BUILDINFO=$(mktemp)
    ${ROOT}/build/get_workspace_status.sh > ${BUILDINFO}
fi

LD_BUILDFLAGS=""
while read line; do
    read SYMBOL VALUE < <(echo $line)
    LD_BUILDFLAGS=${LD_BUILDFLAGS}" -X '${BUILDINFO_PACKAGE}.${SYMBOL}=${VALUE}'"
done < "${BUILDINFO}"

# go build
echo GOOS=${GOOS} GOARCH=${GOARCH} ${GOBINARY} build ${V} ${GOBUILDFLAGS:-"-tags=jsoniter" "${GOBUILDFLAGS}"} ${GCFLAGS:+-gcflags "${GCFLAGS}"} -o ${BUILDOUT} \
            -pkgdir=${GOPKG}/${GOOS}_${GOARCH} -ldflags "${LDFLAGS}${LD_BUILDFLAGS}" "${BUILDPATH}"

time GOOS=${GOOS} GOARCH=${GOARCH} ${GOBINARY} build ${V} ${GOBUILDFLAGS:-"-tags=jsoniter" "${GOBUILDFLAGS}"} ${GCFLAGS:+-gcflags "${GCFLAGS}"} -o ${BUILDOUT} \
       -pkgdir=${GOPKG}/${GOOS}_${GOARCH} -ldflags "${LDFLAGS}${LD_BUILDFLAGS}" "${BUILDPATH}"
