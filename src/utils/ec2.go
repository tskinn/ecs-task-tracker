package utils

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
)

var instancePrivateIPs map[string]string

// GetInstancePrivateIP gets the private ip
func (req *request) getInstancePrivateIP(instanceID string) (string, error) {
	// check to see if we already have it
	util.Mutex.Lock()
	if address, exists := instancePrivateIPs[instanceID]; exists {
		util.Mutex.Unlock()
		return address, nil
	}
	util.Mutex.Unlock()

	params := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}
	resp, err := util.EC2.DescribeInstances(params)
	if err != nil {
		return "", errors.Wrap(err, "ec2.DescribeInstances()")
	}
	if len(resp.Reservations) < 1 || len(resp.Reservations[0].Instances) < 1 {
		return "", errors.New("not instances found")
	}
	// save for later
	req.debug("saving instance and ip: " + instanceID + " " + *resp.Reservations[0].Instances[0].PrivateIpAddress)
	util.Mutex.Lock()
	instancePrivateIPs[instanceID] = *resp.Reservations[0].Instances[0].PrivateIpAddress
	util.Mutex.Unlock()
	return *resp.Reservations[0].Instances[0].PrivateIpAddress, nil
}
