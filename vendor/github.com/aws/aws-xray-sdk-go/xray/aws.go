// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package xray

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http/httptrace"
	"reflect"
	"strings"
	"unicode"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-xray-sdk-go/resources"
	log "github.com/cihub/seelog"
)

const RequestIDKey string = "request_id"
const ExtendedRequestIDKey string = "id_2"
const S3ExtendedRequestIDHeaderKey string = "x-amz-id-2"

type jsonMap struct {
	object interface{}
}

const (
	requestKeyword = iota
	responseKeyword
)

func beginSubsegment(r *request.Request, name string) {
	ctx, _ := BeginSubsegment(r.HTTPRequest.Context(), name)
	r.HTTPRequest = r.HTTPRequest.WithContext(ctx)
}

func endSubsegment(r *request.Request) {
	seg := GetSegment(r.HTTPRequest.Context())
	if seg == nil {
		return
	}
	seg.Close(r.Error)
	r.HTTPRequest = r.HTTPRequest.WithContext(context.WithValue(r.HTTPRequest.Context(), ContextKey, seg.parent))
}

var xRayBeforeValidateHandler = request.NamedHandler{
	Name: "XRayBeforeValidateHandler",
	Fn: func(r *request.Request) {
		ctx, opseg := BeginSubsegment(r.HTTPRequest.Context(), r.ClientInfo.ServiceName)
		if opseg == nil {
			return
		}
		opseg.Namespace = "aws"
		marshalctx, _ := BeginSubsegment(ctx, "marshal")

		r.HTTPRequest = r.HTTPRequest.WithContext(marshalctx)
		r.HTTPRequest.Header.Set("x-amzn-trace-id", opseg.DownstreamHeader().String())
	},
}

var xRayAfterBuildHandler = request.NamedHandler{
	Name: "XRayAfterBuildHandler",
	Fn: func(r *request.Request) {
		endSubsegment(r)
	},
}

var xRayBeforeSignHandler = request.NamedHandler{
	Name: "XRayBeforeSignHandler",
	Fn: func(r *request.Request) {
		ctx, seg := BeginSubsegment(r.HTTPRequest.Context(), "attempt")
		if seg == nil {
			return
		}
		ct, _ := NewClientTrace(ctx)
		r.HTTPRequest = r.HTTPRequest.WithContext(httptrace.WithClientTrace(ctx, ct.httpTrace))
	},
}

var xRayAfterSignHandler = request.NamedHandler{
	Name: "XRayAfterSignHandler",
	Fn: func(r *request.Request) {
		endSubsegment(r)
	},
}

var xRayBeforeSendHandler = request.NamedHandler{
	Name: "XRayBeforeSendHandler",
	Fn: func(r *request.Request) {
	},
}

var xRayAfterSendHandler = request.NamedHandler{
	Name: "XRayAfterSendHandler",
	Fn: func(r *request.Request) {
		endSubsegment(r)
	},
}

var xRayBeforeUnmarshalHandler = request.NamedHandler{
	Name: "XRayBeforeUnmarshalHandler",
	Fn: func(r *request.Request) {
		endSubsegment(r) // end attempt subsegment
		beginSubsegment(r, "unmarshal")
	},
}

var xRayAfterUnmarshalHandler = request.NamedHandler{
	Name: "XRayAfterUnmarshalHandler",
	Fn: func(r *request.Request) {
		endSubsegment(r)
	},
}

var xRayBeforeRetryHandler = request.NamedHandler{
	Name: "XRayBeforeRetryHandler",
	Fn: func(r *request.Request) {
		endSubsegment(r) // end attempt subsegment
		ctx, _ := BeginSubsegment(r.HTTPRequest.Context(), "wait")

		r.HTTPRequest = r.HTTPRequest.WithContext(ctx)
	},
}

var xRayAfterRetryHandler = request.NamedHandler{
	Name: "XRayAfterRetryHandler",
	Fn: func(r *request.Request) {
		endSubsegment(r)
	},
}

func pushHandlers(c *client.Client) {
	c.Handlers.Validate.PushFrontNamed(xRayBeforeValidateHandler)
	c.Handlers.Build.PushBackNamed(xRayAfterBuildHandler)
	c.Handlers.Sign.PushFrontNamed(xRayBeforeSignHandler)
	c.Handlers.Unmarshal.PushFrontNamed(xRayBeforeUnmarshalHandler)
	c.Handlers.Unmarshal.PushBackNamed(xRayAfterUnmarshalHandler)
	c.Handlers.Retry.PushFrontNamed(xRayBeforeRetryHandler)
	c.Handlers.AfterRetry.PushBackNamed(xRayAfterRetryHandler)
}

