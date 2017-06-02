#!/bin/bash
#
# Pushes docker images to AWS ECR
# The image number used is the most recently uploaed image number + 1
#
# The 'latest' tag is created for edge cases when we just want to deploy most recent
# build without knowing the latest image number (not ideal for everday use)

set -e

. scripts/utils.sh

NAME=ecs-task-tracker
IMAGE_TAG=${1}
if [[ ${IMAGE_TAG} == "" ]]; then
    IMAGE_TAG=test
fi

docker push tskinn12/${NAME}:latest
docker push tskinn12/${NAME}:${IMAGE_TAG}
