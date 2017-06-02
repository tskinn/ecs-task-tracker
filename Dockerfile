FROM alpine:3.5
MAINTAINER Taylor <tskinn12@gmail.com>

RUN apk --update upgrade && \
    apk add curl ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/cache/apk/*

EXPOSE 80

ADD ["bin/linux/ecs-task-tracker", "/opt/ecs-task-tracker"]
CMD ["/opt/KMS_NAME"]
