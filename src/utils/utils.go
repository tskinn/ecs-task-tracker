package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
	"github.com/containous/traefik/types"
	"github.com/pkg/errors"
)

const (
	// Running matches the running status of an ecs task. See http://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_life_cycle.html
	Running = "RUNNING"
	// Stopped matches the stopped status of an ecs task
	Stopped = "STOPPED"
	// ErrNoNetworkBindings is thrown when there are not network bindings attached to a task
	ErrNoNetworkBindings = "NoNetworkBindings"
	// ErrItemNotFound is thrown when an item isn't found in the database
	ErrItemNotFound = "ItemNotFound"
)

var util Util

type request struct {
	id string
}

// Util holds global configuraton for the utils package
type Util struct {
	DynamoDB       dynamodbiface.DynamoDBAPI
	EC2            ec2iface.EC2API
	ECS            ecsiface.ECSAPI
	ECSCluster     string
	HostNameTable  string
	PrivateIPTable string
	TraefikTable   string
	MaxTries       int
	Mutex          *sync.Mutex
	Debug          bool
}

// EndItem is a backend or frontend that will be marshalled into a dynamodb item
type EndItem struct {
	ID      string `dynamodbav:"id"`
	Name    string `dynamodbav:"name"`
	Version uint64 `dynamodbav:"version"`
}

// BackendItem will be marshaled into dynamodb item
type BackendItem struct {
	Backend types.Backend `dynamodbav:"backend"`
	EndItem
}

// FrontendItem will be marshaled into dynamodb item
type FrontendItem struct {
	Frontend types.Frontend `dynamodbav:"frontend"`
	EndItem
}

// NetworkBinding is ...
type NetworkBinding struct {
	HostPort int `json:"hostPort"`
}

// Container is ...
type Container struct {
	ContainerArn    string `json:"containerArn"`
	LastStatus      string `json:"lastStatus"`
	Name            string `json:"name"`
	NetworkBindings []NetworkBinding
}

// Detail is ...
type Detail struct {
	ClusterArn           string `json:"clusterArn"`
	ContainerInstanceArn string `json:"containerInstanceArn"`
	DesiredStatus        string `json:"desiredStatus"`
	Group                string `json:"group"`
	LastStatus           string `json:"lastStatus"`
	TaskArn              string `json:"taskArn"`
	TaskDefinitionArn    string `json:"taskDefinitionArn"`
	Containers           []Container
}

// Event is the format of the event sent from ecs
// which actually maps to the Message in the Notification
type Event struct {
	Version    string `json:"version"`
	ID         string `json:"id"`
	DetailType string `json:"detail-type"`
	Source     string `json:"source"`
	Account    string `json:"account"`
	Time       string `json:"time"`
	Region     string `json:"region"`
	Detail     Detail `json:"detail"`
}

// Notification is the format of SNS Notification
type Notification struct {
	Message          string
	MessageID        string
	Signature        string
	SignatureVersion string
	SigningCertURL   string
	SubscribeURL     string
	Subject          string
	Timestamp        string
	TopicArn         string
	Type             string
	UnsubscribeURL   string
}

// Init sets the necesary values for this package to function properly
// This must be called before using this package!
func Init(traefikTable, ecsCluster string,
	maxTries int,
	dynamo dynamodbiface.DynamoDBAPI,
	ec2Svc ec2iface.EC2API,
	ecsSvc ecsiface.ECSAPI,
	debugOn string) {
	debug := false
	if debugOn == "on" {
		debug = true
	}
	util = Util{
		TraefikTable: traefikTable,
		ECSCluster:   ecsCluster,
		MaxTries:     maxTries,
		DynamoDB:     dynamo,
		EC2:          ec2Svc,
		ECS:          ecsSvc,
		Mutex:        &sync.Mutex{},
		Debug:        debug,
	}

	arnToInstanceIDs = make(map[string]*string)
	instancePrivateIPs = make(map[string]string)
}

func (req *request) getIP(containerInstanceArn string) (string, error) {
	instanceID, err := req.getInstanceID(containerInstanceArn)
	if err != nil {
		return "", errors.Wrap(err, "getInstanceID()")
	}
	ip, err := req.getInstancePrivateIP(*instanceID)
	if err != nil {
		return "", errors.Wrap(err, "getInstancePrivateIP()")
	}
	return ip, nil
}

// debug is just a crappy debugging mechanism
func (req *request) debug(str string) {
	if !util.Debug {
		return
	}

	_, file, line, ok := runtime.Caller(1)
	if ok {
		base := filepath.Base(file)
		fmt.Printf("%s::%d::%s::%s\n", base, line, string(req.id), str)
	}
}

func (req *request) log(str interface{}) {
	fmt.Printf("%s ::: %s\n", string(req.id), str)
}

