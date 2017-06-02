package utils

import (
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/containous/traefik/types"
	"github.com/pkg/errors"
)

// GetItem gets an item from a dynamodb table
func (req *request) getItem(primaryKey, value string) (map[string]*dynamodb.AttributeValue, error) {
	params := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			primaryKey: {
				S: aws.String(value),
			},
		},
		TableName:      aws.String(util.TraefikTable),
		ConsistentRead: aws.Bool(true),
	}
	resp, err := util.DynamoDB.GetItem(params)
	if err != nil {
		req.debug("error getting item from dynamodb")
		return nil, errors.Wrap(err, "dynamodb.GetItem()")
	}
	if len(resp.Item) < 1 {
		req.debug("warning no item returned from dynamodb")
		return nil, errors.New(ErrItemNotFound)
	}
	req.debug("successfully got item from dynamodb")
	return resp.Item, nil
}

// GetBackend gest the backend
func (req *request) getBackendItem(backendName string) (BackendItem, error) {
	backend := BackendItem{}
	item, err := req.getItem("id", backendName+"__backend")
	if err != nil {
		req.debug("error getting backend from dynamodb: " + backendName)
		return backend, errors.Wrap(err, "getItem(id, "+backendName+")")
	}
	if err := dynamodbattribute.UnmarshalMap(item, &backend); err != nil {
		req.debug("error unmarshalling dynamodb item: " + backendName)
		return backend, errors.Wrap(err, "dynamodbattribute.UnmarshalMap()")
	}
	return backend, nil
}

func (req *request) getBackend(name string) (types.Backend, error) {
	item, err := req.getBackendItem(name)
	if err != nil {
		return types.Backend{}, errors.Wrap(err, "getBackendItem("+name+")")
	}
	return item.Backend, nil
}

// UpdateBackendWithLock updates the item with a lock. endItem must be a BackendItem or FrontendItem
func (req *request) updateBackendWithLock(endItem BackendItem) error {
	version := strconv.FormatUint(endItem.Version, 10)

	backendAttribute, err := dynamodbattribute.Marshal(endItem.Backend)
	if err != nil {
		return errors.Wrap(err, "dynamodbattribute.Marshal()")
	}
	params := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"id": {S: aws.String(endItem.ID)},
		},
		TableName:           aws.String(util.TraefikTable),
		ConditionExpression: aws.String("#v = :v"),
		UpdateExpression:    aws.String("SET #v = #v + :one, #b = :b"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":v":   {N: aws.String(version)},
			":one": {N: aws.String("1")},
			":b":   backendAttribute,
		},
		ExpressionAttributeNames: map[string]*string{
			"#v": aws.String("version"),
			"#b": aws.String("backend"),
		},
	}
	_, err = util.DynamoDB.UpdateItem(params)
	if err != nil {
		req.debug("error updataing backend in dynamodb")
		return errors.Wrap(err, "dynamodb.UpdateItem()")
	}
	return nil
}

// UpdateBackendDynamoDB updates the backend. If it doesn't exist it is created
// It will attempt as many times as MaxTries if the version is off
func (req *request) updateBackendDynamoDB(backendName string, traefikBackend types.Backend, overwriteServers bool) error {
	var err error
	var backend BackendItem
	for i := 0; i < util.MaxTries; i++ {
		// Get backend
		backend, err = req.getBackendItem(backendName)
		if err != nil {
			// Create Item if it doesn't exist
			if strings.Contains(err.Error(), ErrItemNotFound) {
				req.debug("backend not found: " + backendName)
				backendItem := req.createBackendItem(backendName, traefikBackend)
				err := req.createBackendDynamoDB(backendName, backendItem)
				return err
			}
			// if we get here then we got other issues
			return errors.Wrap(err, "getBackendItem("+backendName+")")
		}
		req.debug("successfully retrieved backend: " + backendName)

		var updatedBackend BackendItem
		if overwriteServers {
			updatedBackend = backend
			updatedBackend.Backend.Servers = traefikBackend.Servers
		} else {
			updatedBackend = req.updateBackendItemServers(backend, traefikBackend)
		}

		// Attempt to update dynamodb
		err = req.updateBackendWithLock(updatedBackend)
		if err == nil {
			req.debug("successfully updated backend: " + backendName)
			return nil
		}

		// if the error is because of something other than the condition not being met
		// then bail otherwise try again
		if !strings.Contains(err.Error(), dynamodb.ErrCodeConditionalCheckFailedException) {
			req.debug("error updating backend: " + backendName + " on try: " + strconv.Itoa(i))
			break
		}
		req.debug("item locked. trying again...")
		time.Sleep(100 * time.Millisecond)
	}
	return errors.Wrap(err, "tried to update "+strconv.Itoa(util.MaxTries)+" times")
}

// RemoveServerFromBackendDynamoDB removes a server from a backend
func (req *request) removeServerFromBackendDynamoDB(backendName, portIP string) error {
	req.debug("removing server: " + portIP + " from " + backendName)
	var err error
	var backend BackendItem
	for i := 0; i < util.MaxTries; i++ {
		backend, err = req.getBackendItem(backendName)
		if err != nil {
			return errors.Wrap(err, "getBackendItem("+backendName+")")
		}
		delete(backend.Backend.Servers, portIP)
		err = req.updateBackendWithLock(backend)
		if err == nil {
			return nil
		}

		if !strings.Contains(err.Error(), dynamodb.ErrCodeConditionalCheckFailedException) {
			return errors.Wrap(err, "updateBackendWithLock()")
		}
	}
	return err
}

// CreateBackendDynamoDB creates a backend item in dynamodb
func (req *request) createBackendDynamoDB(name string, backend BackendItem) error {
	req.debug("creating backend in dynamodb: " + name)
	backendItem, err := dynamodbattribute.MarshalMap(backend)
	if err != nil {
		req.debug("error marshaling backend: " + name)
		return errors.Wrap(err, "dynamodbattribute.MarshalMap()")
	}
	params := &dynamodb.PutItemInput{
		Item:      backendItem,
		TableName: aws.String(util.TraefikTable),
	}
	_, err = util.DynamoDB.PutItem(params)
	if err != nil {
		req.debug("error putting item in dynamodb: " + name)
		return errors.Wrap(err, "dynamodb.PutItem()")
	}
	req.debug("successfully created backend in dynamodb: " + name)
	return nil
}
