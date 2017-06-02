package utils

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// HandleDiff diffs one service
func HandleDiff(serviceName string) (bool, error) {
	req := request{id: "DiffOne:::" + strconv.FormatInt(time.Now().Unix(), 10)}
	synced, err := req.diff(serviceName)
	if err != nil {
		req.log("error diffing service: " + serviceName + " : " + err.Error())
		return false, err
	}
	if !synced {
		req.log(serviceName + " is not in sync")
	} else {
		req.log(serviceName + " is in sync")
	}
	return synced, nil
}

// HandleDiffAll diffs all services in an ecs cluster
func HandleDiffAll() ([]string, error) {
	outOfSync := make([]string, 0)
	req := request{id: "DiffAll:::" + strconv.FormatInt(time.Now().Unix(), 10)}
	services, err := req.listServices()
	if err != nil {
		return outOfSync, errors.Wrap(err, "listServices()")
	}

	for _, service := range services {
		inSync, ierr := req.diff(service)
		if ierr != nil {
			if err == nil {
				err = errors.New("")
			}
			err = errors.Wrap(ierr, "diff("+service+"): "+err.Error())
			req.debug("error diffing service: " + service)
		}
		if !inSync {
			outOfSync = append(outOfSync, service)
		}
	}
	if len(outOfSync) > 0 {
		req.log("services that are out of sync: " + strings.Join(outOfSync, ", "))
	} else {
		req.log("all services are in sync")
	}
	return outOfSync, err
}

// HandleSNS parses a message from AWS SNS which contains info about ECS task
// updates (is it running or stopping, and port mapping) which is pushed to dynamodb
func HandleSNS(messageID string, body io.ReadCloser) error {
	req := request{
		id: "SNSNotif::" + messageID,
	}
	// Note the same endpoint needs to be able to handle subscription confirmations from sns
	notif, err := DecodeNotification(body)
	if err != nil {
		req.log("error decoding notfiction: DecodeNotification() " + err.Error())
		return errors.Wrap(err, "Notififcation DecodeNotification()")
	}
	req.debug("type is notification")
	event := Event{}
	err = json.Unmarshal([]byte(notif.Message), &event)
	if err != nil {
		req.log("failed to unmarshall message: " + err.Error())
		return errors.Wrap(err, "Unmarshal()")
	}
	err = req.processECSEventMessage(event.Detail)
	if err != nil {
		req.log("error processing ecs event message: " + err.Error())
		return err
	}
	req.log("handled sns notification for service: " + event.Detail.Group)
	return nil
}

// HandleSync syncs all tasks of one service with dynamodb
// It gets host ip and port on which the services tasks are listening and
// puts those in dynamodb as a backend
func HandleSync(service string) error {
	req := request{id: "SyncOne:::" + strconv.FormatInt(time.Now().Unix(), 10)}
	err := req.sync(service)
	if err != nil {
		req.log("error syncing service '" + service + "': " + err.Error())
		return errors.Wrap(err, "sync("+service+")")
	}
	req.log("successfully synced service: " + service)
	return nil
}

// HandleSyncAll syncs all the clusters tasks networking information to dynamodb
func HandleSyncAll() error {
	req := request{id: "SyncAll:::" + strconv.FormatInt(time.Now().Unix(), 10)}
	req.debug("syncing all")
	err := req.syncAll(0)
	if err != nil {
		req.log("error syncying one or more services: " + err.Error())
		return errors.Wrap(err, "syncAll(0)")
	}
	req.log("sucessfully synced all services")
	return nil
}

// HandleSyncSlow syncs every service in an ECS cluster with the dynamodb table
// and sleeps 'seconds' in between syncing each service
func HandleSyncSlow(milliseconds int) error {
	req := request{id: "SyncSlow::" + strconv.FormatInt(time.Now().Unix(), 10)}
	req.debug("syncing all services at a rate of one service every " + strconv.Itoa(milliseconds) + " milliseconds")
	err := req.syncAll(milliseconds)
	if err != nil {
		req.log("error slow syncing all services: " + err.Error())
		return errors.Wrap(err, "syncAll("+strconv.Itoa(milliseconds)+")")
	}
	req.log("successfully synced all services, sycing one service every " + strconv.Itoa(milliseconds) + " milliseconds")
	return nil
}
