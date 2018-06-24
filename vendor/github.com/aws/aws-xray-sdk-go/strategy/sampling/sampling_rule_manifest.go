// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package sampling

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
)

// RuleManifest represents a full sampling ruleset, with a list of
// custom rules and default values for incoming requests that do
// not match any of the provided rules.
type RuleManifest struct {
	Version int     `json:"version"`
	Default *Rule   `json:"default"`
	Rules   []*Rule `json:"rules"`
}

// ManifestFromFilePath creates a sampling ruleset from a given filepath fp.
func ManifestFromFilePath(fp string) (*RuleManifest, error) {
	b, err := ioutil.ReadFile(fp)
	if err == nil {
		s, e := ManifestFromJSONBytes(b)
		if e != nil {
			return nil, e
		}
		err = processManifest(s)
		if err != nil {
			return nil, err
		}
		return s, nil
	}
	return nil, err
}

// ManifestFromJSONBytes creates a sampling ruleset from given JSON bytes b.
func ManifestFromJSONBytes(b []byte) (*RuleManifest, error) {
	s := &RuleManifest{}
	err := json.Unmarshal(b, s)
	if err != nil {
		return nil, err
	}
	err = processManifest(s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// processManifest returns the provided manifest if valid,
// or an error if the provided manifest is invalid.
func processManifest(srm *RuleManifest) error {
	if nil == srm {
		return errors.New("sampling rule manifest must not be nil")
	}
	if 1 != srm.Version {
		return fmt.Errorf("sampling rule manifest version %d not supported", srm.Version)
	}
	if nil == srm.Default {
		return errors.New("sampling rule manifest must include a default rule")
	}
	if "" != srm.Default.URLPath || "" != srm.Default.ServiceName || "" != srm.Default.HTTPMethod {
		return errors.New("the default rule must not specify values for url_path, service_name, or http_method")
	}
	if srm.Default.FixedTarget < 0 || srm.Default.Rate < 0 {
		return errors.New("the default rule must specify non-negative values for fixed_target and rate")
	}

	res, err := NewReservoir(srm.Default.FixedTarget)
	if err != nil {
		return err
	}
	srm.Default.reservoir = res

	if srm.Rules != nil {
		for _, r := range srm.Rules {
			res, err := NewReservoir(r.FixedTarget)
			if err != nil {
				return err
			}
			r.reservoir = res
		}
	}
	return nil
}
