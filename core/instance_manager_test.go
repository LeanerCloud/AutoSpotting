// Copyright (c) 2016-2021 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func TestMake(t *testing.T) {
	expected := instanceMap{}
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
		catalog  instanceMap
		expected instanceMap
	}{
		{name: "map contains a nil pointer",
			catalog: instanceMap{
				"inst1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
				"inst2": nil,
			},
			expected: instanceMap{
				"1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
			},
		},
		{name: "map has 1 instance",
			catalog: instanceMap{
				"inst1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
			},
			expected: instanceMap{
				"1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
			},
		},
		{name: "map has several instances",
			catalog: instanceMap{
				"inst1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
				"inst2": {Instance: &ec2.Instance{InstanceId: aws.String("2")}},
				"inst3": {Instance: &ec2.Instance{InstanceId: aws.String("3")}},
			},
			expected: instanceMap{
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
		catalog  instanceMap
		idToGet  string
		expected *instance
	}{
		{name: "map contains the required instance",
			catalog: instanceMap{
				"inst1": {Instance: &ec2.Instance{InstanceId: aws.String("1")}},
				"inst2": {Instance: &ec2.Instance{InstanceId: aws.String("2")}},
				"inst3": {Instance: &ec2.Instance{InstanceId: aws.String("3")}},
			},
			idToGet:  "inst2",
			expected: &instance{Instance: &ec2.Instance{InstanceId: aws.String("2")}},
		},
		{name: "catalog doesn't contain the instance",
			catalog: instanceMap{
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
		catalog  instanceMap
		expected int
	}{
		{name: "map is nil",
			catalog:  nil,
			expected: 0,
		},
		{name: "map is empty",
			catalog:  instanceMap{},
			expected: 0,
		},
		{name: "map has 1 instance",
			catalog: instanceMap{
				"id-1": {},
			},
			expected: 1,
		},
		{name: "map has several instances",
			catalog: instanceMap{
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
		catalog  instanceMap
		expected int64
	}{
		{name: "map is nil",
			catalog:  nil,
			expected: 0,
		},
		{name: "map is empty",
			catalog:  instanceMap{},
			expected: 0,
		},
		{name: "map has 1 instance",
			catalog: instanceMap{
				"id-1": {},
			},
			expected: 1,
		},
		{name: "map has several instances",
			catalog: instanceMap{
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