// AWS adds X-Ray tracing to an AWS client.
func AWS(c *client.Client) {
	if c == nil {
		panic("Please initialize the provided AWS client before passing to the AWS() method.")
	}
	pushHandlers(c)
	c.Handlers.Complete.PushFrontNamed(xrayCompleteHandler(""))
}

// AWSWithWhitelist allows a custom parameter whitelist JSON file to be defined.
func AWSWithWhitelist(c *client.Client, filename string) {
	if c == nil {
		panic("Please initialize the provided AWS client before passing to the AWSWithWhitelist() method.")
	}
	pushHandlers(c)
	c.Handlers.Complete.PushFrontNamed(xrayCompleteHandler(filename))
}

func xrayCompleteHandler(filename string) request.NamedHandler {
	whitelistJSON := parseWhitelistJSON(filename)
	whitelist := &jsonMap{}
	err := json.Unmarshal(whitelistJSON, &whitelist.object)
	if err != nil {
		panic(err)
	}

	return request.NamedHandler{
		Name: "XRayCompleteHandler",
		Fn: func(r *request.Request) {
			curseg := GetSegment(r.HTTPRequest.Context())

			for curseg != nil && curseg.Namespace != "aws" {
				curseg.Close(nil)
				curseg = curseg.parent
			}
			if curseg == nil {
				return
			}

			opseg := curseg

			opseg.Lock()
			for k, v := range extractRequestParameters(r, whitelist) {
				opseg.GetAWS()[strings.ToLower(addUnderScoreBetweenWords(k))] = v
			}
			for k, v := range extractResponseParameters(r, whitelist) {
				opseg.GetAWS()[strings.ToLower(addUnderScoreBetweenWords(k))] = v
			}

			opseg.GetAWS()["region"] = r.ClientInfo.SigningRegion
			opseg.GetAWS()["operation"] = r.Operation.Name
			opseg.GetAWS()["retries"] = r.RetryCount
			opseg.GetAWS()[RequestIDKey] = r.RequestID

			if r.HTTPResponse != nil {
				opseg.GetHTTP().GetResponse().Status = r.HTTPResponse.StatusCode
				opseg.GetHTTP().GetResponse().ContentLength = int(r.HTTPResponse.ContentLength)

				if extendedRequestID := r.HTTPResponse.Header.Get(S3ExtendedRequestIDHeaderKey); extendedRequestID != "" {
					opseg.GetAWS()[ExtendedRequestIDKey] = extendedRequestID
				}
			}

			if request.IsErrorThrottle(r.Error) {
				opseg.Throttle = true
			}

			opseg.Unlock()
			opseg.Close(r.Error)
		},
	}
}

func parseWhitelistJSON(filename string) []byte {
	if filename != "" {
		readBytes, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Errorf("Error occurred while reading customized AWS whitelist JSON file. %v \nReverting to default AWS whitelist JSON file.", err)
		} else {
			return readBytes
		}
	}

	defaultBytes, err := resources.Asset("resources/AWSWhitelist.json")
	if err != nil {
		panic(err)
	}
	return defaultBytes
}

func keyValue(r interface{}, tag string) interface{} {
	v := reflect.ValueOf(r)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		log.Errorf("keyValue only accepts structs; got %T", v)
	}
	typ := v.Type()
	for i := 1; i < v.NumField(); i++ {
		if typ.Field(i).Name == tag {
			return v.Field(i).Interface()
		}
	}
	return nil
}

func addUnderScoreBetweenWords(name string) string {
	var buffer bytes.Buffer
	for i, char := range name {
		if unicode.IsUpper(char) && i != 0 {
			buffer.WriteRune('_')
		}
		buffer.WriteRune(char)
	}
	return buffer.String()
}

func (j *jsonMap) data() interface{} {
	if j == nil {
		return nil
	}
	return j.object
}

func (j *jsonMap) search(keys ...string) *jsonMap {
	var object interface{}
	object = j.data()

	for target := 0; target < len(keys); target++ {
		if mmap, ok := object.(map[string]interface{}); ok {
			object, ok = mmap[keys[target]]
			if !ok {
				return nil
			}
		} else {
			return nil
		}
	}
	return &jsonMap{object}
}

func (j *jsonMap) children() ([]interface{}, error) {
	if slice, ok := j.data().([]interface{}); ok {
		return slice, nil
	}
	return nil, errors.New("cannot get corresponding items for given aws whitelisting json file")
}

