// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0
package autospotting

import (
	"encoding/base64"
	"reflect"
	"testing"
)

func TestDecodeUserData(t *testing.T) {
	tests := []struct {
		name     string
		userData string
		want     string
	}{
		{
			name:     "returns plain user data as is",
			userData: "userDataPlain",
			want:     "userDataPlain",
		},
		{
			name:     "decodes base64 data",
			userData: base64.StdEncoding.EncodeToString([]byte("userData")),
			want:     "userData",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeUserData(&tt.userData)

			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("decodeUserData() = %v, want %v", *got, tt.want)
			}
		})
	}
}

func TestEncodeUserData(t *testing.T) {
	tests := []struct {
		name     string
		userData string
		want     string
	}{
		{
			name:     "encodes data to base64",
			userData: "userData",
			want:     base64.StdEncoding.EncodeToString([]byte("userData")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeUserData(&tt.userData)

			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("encodeUserData() = %v, want %v", *got, tt.want)
			}
		})
	}
}

func TestGetPatchedUserDataForBeanstalk(t *testing.T) {
	tests := []struct {
		name     string
		userData string
		want     string
	}{
		{
			name:     "does nothing if the user data does not belong to Beanstalk",
			userData: "userData",
			want:     "userData",
		},
		{
			name:     "decodes base64 data and does nothing if the user data does not belong to Beanstalk",
			userData: base64.StdEncoding.EncodeToString([]byte("userData")),
			want:     base64.StdEncoding.EncodeToString([]byte("userData")),
		},
		{
			name:     "adds wrappers",
			userData: "ebbootstrap\n#!/bin/bash\nscript",
			want:     base64.StdEncoding.EncodeToString([]byte("ebbootstrap\n#!/bin/bash\n" + beanstalkUserDataCFNWrappers + "script")),
		},
		{
			name:     "adds wrappers",
			userData: base64.StdEncoding.EncodeToString([]byte("ebbootstrap\n#!/bin/bash\nscript")),
			want:     base64.StdEncoding.EncodeToString([]byte("ebbootstrap\n#!/bin/bash\n" + beanstalkUserDataCFNWrappers + "script")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPatchedUserDataForBeanstalk(&tt.userData)

			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("getPatchedUserDataForBeanstalk() = %v, want %v", *got, tt.want)
			}
		})
	}
}
