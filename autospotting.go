package main

import (
	"fmt"

	autospotting "github.com/cristim/autospotting/core"
)

// upstream data
// const instancesURL = "https://raw.githubusercontent.com/powdahound/ec2instances.info/master/www/instances.json"

// my Github fork
const instancesURL = "https://raw.githubusercontent.com/cristim/ec2instances.info/master/www/instances.json"

// Main intefaces with the lambda event handler wrapper written
// in javascript.
func main() {
	fmt.Println("Starting agent...")

	autospotting.Run(instancesURL)

	fmt.Println("Exiting main, nothing left to do")
}