// processECSEventMessage parses an event from ECS and updates dynamodb accordingly
func (req *request) processECSEventMessage(msg Detail) error {
	if len(msg.Containers) < 1 {
		req.debug("skipping message. no containers listed")
		return nil
	}
	if len(msg.Containers[0].NetworkBindings) < 1 {
		req.debug("skipping message. no networkbindings on container")
		return nil
	}
	serviceName := strings.Split(msg.Group, ":")[1]
	ip, err := req.getIP(msg.ContainerInstanceArn)
	if err != nil {
		req.debug("unable to get port")
		return errors.Wrap(err, "getIP("+msg.ContainerInstanceArn+")")
	}
	// Assuming only one container and one networkbinding exists...
	hostPort := strconv.Itoa(msg.Containers[0].NetworkBindings[0].HostPort)
	if hostPort == "" {
		return errors.New("error HostPort doesn't exist")
	}
	portIP := ip + ":" + hostPort

	if msg.LastStatus == Running && msg.DesiredStatus == Running {
		// add to dynamodb
		backend := req.createBackend([]string{portIP})
		err = req.updateBackendDynamoDB(serviceName, backend, false)
		if err != nil {
			req.debug("unable to update backend in dynamodb for " + serviceName + portIP)
			return errors.Wrap(err, "updateBackendDynamoDB("+serviceName+","+portIP+")")
		}
		req.debug("successfully updated backend in dynamodb for " + serviceName + portIP)
	} else if msg.DesiredStatus == Stopped {
		err = req.removeServerFromBackendDynamoDB(serviceName, portIP)
		if err != nil {
			req.debug("unable to remove server from backend in dynamodb" + serviceName + portIP)
			return errors.Wrap(err, "removeServerFromBackendDynamoDB("+serviceName+","+portIP+")")
		}
		req.debug("successfully removed server from backend in dynamodb" + serviceName + portIP)
	} else {
		req.debug("skipping...")
	}
	return nil
}

// decodes an sns notification
func DecodeNotification(body io.ReadCloser) (Notification, error) {
	var notif Notification
	defer body.Close()
	decoder := json.NewDecoder(body)
	err := decoder.Decode(&notif)
	if err != nil {
		return notif, errors.Wrap(err, "Decode()")
	}
	return notif, nil
}

// syncs a given service to dynamodb
func (req *request) sync(service string) error {
	req.debug("syncing service: " + service)

	backend, err := req.getBackendECS(service)
	// if its a no bindings error still update the backend with no empty backend
	if err != nil {
		return errors.Wrap(err, "getBackendECS("+service+")")
	}

	// overwrite current backend
	err = req.updateBackendDynamoDB(service, backend, true)
	if err != nil {
		req.debug("error syncing dyamodb: " + err.Error())
		return errors.Wrap(err, "updateBackendDynamoDB("+service+", interface{})")
	}

	return nil
}

// syncs all services in an ecs cluster to dynamodb
func (req *request) syncAll(milliseconds int) error {
	services, err := req.listServices()
	if err != nil {
		return errors.Wrap(err, "listServices()")
	}
	for _, service := range services {
		ierr := req.sync(service)
		if ierr != nil {
			// don't err on services that don't have networkbindings
			if strings.Contains(ierr.Error(), ErrNoNetworkBindings) {
				continue
			}

			// can't wrap nil errs so create one before we wrap it
			if err == nil {
				err = errors.New("")
			}
			err = errors.Wrap(ierr, "sync("+service+")")
		}
		time.Sleep(time.Duration(milliseconds) * time.Millisecond)
	}
	if err != nil {
		req.debug("error one or more services were unable to be synced")
		return errors.Wrap(err, "error syncing one or more services")
	}
	return nil
}

// compares what is stored in dynamodb to what is returned from ecs api calls
// for a give service in the ecs cluster
// NOTE: it only compares the Servers see traefik types.Server
// returns true, nil if there is no difference
//    and false, nil if there is a difference
//    and false, err if there was an error at any point in the process
func (req *request) diff(service string) (bool, error) {
	req.debug("diffing service: " + service)
	ecsBackend, err := req.getBackendECS(service)
	// ignore the error if it was caused by no networkbindings
	if err != nil {
		return false, errors.Wrap(err, "getBackendECS("+service+")")
	}
	dynamoBackend, err := req.getBackend(service)
	// ignore the error if it was caused by item not being in dynamodb
	if err != nil && !strings.Contains(err.Error(), ErrItemNotFound) {
		return false, errors.Wrap(err, "getBackend( "+service+")")
	}

	// only compare servers. we will allow different config to be set manually for the backend
	// that will not be generated by queurying ECS for addresses
	// for example if the user wanted to create a circuit breaker they could add this manually
	// in dynamodb without worrying about it getting modified by this program
	if reflect.DeepEqual(ecsBackend.Servers, dynamoBackend.Servers) {
		req.debug("the " + service + " service is in sync")
		return true, nil
	}

	req.debug("the " + service + " service is NOT in sync")
	return false, nil
}

// creates a types.Backend given a []string of addresses (ip:port)
func (req *request) createBackend(addresses []string) types.Backend {
	if len(addresses) == 0 {
		return types.Backend{}
	}
	servers := make(map[string]types.Server)
	for _, addr := range addresses {
		servers[addr] = types.Server{URL: "http://" + addr}
	}

	backend := types.Backend{
		Servers: servers,
	}
	return backend
}

// creates a BackendItem given the name of the backend and the types.Backend
func (req *request) createBackendItem(name string, backend types.Backend) BackendItem {
	backendItem := BackendItem{
		Backend: backend,
		EndItem: EndItem{
			ID:      name + "__backend",
			Name:    name,
			Version: 0,
		},
	}

	return backendItem
}

// adds the servers from backend to backendItems servers
func (req *request) updateBackendItemServers(backendItem BackendItem, backend types.Backend) BackendItem {
	if backendItem.Backend.Servers == nil {
		backendItem.Backend.Servers = make(map[string]types.Server)
	}
	for name, server := range backend.Servers {
		backendItem.Backend.Servers[name] = server
	}
	return backendItem
}
