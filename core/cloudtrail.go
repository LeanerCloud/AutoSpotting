package autospotting

// CloudTrailEvent s used to unmarshal a CloudTrail Event from the Detail field
// of a CloudWatch event
type CloudTrailEvent struct {
	EventName         string            `json:"eventName"`
	AwsRegion         string            `json:"awsRegion"`
	ErrorCode         string            `json:"errorCode"`
	ErrorMessage      string            `json:"errorMessage"`
	RequestParameters RequestParameters `json:"requestParameters"`
}

// RequestParameters is used to unmarshal the parameters of a CloudTrail event
type RequestParameters struct {
	LifecycleHookName     string `json:"lifecycleHookName"`
	InstanceID            string `json:"instanceId"`
	LifecycleActionResult string `json:"lifecycleActionResult"`
	AutoScalingGroupName  string `json:"autoScalingGroupName"`
}
