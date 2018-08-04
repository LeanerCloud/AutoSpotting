// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package sampling

import (
	"github.com/aws/aws-xray-sdk-go/resources"
	log "github.com/cihub/seelog"
)

// LocalizedStrategy makes trace sampling decisions based on
// a set of rules provided in a local JSON file. Trace sampling
// decisions are made by the root node in the trace. If a
// sampling decision is made by the root service, it will be passed
// to downstream services through the trace header.
type LocalizedStrategy struct {
	manifest *RuleManifest
}

// NewLocalizedStrategy initializes an instance of LocalizedStrategy
// with the default trace sampling rules. The default rules sample
// the first request per second, and 5% of requests thereafter.
func NewLocalizedStrategy() (*LocalizedStrategy, error) {
	bytes, err := resources.Asset("resources/DefaultSamplingRules.json")
	if err != nil {
		return nil, err
	}
	manifest, err := ManifestFromJSONBytes(bytes)
	if err != nil {
		return nil, err
	}
	return &LocalizedStrategy{manifest: manifest}, nil
}

// NewLocalizedStrategyFromFilePath initializes an instance of
// LocalizedStrategy using a custom ruleset found at the filepath fp.
func NewLocalizedStrategyFromFilePath(fp string) (*LocalizedStrategy, error) {
	manifest, err := ManifestFromFilePath(fp)
	if err != nil {
		return nil, err
	}
	return &LocalizedStrategy{manifest: manifest}, nil
}

// NewLocalizedStrategyFromJSONBytes initializes an instance of
// LocalizedStrategy using a custom ruleset provided in the json bytes b.
func NewLocalizedStrategyFromJSONBytes(b []byte) (*LocalizedStrategy, error) {
	manifest, err := ManifestFromJSONBytes(b)
	if err != nil {
		return nil, err
	}
	return &LocalizedStrategy{manifest: manifest}, nil
}

// ShouldTrace consults the LocalizedStrategy's rule set to determine
// if the given request should be traced or not.
func (lss *LocalizedStrategy) ShouldTrace(serviceName string, path string, method string) bool {
	log.Tracef("Determining ShouldTrace decision for:\n\tserviceName: %s\n\tpath: %s\n\tmethod: %s", serviceName, path, method)
	if nil != lss.manifest.Rules {
		for _, r := range lss.manifest.Rules {
			if r.AppliesTo(serviceName, path, method) {
				log.Tracef("Applicable rule:\n\tfixed_target: %d\n\trate: %f\n\tservice_name: %s\n\turl_path: %s\n\thttp_method: %s", r.FixedTarget, r.Rate, r.ServiceName, r.URLPath, r.HTTPMethod)
				return r.Sample()
			}
		}
	}
	log.Tracef("Default rule applies:\n\tfixed_target: %d\n\trate: %f", lss.manifest.Default.FixedTarget, lss.manifest.Default.Rate)
	return lss.manifest.Default.Sample()
}
