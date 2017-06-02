#!/bin/bash
#
# Build the Dockerfile at the root directory of the project.
# If consul is running on the local host and has relevant key/values
# inject them into the Dockerfile. Otherwise look for an env file
# and inject it into the Dockerfile.
#
# It is assumed that the largest image tag number is also the
# most recently created image uploaded. So that number + 1 is used
# as the tag for image built by the file
#
# The 'latest' tag is created for edge cases when we just want to deploy most recent
# build without knowing the latest image number (not ideal for everday use)

set -e

. scripts/utils.sh

NAME=ecs-task-tracker
IMAGE_TAG=${1}
if [[ ${IMAGE_TAG == "" }]]; then
    IMAGE_TAG=test
fi
FILE=Dockerfile

echo "ENVIRONMENT : ${ENVIRONMENT}"
echo "NAME    : ${NAME}"
echo "IMAGE_TAG: ${IMAGE_TAG}"

sed -i "s|NAME|${NAME}|g" ${FILE}

cat ${FILE}

docker build -t tskinn/${NAME}:latest .
docker build -t tskinn/${NAME}:${IMAGE_TAG} .
