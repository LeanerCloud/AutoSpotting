package autospotting

import (
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
)

type instanceTypeInformation struct {
	instanceType             string
	vCPU                     int
	pricing                  prices
	memory                   float32
	virtualizationTypes      []string
	hasInstanceStore         bool
	instanceStoreDeviceSize  float32
	instanceStoreDeviceCount int
	instanceStoreIsSSD       bool
}

// The key in this map is the instance ID, useful for quick retrieval of
// instance attributes.
type instances struct {
	catalog map[string]*instance
}

func (i *instances) add(inst *instance) {
	debug.Println(inst)
	i.catalog[*inst.InstanceId] = inst
}

func (i *instances) get(id string) (inst *instance) {
	return i.catalog[id]
}

type instance struct {
	*ec2.Instance
	asg      *autoScalingGroup
	typeInfo instanceTypeInformation
	price    float64
}

func (it *instance) isSpot() bool {
	return (it.InstanceLifecycle != nil &&
		*it.InstanceLifecycle == "spot")
}

func (it *instance) terminate(svc *ec2.EC2) {

	if _, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{it.InstanceId},
	}); err != nil {
		logger.Println(err.Error())
	}
}

func (it *instance) filterTags() []*ec2.Tag {
	var filteredTags []*ec2.Tag

	tags := it.Tags

	// filtering reserved tags, which start with the "aws:" prefix
	for _, tag := range tags {
		if !strings.HasPrefix(*tag.Key, "aws:") {
			filteredTags = append(filteredTags, tag)
		}
	}

	return filteredTags
}
