package main

import (
	"fmt"
	"log"
	"os"

	autospotting "github.com/cristim/autospotting/core"
)

// Main intefaces with the lambda event handler wrapper written
// in javascript.
func main() {
	fmt.Println("Starting agent...")

	if len(os.Args) != 2 {
		log.Fatal("The program needs a command line argument: ",
			os.Args[0], " <instances.json>")
	}
	instancesFile := os.Args[1]

	autospotting.Run(instancesFile)

	fmt.Println("Exiting main, nothing left to do")
}
