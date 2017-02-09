package autospotting

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"testing"
)

func TestIsSpot(t *testing.T) {

	tests := []struct {
		name      string
		lifeCycle *string
		expected  bool
	}{
		{name: "LifeCycle is nil",
			lifeCycle: nil,
			expected:  false,
		},
		{name: "LifeCycle is not nil but not spot",
			lifeCycle: aws.String("something"),
			expected:  false,
		},
		{name: "LifeCycle is not nil and is spot",
			lifeCycle: aws.String("spot"),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{Instance: &ec2.Instance{}}
			i.InstanceLifecycle = tt.lifeCycle
			retValue := i.isSpot()
			if retValue != tt.expected {
				if tt.lifeCycle != nil {
					t.Errorf("Value received for '%v': %t expected %t", *tt.lifeCycle, retValue, tt.expected)
				} else {
					t.Errorf("Value received for '%v': %t expected %t", tt.lifeCycle, retValue, tt.expected)
				}
			}
		})
	}
}

func TestIsPriceCompatible(t *testing.T) {
	tests := []struct {
		name             string
		spotPrices       prices
		availabilityZone *string
		instancePrice    float64
		bestPrice        float64
		expected         bool
	}{
		{name: "Not spot price for such availability zone",
			spotPrices: prices{
				spot: map[string]float64{
					"eu-central-1": 0.5,
					"eu-west-1":    1.0,
					"eu-west-2":    2.0,
				},
			},
			availabilityZone: aws.String("eu-west-42"),
			instancePrice:    5.0,
			bestPrice:        0.7,
			expected:         false,
		},
		{name: "Spot price is higher than bestPrice",
			spotPrices: prices{
				spot: map[string]float64{
					"eu-central-1": 0.5,
					"eu-west-1":    1.0,
					"eu-west-2":    2.0,
				},
			},
			availabilityZone: aws.String("eu-west-1"),
			instancePrice:    5.0,
			bestPrice:        0.7,
			expected:         false,
		},
		{name: "Spot price is lower than bestPrice",
			spotPrices: prices{
				spot: map[string]float64{
					"eu-central-1": 0.5,
					"eu-west-1":    1.0,
					"eu-west-2":    2.0,
				},
			},
			availabilityZone: aws.String("eu-west-1"),
			instancePrice:    5.0,
			bestPrice:        1.4,
			expected:         true,
		},
		{name: "Spot price is equal 0.0",
			spotPrices: prices{
				spot: map[string]float64{
					"eu-central-1": 0.0,
					"eu-west-1":    0.0,
					"eu-west-2":    0.0,
				},
			},
			availabilityZone: aws.String("eu-west-1"),
			instancePrice:    5.0,
			bestPrice:        1.4,
			expected:         false,
		},
		{name: "Spot price is higher than instance price",
			spotPrices: prices{
				spot: map[string]float64{
					"eu-central-1": 0.5,
					"eu-west-1":    1.0,
					"eu-west-2":    2.0,
				},
			},
			availabilityZone: aws.String("eu-west-1"),
			instancePrice:    0.7,
			bestPrice:        0.7,
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{Instance: &ec2.Instance{
				Placement: &ec2.Placement{
					AvailabilityZone: tt.availabilityZone,
				}},
				price: tt.instancePrice,
			}
			candidate := instanceTypeInformation{pricing: prices{}}
			candidate.pricing = tt.spotPrices
			retValue := i.isPriceCompatible(candidate, tt.bestPrice)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestIsClassCompatible(t *testing.T) {
	tests := []struct {
		name           string
		spotInfo       instanceTypeInformation
		instanceCPU    int
		instanceMemory float32
		expected       bool
	}{
		{name: "Spot is higher in both CPU & memory",
			spotInfo: instanceTypeInformation{
				vCPU:   10,
				memory: 2.5,
			},
			instanceCPU:    5,
			instanceMemory: 1.0,
			expected:       true,
		},
		{name: "Spot is lower in CPU but higher in memory",
			spotInfo: instanceTypeInformation{
				vCPU:   10,
				memory: 2.5,
			},
			instanceCPU:    15,
			instanceMemory: 1.0,
			expected:       false,
		},
		{name: "Spot is lower in memory but higher in CPU",
			spotInfo: instanceTypeInformation{
				vCPU:   10,
				memory: 2.5,
			},
			instanceCPU:    5,
			instanceMemory: 10.0,
			expected:       false,
		},
		{name: "Spot is lower in both CPU & memory",
			spotInfo: instanceTypeInformation{
				vCPU:   10,
				memory: 2.5,
			},
			instanceCPU:    15,
			instanceMemory: 5.0,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{typeInfo: instanceTypeInformation{
				vCPU:   tt.instanceCPU,
				memory: tt.instanceMemory,
			},
			}
			retValue := i.isClassCompatible(tt.spotInfo)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestIsStorageCompatible(t *testing.T) {
	tests := []struct {
		name            string
		spotInfo        instanceTypeInformation
		instanceInfo    instanceTypeInformation
		attachedVolumes int
		expected        bool
	}{
		{name: "Instance has no attached volumes",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 0.0,
				instanceStoreDeviceSize:  0.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 0.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 0,
			expected:        true,
		},
		{name: "Spot's storage is identical to instance",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 1,
				instanceStoreDeviceSize:  50.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 1,
			expected:        true,
		},
		{name: "Spot's storage is lower than the instance's one",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 1,
				instanceStoreDeviceSize:  25.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 1,
			expected:        false,
		},
		{name: "Spot's storage is bigger than the instance's one",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 1,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 1,
			expected:        true,
		},
		{name: "Spot's storage is bigger and has less storage attached",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 1,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 2,
			expected:        false,
		},
		{name: "Spot's storage is bigger and has more storage attached",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 4,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      false,
			},
			attachedVolumes: 1,
			expected:        true,
		},
		{name: "Spot's storage is bigger and has more storage attached but isn't SSD (unlike the instance)",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 4,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       false,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      true,
			},
			attachedVolumes: 1,
			expected:        false,
		},
		{name: "Spot's storage is bigger, has more storage attached, is SSD (like the instance)",
			spotInfo: instanceTypeInformation{
				instanceStoreDeviceCount: 4,
				instanceStoreDeviceSize:  150.0,
				instanceStoreIsSSD:       true,
			},
			instanceInfo: instanceTypeInformation{
				instanceStoreDeviceSize: 50.0,
				instanceStoreIsSSD:      true,
			},
			attachedVolumes: 1,
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{typeInfo: tt.instanceInfo}
			retValue := i.isStorageCompatible(tt.spotInfo, tt.attachedVolumes)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestIsVirtualizationCompatible(t *testing.T) {
	tests := []struct {
		name                       string
		spotVirtualizationTypes    []string
		instanceVirtualizationType *string
		expected                   bool
	}{
		{name: "Spot's virtualization includes instance's one (pv case)",
			spotVirtualizationTypes:    []string{"PV", "HVM"},
			instanceVirtualizationType: aws.String("paravirtual"),
			expected:                   true,
		},
		{name: "Spot's virtualization includes instance's one (hvm case)",
			spotVirtualizationTypes:    []string{"PV", "HVM"},
			instanceVirtualizationType: aws.String("hvm"),
			expected:                   true,
		},
		{name: "Spot's virtualization doesn't include instance's one (pv case)",
			spotVirtualizationTypes:    []string{"HVM"},
			instanceVirtualizationType: aws.String("paravirtual"),
			expected:                   false,
		},
		{name: "Spot's virtualization doesn't include instance's one (hvm case)",
			spotVirtualizationTypes:    []string{"PV"},
			instanceVirtualizationType: aws.String("hvm"),
			expected:                   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &instance{Instance: &ec2.Instance{
				VirtualizationType: tt.instanceVirtualizationType,
			}}
			retValue := i.isVirtualizationCompatible(tt.spotVirtualizationTypes)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestIsQuantityCompatible(t *testing.T) {
	tests := []struct {
		name               string
		asgName            string
		asgInstances       instances
		asgDesiredCapacity *int64
		availabilityZone   *string
		spotInfo           instanceTypeInformation
		expected           bool
	}{
		{name: "ASG spot ratio is too low (already 1 for 4)",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("id-1"),
							InstanceType:      aws.String("m3.medium"),
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			),
			asgDesiredCapacity: aws.Int64(4),
			spotInfo:           instanceTypeInformation{instanceType: "m3.medium"},
			availabilityZone:   aws.String("eu-west-1a"),
			expected:           false,
		},
		{name: "ASG spot ratio is high enough (only 1 for 10)",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("id-1"),
							InstanceType:      aws.String("m3.medium"),
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			),
			asgDesiredCapacity: aws.Int64(10),
			spotInfo:           instanceTypeInformation{instanceType: "m3.medium"},
			availabilityZone:   aws.String("eu-west-1a"),
			expected:           true,
		},
		{name: "ASG has no instances of this type",
			asgName: "test-asg",
			asgInstances: makeInstancesWithCatalog(
				map[string]*instance{
					"id-1": {
						Instance: &ec2.Instance{
							InstanceId:        aws.String("id-1"),
							InstanceType:      aws.String("m3.medium"),
							Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
							InstanceLifecycle: aws.String("spot"),
						},
					},
				},
			),
			asgDesiredCapacity: aws.Int64(5),
			spotInfo:           instanceTypeInformation{instanceType: "t2.micro"},
			availabilityZone:   aws.String("eu-west-1a"),
			expected:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &autoScalingGroup{
				name:      tt.name,
				instances: tt.asgInstances,
				Group: &autoscaling.Group{
					DesiredCapacity: tt.asgDesiredCapacity,
				},
			}
			i := &instance{
				Instance: &ec2.Instance{
					Placement: &ec2.Placement{
						AvailabilityZone: tt.availabilityZone,
					},
				},
				asg: a,
			}
			retValue := i.isSpotQuantityCompatible(tt.spotInfo)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
			}
		})
	}
}

func TestGetCheapestCompatibleSpotInstanceType(t *testing.T) {
	tests := []struct {
		name               string
		spotInfos          map[string]instanceTypeInformation
		instanceInfo       *instance
		asg                *autoScalingGroup
		lc                 *launchConfiguration
		expectedString     string
		expectedError      error
	}{
		{name: "ASG has no 'running' instance but has some",
			spotInfos: map[string]instanceTypeInformation{
				"test": instanceTypeInformation{
					instanceType: "m3.medium",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.5,
							"eu-west-1": 1.0,
							"eu-west-2": 2.0,
						},
					},
					vCPU: 10,
					memory: 2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize: 50.0,
					instanceStoreIsSSD: false,
					virtualizationTypes: []string{"PV", "else"},
				},
			},
			instanceInfo: &instance{
				Instance: &ec2.Instance{
					VirtualizationType: aws.String("paravirtual"),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String("eu-central-1"),
					},
				},
				typeInfo: instanceTypeInformation{
					instanceType: "t2.micro",
					vCPU: 10,
					memory: 2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize: 50.0,
					instanceStoreIsSSD: false,
				},
				price: 10,
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "test-asg",
				instances: makeInstancesWithCatalog(
					map[string]*instance{
						"id-1": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("id-1"),
								InstanceType:      aws.String("t2.micro"),
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1a")},
								InstanceLifecycle: aws.String("spot"),
							},
						},
					},
				),
				Group: &autoscaling.Group{
					DesiredCapacity: aws.Int64(4),
				},
			},
			lc: &launchConfiguration{
				LaunchConfiguration: &autoscaling.LaunchConfiguration{
					BlockDeviceMappings: []*autoscaling.BlockDeviceMapping{
						&autoscaling.BlockDeviceMapping{
							VirtualName: aws.String("vn1"),
						},
						&autoscaling.BlockDeviceMapping{
							VirtualName: aws.String("ephemeral"),
						},
					},
				},
			},
			expectedString: "m3.medium",
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := tt.instanceInfo
			i.region.instanceTypeInformation = tt.spotInfos
			i.asg = tt.asg
			retValue, err := i.getCheapestCompatibleSpotInstanceType()
			if err != tt.expectedError {
				t.Errorf("Error received: %v expected %v", err, tt.expectedError)
			} else if retValue != tt.expectedString {
				t.Errorf("Value received: %s expected %s", retValue, tt.expectedString)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name     string
		x        int
		y        int
		expected int
	}{
		{name: "Testing min between 0 and 0",
			x:        0,
			y:        0,
			expected: 0,
		},
		{name: "Testing min between 0 and 10",
			x:        0,
			y:        10,
			expected: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retValue := min(tt.x, tt.y)
			if retValue != tt.expected {
				t.Errorf("Value received: %d expected %d", retValue, tt.expected)
			}
		})
	}
}
