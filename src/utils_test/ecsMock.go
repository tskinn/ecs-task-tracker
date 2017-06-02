package utils_test

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
)

type EcsMock struct {
	ecsiface.ECSAPI
	ContainerInstances map[string]*ecs.ContainerInstance
	Services           map[string]bool
	Tasks              map[string]*ecs.Task
}

func (e *EcsMock) AddContainerInstance(instance *ecs.ContainerInstance) {
	e.ContainerInstances[*instance.ContainerInstanceArn] = instance
}

func (e *EcsMock) RemoveContainerInstance(instanceArn string) {
	delete(e.ContainerInstances, instanceArn)
}

func (e *EcsMock) GetContainerInstances() []*ecs.ContainerInstance {
	instances := make([]*ecs.ContainerInstance, len(e.ContainerInstances))
	i := 0
	for _, instance := range e.ContainerInstances {
		instances[i] = instance
		i++
	}
	return instances
}

func (e *EcsMock) AddService(service string) {
	service = "garbage/" + service
	e.Services[service] = true
}

func (e *EcsMock) RemoveService(service string) {
	service = "garbage/" + service
	delete(e.Services, service)
}

func (e *EcsMock) GetServices() []*string {
	services := make([]*string, 0)
	for service := range e.Services {
		services = append(services, &service)
	}
	return services
}

func (e *EcsMock) AddTask(task *ecs.Task) {
	// add the service if there is one in the task
	if task.Group != nil && strings.Contains(*task.Group, ":") {
		serviceName := strings.Split(*task.Group, ":")[1]
		if serviceName != "" {
			e.AddService(serviceName)
		}
	}

	e.Tasks[*task.TaskArn] = task
}

func (e *EcsMock) RemoveTask(taskArn string) {
	delete(e.Tasks, taskArn)
}

func (e *EcsMock) GetTasks() []*ecs.Task {
	tasks := make([]*ecs.Task, len(e.Tasks))
	i := 0
	for _, task := range e.Tasks {
		tasks[i] = task
		i++
	}
	return tasks
}

func (e *EcsMock) GetTaskArns() []*string {
	tasks := e.GetTasks()
	arns := make([]*string, len(tasks))
	for i := 0; i < len(tasks); i++ {
		arns[i] = tasks[i].TaskArn
	}
	return arns
}

func (e *EcsMock) DescribeContainerInstances(params *ecs.DescribeContainerInstancesInput) (*ecs.DescribeContainerInstancesOutput, error) {
	containerInstances := e.GetContainerInstances()
	if len(containerInstances) == 0 {
		return nil, errors.New("boom")
	}

	return &ecs.DescribeContainerInstancesOutput{
		ContainerInstances: containerInstances,
	}, nil
}

func (e *EcsMock) ListServicesPages(params *ecs.ListServicesInput, fn func(*ecs.ListServicesOutput, bool) bool) error {
	services := e.GetServices()
	if len(services) == 0 {
		return errors.New("boom fail")
	}
	for i := 0; i < len(services); i++ {
		tmpOutput := &ecs.ListServicesOutput{
			ServiceArns: []*string{
				services[i],
			},
		}
		lastPage := false
		if i == len(services)-1 {
			lastPage = true
		}
		fn(tmpOutput, lastPage)
	}
	return nil
}

func (e *EcsMock) ListTasksPages(params *ecs.ListTasksInput, fn func(*ecs.ListTasksOutput, bool) bool) error {
	taskArns := e.GetTaskArns()
	if len(taskArns) == 0 {
		return nil
		//return errors.New("boofil")
	}

	// call the function for each item to simulate getting pages
	for i := 0; i < len(taskArns); i++ {
		tmpOutput := &ecs.ListTasksOutput{
			TaskArns: []*string{
				taskArns[i],
			},
		}
		lastPage := false
		if i == len(taskArns)-1 {
			lastPage = true
		}
		fn(tmpOutput, lastPage)
	}
	return nil
}

func (e *EcsMock) DescribeTasks(params *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
	if len(e.Tasks) == 0 {
		return nil, errors.New("bofal")
	}

	return &ecs.DescribeTasksOutput{
		Tasks: e.GetTasks(),
	}, nil
}
