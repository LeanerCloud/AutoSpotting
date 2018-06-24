// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package plugins

const (
	EBServiceName  = "elastic_beanstalk"
	EC2ServiceName = "ec2"
	ECSServiceName = "ecs"
)

// InstancePluginMetadata points to the PluginMetadata struct.
var InstancePluginMetadata = &PluginMetadata{}

// PluginMetadata struct contains items to record information
// about the AWS infrastructure hosting the traced application.
type PluginMetadata struct {

	// EC2Metadata records the ec2 instance ID and availability zone.
	EC2Metadata *EC2Metadata

	// BeanstalkMetadata records the Elastic Beanstalk
	// environment name, version label, and deployment ID.
	BeanstalkMetadata *BeanstalkMetadata

	// ECSMetadata records the ECS container ID.
	ECSMetadata *ECSMetadata

	// Origin records original service of the segment.
	Origin string
}

// EC2Metadata provides the shape for unmarshalling EC2 metadata.
type EC2Metadata struct {
	InstanceID       string `json:"instance_id"`
	AvailabilityZone string `json:"availability_zone"`
}

// ECSMetadata provides the shape for unmarshalling
// ECS metadata.
type ECSMetadata struct {
	ContainerName string `json:"container"`
}

// BeanstalkMetadata provides the shape for unmarshalling
// Elastic Beanstalk environment metadata.
type BeanstalkMetadata struct {
	Environment  string `json:"environment_name"`
	VersionLabel string `json:"version_label"`
	DeploymentID int    `json:"deployment_id"`
}