func (j *jsonMap) childrenMap() (map[string]interface{}, error) {
	if mmap, ok := j.data().(map[string]interface{}); ok {
		return mmap, nil
	}
	return nil, errors.New("cannot get corresponding items for given aws whitelisting json file")
}

func extractRequestParameters(r *request.Request, whitelist *jsonMap) map[string]interface{} {
	valueMap := make(map[string]interface{})

	extractParameters("request_parameters", requestKeyword, r, whitelist, valueMap)
	extractDescriptors("request_descriptors", requestKeyword, r, whitelist, valueMap)

	return valueMap
}

func extractResponseParameters(r *request.Request, whitelist *jsonMap) map[string]interface{} {
	valueMap := make(map[string]interface{})

	extractParameters("response_parameters", responseKeyword, r, whitelist, valueMap)
	extractDescriptors("response_descriptors", responseKeyword, r, whitelist, valueMap)

	return valueMap
}

func extractParameters(whitelistKey string, rType int, r *request.Request, whitelist *jsonMap, valueMap map[string]interface{}) {
	params := whitelist.search("services", r.ClientInfo.ServiceName, "operations", r.Operation.Name, whitelistKey)
	if params != nil {
		children, err := params.children()
		if err != nil {
			log.Errorf("failed to get values for aws attribute: %v", err)
			return
		}
		for _, child := range children {
			if child != nil {
				var value interface{}
				if rType == requestKeyword {
					value = keyValue(r.Params, child.(string))
				} else if rType == responseKeyword {
					value = keyValue(r.Data, child.(string))
				}
				if (value != reflect.Value{}) {
					valueMap[child.(string)] = value
				}
			}
		}
	}
}

func extractDescriptors(whitelistKey string, rType int, r *request.Request, whitelist *jsonMap, valueMap map[string]interface{}) {
	responseDtr := whitelist.search("services", r.ClientInfo.ServiceName, "operations", r.Operation.Name, whitelistKey)
	if responseDtr != nil {
		items, err := responseDtr.childrenMap()
		if err != nil {
			log.Errorf("failed to get values for aws attribute: %v", err)
			return
		}
		for k := range items {
			descriptorMap, _ := whitelist.search("services", r.ClientInfo.ServiceName, "operations", r.Operation.Name, whitelistKey, k).childrenMap()
			if rType == requestKeyword {
				insertDescriptorValuesIntoMap(k, r.Params, descriptorMap, valueMap)
			} else if rType == responseKeyword {
				insertDescriptorValuesIntoMap(k, r.Data, descriptorMap, valueMap)
			}
		}
	}
}

func descriptorType(descriptorMap map[string]interface{}) string {
	var typeValue string
	if (descriptorMap["map"] != nil) && (descriptorMap["get_keys"] != nil) {
		typeValue = "map"
	} else if (descriptorMap["list"] != nil) && (descriptorMap["get_count"] != nil) {
		typeValue = "list"
	} else if descriptorMap["value"] != nil {
		typeValue = "value"
	} else {
		log.Error("Missing keys in request / response descriptors in AWS whitelist JSON file.")
	}
	return typeValue
}

func insertDescriptorValuesIntoMap(key string, data interface{}, descriptorMap map[string]interface{}, valueMap map[string]interface{}) {
	descriptorType := descriptorType(descriptorMap)
	if descriptorType == "map" {
		var keySlice []interface{}
		m := keyValue(data, key)
		val := reflect.ValueOf(m)
		if val.Kind() == reflect.Map {
			for _, key := range val.MapKeys() {
				keySlice = append(keySlice, key.Interface())
			}
		}
		if descriptorMap["rename_to"] != nil {
			valueMap[descriptorMap["rename_to"].(string)] = keySlice
		} else {
			valueMap[strings.ToLower(key)] = keySlice
		}
	} else if descriptorType == "list" {
		var count int
		l := keyValue(data, key)
		val := reflect.ValueOf(l)
		count = val.Len()

		if descriptorMap["rename_to"] != nil {
			valueMap[descriptorMap["rename_to"].(string)] = count
		} else {
			valueMap[strings.ToLower(key)] = count
		}
	} else if descriptorType == "value" {
		val := keyValue(data, key)
		if descriptorMap["rename_to"] != nil {
			valueMap[descriptorMap["rename_to"].(string)] = val
		} else {
			valueMap[strings.ToLower(key)] = val
		}
	}
}
