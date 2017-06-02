# ecs-task-tracker

## Purpose

The ecs-task-tracker was created in order to keep load balancers up-to-date with the everchanging location of tasks running on AWS ECS. It is one piece in a service discovery system for ecs which relies on [traefik](https://traefik.io/)


## Flow of stuff
ECS emmits cloud watch events when a task changes state. For example when a task is created and running an event will be emitted that says the desired status of the task is "RUNNING" and its current status is "RUNNING". There is other information about the task and its containers in the event message like the host port that the container is listening on.

An SNS topic is provided as a target for these events. This SNS topic then sends those events to an http subscriber which happens to be this application. ecs-task-tracker takes the events does one of three things: 

- Adds the privateIP and port of the container to dynamodb
- Deletes the privateIP and port of the container from dynamodb
- Throws the event away and doesn't do anything at all

```
ECS Cluster ---->> TaskEvent ---->> SNS ---->> http://ecs-task-tracker.mydomain.com/event
 /\ /\                                                                          ||
 || ||                                                                          ||
 || ||                                                                          \/
Traefik  <<----	<<----	<<----	<<----	<<----	<<----  <<----  <<----	DynamoDB traefik-table
 /\ /\
 || ||
 || ||
Internet
 ```

## What is stored in DynamoDB?

Since the DynamoDB table is consumed by [traefik](https://traefik.io/) instances, the data stored in dynamodb is almost the same structure of the structs that [traefik](https://traefik.io/) uses to route requests. See traefiks [types](https://github.com/containous/traefik/blob/master/types/types.go).

There are basically two types of items stored in the DynamoDB table: backends and frontends. [Traefik](https://traefik.io/) needs both in order to serve network requests. The frontend specifies rules governing what incoming traffic should go to which backend. The backend keeps track of the network addresses to send traffic to as well as how to load balance to those addresses.

### Backends
Here is an example of one of the DynamoDB items:
```
{
  "backend": {
    "servers": {
      "10.11.11.100:32894": {
        "url": "http://10.11.11.100:32894",
        "weight": 0
      },
      "10.11.11.110:32864": {
        "url": "http://10.11.11.110:32864",
        "weight": 0
      },
      "10.11.11.242:32867": {
        "url": "http://10.11.11.242:32867",
        "weight": 0
      }
    }
  },
  "id": "test__backend",
  "name": "test",
  "version": 4
}
```

- "backend" is just a Backend struct marshalled to dynamodbattribute.
- "id" is the unique identifier usually made unique by taking the name + "__backend"
- "name" is used as the name of the backend (traefik stores a map[string]Backend to keep track of its backends. The "name" is the string in the map[string]Backend)
- "version" is used as a primitive optimistic locking method

NOTE: ecs-task-tracker does not touch anything in the "backend" except the "servers" attribute for the time being. This is so if someone wanted to add a circuit breaker, rate limiter, maximum connections or healthcheck, they could do so and not worry about it being altered in the future.

Here is the same item but in DynamoDB JSON:

```
{
  "backend": {
    "M": {
      "servers": {
        "M": {
          "10.11.11.100:32894": {
            "M": {
              "url": {
                "S": "http://10.11.11.100:32894"
              },
              "weight": {
                "N": "0"
              }
            }
          },
          "10.11.11.110:32864": {
            "M": {
              "url": {
                "S": "http://10.11.11.110:32864"
              },
              "weight": {
                "N": "0"
              }
            }
          },
          "10.11.11.242:32867": {
            "M": {
              "url": {
                "S": "http://10.11.11.242:32867"
              },
              "weight": {
                "N": "0"
              }
            }
          }
        }
      }
    }
  },
  "id": {
    "S": "test"
  },
  "name": {
    "S": "test"
  },
  "version": {
    "N": "4"
  }
}
```

### Frontends

ecs-task-tracker does NOT touch frontends. These are left up to something else to create and modify.

Here is an example frontend: 

```
{
  "frontend": {
    "backend": "test",
    "entryPoints": [
      "http"
    ],
    "routes": {
      "host": {
        "rule": "Host:test.services-staging.com"
      }
    }
  },
  "id": "test__frontend",
  "name": "test"
}
```

- "frontend" is similar to the "backend" above in that it is a traefik struct type Frontend marshalled into a dynamodbattribute
- "id" is a unique identifier usually made unique by using the name + "__frontend"
- "name" similar to the "name" in the backend item, it will be the name of the frontend in traefik

Notice the "frontend.backend" == "test" and that the backend item has a "name" of "test". This means that this front end will route traffic to the "test" backend if the requests have the header "Host:test.services-staging.com".

## Configuration / Environment Variables

There are no defaults for the env variables. The only one that can be left blank is the DEBUG variable.

```bash
REGION=us-east-1               # aws region
PORT=:8080                     # always of the form :port
TRAEFIK_TABLE=traefik-staging  # dynamodb table name
CLUSTER=staging                # ecs cluster name
DEBUG=on                       # if set to on, will print tons of crap
```

## Build

```
GOOS=darwin bash scripts/build_binary.sh
```

## Docker

There is a docker image on dockerhub at [tskinn/ecs-task-tracker](https://hub.docker.com/r/tskinn12/ecs-task-tracker/).
If you are running on AWS ECS there is a sample task-definition.

```
docker run -p 8080:8080 -e REGION=us-east-1 -e PORT=:8080 -e TRAEFIK_TABLE=table -e CLUSTER=cluster tskinn12/ecs-task-tracker
```
