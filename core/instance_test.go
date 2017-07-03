package autospotting

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func TestMake(t *testing.T) {
	expected := map[string]*instance{}
	is := &instanceManager{}

	is.make()
	if !reflect.DeepEqual(is.catalog, expected) {
		t.Errorf("Catalog's type: '%s' expected: '%s'",
			reflect.TypeOf(is.catalog).String(),
			reflect.TypeOf(expected).String())
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name     string
		catalog  map[string]*instance
		expected map[string]*instance
	}{
		{name: "map contains a nil pointer",
			catalog: map[string]*instance{
				"inst1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
				"inst2": nil,
			},
			expected: map[string]*instance{
				"1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
			},
		},
		{name: "map has 1 instance",
			catalog: map[string]*instance{
				"inst1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
			},
			expected: map[string]*instance{
				"1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
			},
		},
		{name: "map has several instances",
			catalog: map[string]*instance{
				"inst1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
				"inst2": {Instance: &ec2.Instance{InstanceId: aws.String("2")}},
				"inst3": {Instance: &ec2.Instance{InstanceId: aws.String("3")}},
			},
			expected: map[string]*instance{
				"1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
				"2": {Instance: &ec2.Instance{InstanceId: aws.String("2")}},
				"3": {Instance: &ec2.Instance{InstanceId: aws.String("3")}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := &instanceManager{}
			is.make()
			for _, c := range tt.catalog {
				is.add(c)
			}
			if !reflect.DeepEqual(tt.expected, is.catalog) {
				t.Errorf("Value received: %v expected %v", is.catalog, tt.expected)
			}
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name     string
		catalog  map[string]*instance
		idToGet  string
		expected *instance
	}{
		{name: "map contains the required instance",
			catalog: map[string]*instance{
				"inst1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
				"inst2": {Instance: &ec2.Instance{InstanceId: aws.String("2")}},
				"inst3": {Instance: &ec2.Instance{InstanceId: aws.String("3")}},
			},
			idToGet:  "inst2",
			expected: &instance{Instance: &ec2.Instance{InstanceId: aws.String("2")}},
		},
		{name: "catalog doesn't contain the instance",
			catalog: map[string]*instance{
				"inst1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
				"inst2": {Instance: &ec2.Instance{InstanceId: aws.String("2")}},
				"inst3": {Instance: &ec2.Instance{InstanceId: aws.String("3")}},
			},
			idToGet:  "7",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := &instanceManager{}
			is.make()
			is.catalog = tt.catalog
			retInstance := is.get(tt.idToGet)
			if !reflect.DeepEqual(tt.expected, retInstance) {
				t.Errorf("Value received: %v expected %v", retInstance, tt.expected)
			}
		})
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		name     string
		catalog  map[string]*instance
		expected int
	}{
		{name: "map is nil",
			catalog:  nil,
			expected: 0,
		},
		{name: "map is empty",
			catalog:  map[string]*instance{},
			expected: 0,
		},
		{name: "map has 1 instance",
			catalog: map[string]*instance{
				"id-1": {},
			},
			expected: 1,
		},
		{name: "map has several instances",
			catalog: map[string]*instance{
				"id-1": {},
				"id-2": {},
				"id-3": {},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := &instanceManager{}
			is.catalog = tt.catalog
			ret := is.count()
			if ret != tt.expected {
				t.Errorf("Value received: '%d' expected %d", ret, tt.expected)
			}
		})
	}
}

func TestCount64(t *testing.T) {
	tests := []struct {
		name     string
		catalog  map[string]*instance
		expected int64
	}{
		{name: "map is nil",
			catalog:  nil,
			expected: 0,
		},
		{name: "map is empty",
			catalog:  map[string]*instance{},
			expected: 0,
		},
		{name: "map has 1 instance",
			catalog: map[string]*instance{
				"id-1": {},
			},
			expected: 1,
		},
		{name: "map has several instances",
			catalog: map[string]*instance{
				"id-1": {},
				"id-2": {},
				"id-3": {},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := &instanceManager{}
			is.catalog = tt.catalog
			ret := is.count64()
			if ret != tt.expected {
				t.Errorf("Value received: '%d' expected %d", ret, tt.expected)
			}
		})
	}
}

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
func TestIsEBSCompatible(t *testing.T) {
	tests := []struct {
		name         string
		spotInfo     instanceTypeInformation
		instanceInfo instance
		expected     bool
	}{
		{name: "EBS not Optimized Spot not Optimized",
			spotInfo: instanceTypeInformation{
				hasEBSOptimization: false,
			},
			instanceInfo: instance{
				Instance: &ec2.Instance{
					EbsOptimized: nil,
				},
			},
			expected: true,
		},
		{name: "EBS Optimized Spot Optimized",
			spotInfo: instanceTypeInformation{
				hasEBSOptimization: true,
			},
			instanceInfo: instance{
				Instance: &ec2.Instance{
					EbsOptimized: &[]bool{true}[0],
				},
			},
			expected: true,
		},
		{name: "EBS Optimized Spot not Optimized",
			spotInfo: instanceTypeInformation{
				hasEBSOptimization: false,
			},
			instanceInfo: instance{
				Instance: &ec2.Instance{
					EbsOptimized: &[]bool{true}[0],
				},
			},
			expected: false,
		},
		{name: "EBS not Optimized Spot Optimized",
			spotInfo: instanceTypeInformation{
				hasEBSOptimization: true,
			},
			instanceInfo: instance{
				Instance: &ec2.Instance{
					EbsOptimized: &[]bool{false}[0],
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &tt.instanceInfo
			retValue := i.isEBSCompatible(tt.spotInfo)
			if retValue != tt.expected {
				t.Errorf("Value received: %t expected %t", retValue, tt.expected)
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
		{name: "No spot price for such availability zone",
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
		{name: "Spot price is 0.0",
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
			spotPrice := i.calculatePrice(candidate)
			retValue := i.isPriceCompatible(spotPrice, tt.bestPrice)
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
		name           string
		spotInfos      map[string]instanceTypeInformation
		instanceInfo   *instance
		asg            *autoScalingGroup
		lc             *launchConfiguration
		expectedString string
		expectedError  error
	}{
		{name: "better/cheaper spot instance found",
			spotInfos: map[string]instanceTypeInformation{
				"1": {
					instanceType: "type1",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.5,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:   10,
					memory: 2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
				"2": {
					instanceType: "type2",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.8,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:   10,
					memory: 2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
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
					instanceType: "typeX",
					vCPU:         10,
					memory:       2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
				},
				price:  0.75,
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "test-asg",
				instances: makeInstancesWithCatalog(
					map[string]*instance{
						"id-1": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("id-1"),
								InstanceType:      aws.String("typeX"),
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1")},
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
						{
							VirtualName: aws.String("vn1"),
						},
						{
							VirtualName: aws.String("ephemeral"),
						},
					},
				},
			},
			expectedString: "type1",
			expectedError:  nil,
		},
		{name: "better/cheaper spot instance not found",
			spotInfos: map[string]instanceTypeInformation{
				"1": {
					instanceType: "type1",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.5,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:   10,
					memory: 2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
				},
				"2": {
					instanceType: "type2",
					pricing: prices{
						spot: map[string]float64{
							"eu-central-1": 0.8,
							"eu-west-1":    1.0,
							"eu-west-2":    2.0,
						},
					},
					vCPU:   10,
					memory: 2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
					virtualizationTypes:      []string{"PV", "else"},
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
					instanceType: "typeX",
					vCPU:         10,
					memory:       2.5,
					instanceStoreDeviceCount: 1,
					instanceStoreDeviceSize:  50.0,
					instanceStoreIsSSD:       false,
				},
				price:  0.45,
				region: &region{},
			},
			asg: &autoScalingGroup{
				name: "test-asg",
				instances: makeInstancesWithCatalog(
					map[string]*instance{
						"id-1": {
							Instance: &ec2.Instance{
								InstanceId:        aws.String("id-1"),
								InstanceType:      aws.String("typeX"),
								Placement:         &ec2.Placement{AvailabilityZone: aws.String("eu-west-1")},
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
					LaunchConfigurationName: aws.String("test"),
					BlockDeviceMappings: []*autoscaling.BlockDeviceMapping{
						{
							VirtualName: aws.String("vn1"),
						},
						{
							VirtualName: aws.String("ephemeral"),
						},
					},
				},
			},
			expectedString: "",
			expectedError:  errors.New("No cheaper spot instance types could be found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := tt.instanceInfo
			i.region.instanceTypeInformation = tt.spotInfos
			i.asg = tt.asg
			retValue, err := i.getCheapestCompatibleSpotInstanceType()
			if err == nil && tt.expectedError != err {
				t.Errorf("Error received: %v expected %v", err, tt.expectedError.Error())
			} else if err != nil && tt.expectedError == nil {
				t.Errorf("Error received: %s expected %s", err.Error(), tt.expectedError)
			} else if err != nil && tt.expectedError != nil && err.Error() != tt.expectedError.Error() {
				t.Errorf("Error received: %s expected %s", err.Error(), tt.expectedError.Error())
			} else if retValue != tt.expectedString {
				t.Errorf("Value received: %s expected %s", retValue, tt.expectedString)
			}
		})
	}
}

func TestTerminate(t *testing.T) {
	tests := []struct {
		name     string
		tags     []*ec2.Tag
		inst     *instance
		expected error
	}{
		{
			name: "no issue with terminate",
			tags: []*ec2.Tag{},
			inst: &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String("id1"),
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							tierr: nil,
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "issue with terminate",
			tags: []*ec2.Tag{},
			inst: &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String("id1"),
				},
				region: &region{
					services: connections{
						ec2: mockEC2{
							tierr: errors.New(""),
						},
					},
				},
			},
			expected: errors.New(""),
		},
	}
	for _, tt := range tests {
		ret := tt.inst.terminate()
		if ret != nil && ret.Error() != tt.expected.Error() {
			t.Errorf("error actual: %s, expected: %s", ret.Error(), tt.expected.Error())
		}
	}
}

// Ideally should find a better way to test tagging
// and avoid having a small wait of 1 timeout
func TestTag(t *testing.T) {

	tests := []struct {
		name          string
		tags          []*ec2.Tag
		inst          *instance
		expectedError error
	}{
		{
			name: "no tags without error",
			tags: []*ec2.Tag{},
			inst: &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String("id1"),
				},
				region: &region{
					name: "test",
					services: connections{
						ec2: mockEC2{
							cterr: nil,
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "no tags with error",
			tags: []*ec2.Tag{},
			inst: &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String("id1"),
				},
				region: &region{
					name: "test",
					services: connections{
						ec2: mockEC2{
							cterr: errors.New("no tags with error"),
						},
					},
				},
			},
			expectedError: errors.New("no tags with error"),
		},
		{
			name: "tags without error",
			tags: []*ec2.Tag{
				{Key: aws.String("proj"), Value: aws.String("test")},
				{Key: aws.String("x"), Value: aws.String("3")},
			},
			inst: &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String("id1"),
				},
				region: &region{
					name: "test",
					services: connections{
						ec2: mockEC2{
							cterr: nil,
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "tags with error",
			tags: []*ec2.Tag{
				{Key: aws.String("proj"), Value: aws.String("test")},
				{Key: aws.String("x"), Value: aws.String("3")},
			},
			inst: &instance{
				Instance: &ec2.Instance{
					InstanceId: aws.String("id1"),
				},
				region: &region{
					name: "test",
					services: connections{
						ec2: mockEC2{
							cterr: errors.New("tags with error"),
						},
					},
				},
			},
			expectedError: errors.New("tags with error"),
		},
	}

	mockedSleep := func(time.Duration) {}

	for _, tt := range tests {
		err := tt.inst.tag(tt.tags, 1, mockedSleep)
		CheckErrors(t, err, tt.expectedError)
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
