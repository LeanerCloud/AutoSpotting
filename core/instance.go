// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import "github.com/aws/aws-sdk-go-v2/service/ec2/types"

type instance struct {
	*types.Instance
	typeInfo    instanceTypeInformation
	odPrice     float64
	region      *region
	protected   bool
	asg         *autoScalingGroup
	spotProduct *string
}
