package utils

import (
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/containous/traefik/types"
	"github.com/pkg/errors"
)

var arnToInstanceIDs map[string]*string

func (req *request) getInstanceIDs(containerInstanceARNS []*string) ([]*string, error) {
	// list to return
	instanceIDs := make([]*string, 0)
	// list of arns to query api
	paramsArns := make([]*string, 0)
	// add arns that aren't already stored in memory to list of arns to send to api
	// or get instanceID from memory and add to list of instanceIDs
	for _, arn := range containerInstanceARNS {
		util.Mutex.Lock()
		if id, exists := arnToInstanceIDs[*arn]; exists {
			instanceIDs = append(instanceIDs, id)
		} else {
			paramsArns = append(paramsArns, arn)
		}
		util.Mutex.Unlock()
	}
	if len(paramsArns) < 1 {
		return instanceIDs, nil
	}
	params := &ecs.DescribeContainerInstancesInput{
		ContainerInstances: paramsArns,
		Cluster:            aws.String(util.ECSCluster),
	}
	resp, err := util.ECS.DescribeContainerInstances(params)
	if err != nil {
		req.debug("error getting instance ids")
		return instanceIDs, errors.Wrap(err, "ecs.DescribeContainerInstances()")
	}
	for _, instance := range resp.ContainerInstances {
		instanceIDs = append(instanceIDs, instance.Ec2InstanceId)
		// save the arn to instance
		for i := range containerInstanceARNS {
			if *containerInstanceARNS[i] == *instance.ContainerInstanceArn {
				req.debug("saving instanceID: " + *instance.Ec2InstanceId)
				util.Mutex.Lock()
				arnToInstanceIDs[*containerInstanceARNS[i]] = instance.Ec2InstanceId
				util.Mutex.Unlock()
				break
			}
		}
	}
	return instanceIDs, nil
}

func (req *request) getInstanceID(ctrInstanceArn string) (*string, error) {
	instanceIDs, err := req.getInstanceIDs([]*string{&ctrInstanceArn})
	if err != nil {
		return nil, errors.Wrap(err, "getInstanceIDs()")
	}
	return instanceIDs[0], nil
}

func (req *request) listServices() ([]string, error) {
	services := make([]*string, 0)
	params := &ecs.ListServicesInput{
		Cluster: aws.String(util.ECSCluster),
	}
	err := util.ECS.ListServicesPages(params,
		func(page *ecs.ListServicesOutput, lastPage bool) bool {
			services = append(services, page.ServiceArns...)
			return !lastPage
		})
	if err != nil {
		req.debug("error listing services")
		return []string{}, errors.Wrap(err, "ecs.ListServicesPages()")
	}

	serviceNames := make([]string, len(services))
	for i := range serviceNames {
		serviceNames[i] = strings.Split(*services[i], "/")[1]
	}
	req.debug(strconv.Itoa(len(serviceNames)) + " services listed")
	return serviceNames, nil
}

func (req *request) getTaskArns(service string) ([]*string, error) {
	taskArns := make([]*string, 0)
	params := &ecs.ListTasksInput{
		Cluster: aws.String(util.ECSCluster),
	}
	if service != "" {
		params.ServiceName = aws.String(service)
	}
	err := util.ECS.ListTasksPages(params, func(page *ecs.ListTasksOutput, lastPage bool) bool {
		taskArns = append(taskArns, page.TaskArns...)
		return lastPage
	})
	return taskArns, err
}

func (req *request) getTasks(arns []*string) ([]*ecs.Task, error) {
	if len(arns) == 0 {
		return make([]*ecs.Task, 0), nil
	}

	params := &ecs.DescribeTasksInput{
		Tasks:   arns,
		Cluster: aws.String(util.ECSCluster),
	}
	resp, err := util.ECS.DescribeTasks(params)
	if err != nil {
		req.debug("error getting tasks: " + err.Error())
		return []*ecs.Task{}, err
	}
	return resp.Tasks, nil
}

func (req *request) getAddressOfTasks(tasks []*ecs.Task) []string {
	addresses := make([]string, 0)
	for _, task := range tasks {
		port := ""
		// we assume only one container and one networkbinding...
		if len(task.Containers) > 0 && len(task.Containers[0].NetworkBindings) > 0 {
			port = strconv.FormatInt(*task.Containers[0].NetworkBindings[0].HostPort, 10)
		}
		// skip entirely if no hostPort is mapped
		if port == "" {
			continue
		}
		ip, err := req.getIP(*task.ContainerInstanceArn)
		if err != nil {
			req.debug("error getting ip: " + err.Error())
			continue
		}
		addresses = append(addresses, ip+":"+port)
	}
	return addresses
}

func (req *request) getBackendECS(service string) (types.Backend, error) {
	var backend types.Backend
	taskArns, err := req.getTaskArns(service)
	if err != nil {
		req.debug("error listing tasks: " + err.Error())
		return backend, errors.Wrap(err, "getTaskArns()")
	}
	tasks, err := req.getTasks(taskArns)
	if err != nil {
		req.debug("error getting tasks: " + err.Error())
		return backend, errors.Wrap(err, "getTasks()")
	}
	addresses := req.getAddressOfTasks(tasks)
	backend = req.createBackend(addresses)

	if len(addresses) < 1 {
		req.debug(service + " has no network attached")
	}

	return backend, nil
}
