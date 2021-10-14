// Copyright (c) 2016-2021 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import "github.com/aws/aws-sdk-go/service/ec2"

type instance struct {
	*ec2.Instance
	typeInfo  instanceTypeInformation
	price     float64
	region    *region
	protected bool
	asg       *autoScalingGroup
}
