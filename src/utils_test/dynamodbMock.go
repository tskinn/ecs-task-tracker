package utils_test

import (
	"errors"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

type DynamodbMock struct {
	dynamodbiface.DynamoDBAPI
	Items      map[string]map[string]*dynamodb.AttributeValue
	FailGet    bool
	FailPut    bool
	FailUpdate bool
}

func (d *DynamodbMock) GetItem(params *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	if d.FailGet {
		return nil, errors.New("boolfai")
	}

	output := &dynamodb.GetItemOutput{}

	paramsId := params.Key["id"].S
	item, ok := d.Items[*paramsId]

	if ok {
		output.Item = item
	}
	return output, nil
}

func (d *DynamodbMock) PutItem(params *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	if d.FailPut {
		return nil, errors.New("boofai")
	}

	idS := params.Item["id"].S
	if idS == nil {
		return nil, errors.New("bad params")
	}

	d.Items[*idS] = params.Item

	return &dynamodb.PutItemOutput{}, nil
}

func (d *DynamodbMock) UpdateItem(params *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	if d.FailUpdate {
		return nil, errors.New("boolfai")
	}

	idS := params.Key["id"].S
	if idS == nil {
		return nil, errors.New("bad params")
	}

	tmpItem, ok := d.Items[*idS]
	if !ok {
		return nil, errors.New("bad params")
	}

	// update backend
	tmpBackend := params.ExpressionAttributeValues[":b"]
	if tmpBackend == nil {
		return nil, errors.New("bad params: backend missing")
	}
	tmpItem["backend"] = tmpBackend

	// update version
	tmpVersion := tmpItem["version"].N
	if tmpVersion == nil {
		return nil, errors.New("bad params: version missing")
	}
	version, err := strconv.Atoi(*tmpVersion)
	if err != nil {
		return nil, errors.New("waht the crap")
	}

	tmpItem["version"] = &dynamodb.AttributeValue{
		N: aws.String(strconv.Itoa(version + 1)),
	}

	return &dynamodb.UpdateItemOutput{}, nil
}

func (d *DynamodbMock) PutBackend(item map[string]*dynamodb.AttributeValue) error {
	params := &dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String("test"),
	}
	_, err := d.PutItem(params)
	return err
}

func (d *DynamodbMock) DeleteItem(params *dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error) {
	id := params.Key["id"]
	if id == nil || id.S == nil {
		return nil, errors.New("Bad params. No 'id'")
	}
	delete(d.Items, *id.S)
	return nil, nil
}
