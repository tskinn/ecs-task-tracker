#!/bin/bash

set -e

. scripts/utils.sh

# build binaries for mac if needed
if [[ -z ${GOOS} ]]; then GOOS=linux; fi
if [[ -z ${GO_VERSION} ]]; then GO_VERSION=1.8; fi
NAME=ecs-task-tracker
BUILD=$(get_build)
BRANCH_NAME=$(get_branch)

echo "Building binary:"
echo "  KMS name:    ${NAME}"
echo "  Build:       ${BUILD}"
echo "  Branch:      ${BRANCH_NAME}"
echo "  GO version:  ${GO_VERSION}"
docker run --rm -i \
       -v ${HOME}/.ssh:/root/.ssh \
       -v ${HOME}/.gitconfig:/root/.gitconfig \
       -v ${PWD}:/go/src/github.com/tskinn/${NAME} \
       -w /go/src/github.com/tskinn/${NAME} \
       golang:${GO_VERSION} \
       sh -c "cd src && go get -d && \
      GOOS=${GOOS} CGO_ENABLED=0 go build -a -ldflags=\"-s\" \
      -o ../bin/${GOOS}/${NAME} -ldflags \"-X main.BUILD=${BUILD}\" ."

if [ $? -eq 0 ]; then
    echo "${GOOS} binary successfully built at bin/${GOOS}/${NAME}"
fi
