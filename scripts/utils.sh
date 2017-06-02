#!/bin/bash
#
# This file contains helper functions that are used in various other scripts.
# The functions are not intended to be usefull outside of the other bash
# files in the scripts/ directory but may come in handy at some point.

get_branch() {
    # not a good idea to use this to actually checkout git branches
    # this is mostly just good for use in getting the environment from get_environment()

    # TODO remember to turn this back on
    # if we are in a jenkins build then the BRANCH_NAME env var will be set
    if [ -z ${BRANCH_NAME} ]; then
        # one way to get the current branch
        git branch | grep "\*" | cut -d' ' -f 2
    else
        echo ${BRANCH_NAME}
    fi
}

get_build() {
    local BUILD_TIME=$(sh -c "date -u +%m%d%H%M")
    if [[ ${BUILD_NUMBER} == "" ]]; then
        BUILD_NUMBER=local_build
    fi
    echo "${BUILD_TIME}-${BUILD_NUMBER}"
}
