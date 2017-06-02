package utils_test

import (
	"errors"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type Ec2Mock struct {
	ec2iface.EC2API
	Instance    *ec2.Instance
	ReturnError bool
}

func (e *Ec2Mock) DescribeInstances(params *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	if e.ReturnError {
		return nil, errors.New("some error")
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{
				Instances: []*ec2.Instance{
					e.Instance,
				},
			},
		},
	}, nil
}
