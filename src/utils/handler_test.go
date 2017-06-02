package utils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/containous/traefik/types"
	"github.com/tskinn/ecs-task-tracker/src/utils_test"
)

var ecsM *utils_test.EcsMock
var ec2M *utils_test.Ec2Mock
var dynamodbM *utils_test.DynamodbMock

func init() {
	ecsM = &utils_test.EcsMock{
		ContainerInstances: make(map[string]*ecs.ContainerInstance),
		Services:           make(map[string]bool),
		Tasks:              make(map[string]*ecs.Task),
	}

	ec2M = &utils_test.Ec2Mock{
		Instance:    &ec2.Instance{},
		ReturnError: false,
	}

	dynamodbM = &utils_test.DynamodbMock{
		Items: make(map[string]map[string]*dynamodb.AttributeValue),
	}

	Init("test", "test", 1, dynamodbM, ec2M, ecsM, "")
}

func TestHandleDiffSame(t *testing.T) {
	//	ecsM.AddService("hello")
	createEnv("myinstancearn", "hello", "myinstanceid", "10.0.0.4", 8090)
	good, err := HandleDiff("hello")
	if err != nil {
		t.Log("there was an error")
		t.Log(err)
		t.Fail()
	}
	if !good {
		t.Log(dynamodbM.Items)
		t.Log(ecsM.Tasks)
		t.Log("dynamodb is not in sync with ecs")
		t.Fail()
	}
}

func TestHandleDiffDifferent(t *testing.T) {

}

func TestHandleDiffAll(t *testing.T) {
	notSynced, err := HandleDiffAll()
	if err != nil {
		t.Log("its not synced up yo")
		t.Log(notSynced)
		t.Fail()
	}
}

func TestHandleSNSNotificationRemoveBackend(t *testing.T) {
	instanceArn, taskName, instanceID, instanceIP, hostPort := "myinstancearn", "taskname", "instanceid", "10.0.0.4", 8080
	createEnv(instanceArn, taskName, instanceID, instanceIP, hostPort)

	msg, _ := json.Marshal(&Event{
		Detail: Detail{
			Group:                "garbage:" + taskName,
			ContainerInstanceArn: instanceArn,
			DesiredStatus:        "STOPPED",
			LastStatus:           "RUNNING",
			TaskArn:              taskName,
			Containers:           []Container{{NetworkBindings: []NetworkBinding{{HostPort: hostPort}}}},
		},
	})

	notification := &Notification{
		Message: string(msg[:]),
	}
	notificationEncoded, _ := json.Marshal(notification)
	body := ioutil.NopCloser(bytes.NewReader(notificationEncoded))
	err := HandleSNS("TestNotification::Remove", body)
	if err != nil {
		t.Fail()
	}
}

func TestHandleSNSNotificationAddBackend(t *testing.T) {
	instanceArn, taskName, instanceID, instanceIP, hostPort := "myinstancearn", "taskname", "instanceid", "10.0.0.4", 8080
	createEnv(instanceArn, taskName, instanceID, instanceIP, hostPort)
	instanceArn, taskName, instanceID, instanceIP, hostPort = "myinstancearn", "secondtask", "instanceid", "10.0.0.4", 8999
	msg, _ := json.Marshal(&Event{
		Detail: Detail{
			Group:                "garbage:" + taskName,
			ContainerInstanceArn: instanceArn,
			DesiredStatus:        "RUNNING",
			LastStatus:           "RUNNING",
			TaskArn:              taskName,
			Containers:           []Container{{NetworkBindings: []NetworkBinding{{HostPort: hostPort}}}},
		},
	})

	notification := &Notification{
		Message: string(msg[:]),
	}
	notificationEncoded, _ := json.Marshal(notification)
	body := ioutil.NopCloser(bytes.NewReader(notificationEncoded))
	err := HandleSNS("TestNotification::Add", body)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
}

func TestHandleSNSUnknownType(t *testing.T) {
	notification := &Notification{}
	notificationEncoded, _ := json.Marshal(notification)
	body := ioutil.NopCloser(bytes.NewReader(notificationEncoded))
	err := HandleSNS("somemessageid", body)
	if err == nil { // should throw an error
		t.Fail()
	}
}

func TestHandleSyncOutOfSync(t *testing.T) {
	instanceArn, taskName, instanceID, instanceIP, hostPort := "myinstancearn", "taskname", "instanceid", "10.0.0.4", 8080
	createEnv(instanceArn, taskName, instanceID, instanceIP, hostPort)
	instanceArn, taskName, instanceID, instanceIP, hostPort = "myinstancearn", "secondtask", "instanceid", "10.0.0.4", 8999

	ecsM.AddTask(&ecs.Task{
		ContainerInstanceArn: aws.String(instanceArn),
		TaskArn:              aws.String(taskName + "-arn2"),
		Group:                aws.String("garbage:" + taskName),
		Containers: []*ecs.Container{
			{
				NetworkBindings: []*ecs.NetworkBinding{
					{
						HostPort: aws.Int64(int64(hostPort)),
					},
				},
			},
		},
	})

	err := HandleSync(taskName)
	if err != nil {
		t.Log("broked")
		t.Fail()
	}
}

func TestHandleSyncInSync(t *testing.T) {
	instanceArn, taskName, instanceID, instanceIP, hostPort := "myinstancearn", "taskname", "instanceid", "10.0.0.4", 8080
	createEnv(instanceArn, taskName, instanceID, instanceIP, hostPort)

	err := HandleSync(taskName)
	if err != nil {
		t.Log("broked")
		t.Fail()
	}
}

func TestHandleSyncAll(t *testing.T) {
	instanceArn, taskName, instanceID, instanceIP, hostPort := "myinstancearn", "taskname", "instanceid", "10.0.0.4", 8080
	createEnv(instanceArn, taskName, instanceID, instanceIP, hostPort)
	t.Log(dynamodbM.Items)

	params := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"id": {S: &taskName}},
	}
	dynamodbM.DeleteItem(params)

	err := HandleSyncAll()
	if err != nil {
		t.Log("kdjosk")
		t.Fail()
	}
}

func createEnv(instanceArn, taskName, instanceID, instanceIP string, hostPort int) {
	// add task
	ecsM.AddTask(&ecs.Task{
		ContainerInstanceArn: aws.String(instanceArn),
		TaskArn:              aws.String(taskName + "-arn"),
		Group:                aws.String("garbage:" + taskName),
		Containers: []*ecs.Container{
			{
				NetworkBindings: []*ecs.NetworkBinding{
					{
						HostPort: aws.Int64(int64(hostPort)),
					},
				},
			},
		},
	})

	// add container instance
	ecsM.AddContainerInstance(&ecs.ContainerInstance{
		ContainerInstanceArn: aws.String(instanceArn),
		Ec2InstanceId:        aws.String(instanceID),
	})

	// set the instance
	ec2M.Instance = &ec2.Instance{
		PrivateIpAddress: aws.String(instanceIP),
		InstanceId:       aws.String(instanceID),
	}

	// create and add backend to dynamodb
	address := instanceIP + ":" + strconv.Itoa(hostPort)
	name := taskName
	traefikBackend := types.Backend{
		Servers: map[string]types.Server{
			address: {
				URL: "http://" + address,
			},
		},
	}
	backendItem := BackendItem{
		Backend: traefikBackend,
		EndItem: EndItem{
			ID:      name + "__backend",
			Name:    name,
			Version: 0,
		},
	}
	item, err := dynamodbattribute.MarshalMap(backendItem)
	if err != nil {
		// crap
	}
	dynamodbM.PutBackend(item)
}
