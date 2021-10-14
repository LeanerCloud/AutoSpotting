// Copyright (c) 2016-2021 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"sync"

	"github.com/davecgh/go-spew/spew"
)

// The key in this map is the instance ID, useful for quick retrieval of
// instance attributes.
type instanceMap map[string]*instance

type instanceManager struct {
	sync.RWMutex
	catalog instanceMap
}

type instances interface {
	add(inst *instance)
	get(string) *instance
	count() int
	count64() int64
	make()
	instances() <-chan *instance
	dump() string
}

type acceptableInstance struct {
	instanceTI instanceTypeInformation
	price      float64
}

type instanceTypeInformation struct {
	instanceType             string
	vCPU                     int
	PhysicalProcessor        string
	GPU                      int
	pricing                  prices
	memory                   float32
	virtualizationTypes      []string
	hasInstanceStore         bool
	instanceStoreDeviceSize  float32
	instanceStoreDeviceCount int
	instanceStoreIsSSD       bool
	hasEBSOptimization       bool
	EBSThroughput            float32
}

func makeInstances() instances {
	return &instanceManager{catalog: instanceMap{}}
}

func makeInstancesWithCatalog(catalog instanceMap) instances {
	return &instanceManager{catalog: catalog}
}

func (is *instanceManager) dump() string {
	is.RLock()
	defer is.RUnlock()
	return spew.Sdump(is.catalog)
}
func (is *instanceManager) make() {
	is.Lock()
	is.catalog = make(instanceMap)
	is.Unlock()
}

func (is *instanceManager) add(inst *instance) {
	if inst == nil {
		return
	}

	is.Lock()
	defer is.Unlock()
	is.catalog[*inst.InstanceId] = inst
}

func (is *instanceManager) get(id string) (inst *instance) {
	is.RLock()
	defer is.RUnlock()
	return is.catalog[id]
}

func (is *instanceManager) count() int {
	is.RLock()
	defer is.RUnlock()

	return len(is.catalog)
}

func (is *instanceManager) count64() int64 {
	return int64(is.count())
}

func (is *instanceManager) instances() <-chan *instance {
	retC := make(chan *instance)
	go func() {
		is.RLock()
		defer is.RUnlock()
		defer close(retC)
		for _, i := range is.catalog {
			retC <- i
		}
	}()

	return retC
}
