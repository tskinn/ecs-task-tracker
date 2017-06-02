#!/bin/bash

set -e

. scripts/utils.sh

NAME=ecs-task-tracker
IMAGE_TAG=${1}
if [[ ${IMAGE_TAG} == "" ]]; then
    IMAGE_TAG=test
fi

echo "ENVIRONMENT : ${ENVIRONMENT}"
echo "NAME    : ${NAME}"
echo "IMAGE_TAG: ${IMAGE_TAG}"

docker build -t tskinn12/${NAME}:latest .
docker build -t tskinn12/${NAME}:${IMAGE_TAG} .
