// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package main

import (
	"log"

	autospotting "github.com/AutoSpotting/AutoSpotting/core"
)

// Version represents the build version being used
var Version = "number missing"

func main() {
	conf := autospotting.Config{
		Version: Version,
	}
	autospotting.ParseConfig(&conf)

	log.Println("Starting autospotting agent, build", Version)
	log.Printf("Configuration flags: %#v", conf)

	autospotting.Run(&conf)

	log.Println("Execution completed, nothing left to do")
}
