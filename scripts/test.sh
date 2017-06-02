#!/bin/bash

set -e

. scripts/utils.sh

# build binaries for mac if needed
if [[ -z ${GOOS} ]]; then GOOS=linux; fi
if [[ -z ${GO_VERSION} ]]; then GO_VERSION=1.7; fi
NAME=ecs-task-tracker
BUILD=$(get_build)
BRANCH_NAME=$(get_branch)

echo "Building binary:"
echo "  Name:    ${NAME}"
echo "  Build:       ${BUILD}"
echo "  Branch:      ${BRANCH_NAME}"
echo "  GO version:  ${GO_VERSION}"
docker run --rm -i \
       -v ${HOME}/.ssh:/root/.ssh \
       -v ${HOME}/.gitconfig:/root/.gitconfig \
       -v ${PWD}:/go/src/github.com/tskinn/${NAME} \
       -w /go/src/github.com/tskinn/${NAME} \
       golang:${GO_VERSION} \
       sh -c "cd src/utils && go get -d -t && \
      GOOS=${GOOS} CGO_ENABLED=0 go test -covermode=count"

if [ $? -eq 0 ]; then
    echo "successfully tested!"
fi

