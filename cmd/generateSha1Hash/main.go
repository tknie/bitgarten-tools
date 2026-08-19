/*
* Copyright © 2018-2026 private, Darmstadt, Germany and/or its licensors
*
* SPDX-License-Identifier: Apache-2.0
*
*   Licensed under the Apache License, Version 2.0 (the "License");
*   you may not use this file except in compliance with the License.
*   You may obtain a copy of the License at
*
*       http://www.apache.org/licenses/LICENSE-2.0
*
*   Unless required by applicable law or agreed to in writing, software
*   distributed under the License is distributed on an "AS IS" BASIS,
*   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*   See the License for the specific language governing permissions and
*   limitations under the License.
*
 */

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/tknie/bitgartentools"
	"github.com/tknie/bitgartentools/tools"
	"github.com/tknie/log"
	"github.com/tknie/services"
)

const description = `This tool checks checksum of all picture entries and compares 
it with the database checksumpicture data content.

`

func init() {
	services.ServerMessage("Start Check Media application %s (build at %s)", bitgartentools.BuildVersion, bitgartentools.BuildDate)

	err := log.InitZapLogWithFilename("checkMedia.log")
	if err != nil {
		fmt.Printf("Error initialzing logging: %v\n", err)
		return
	}
}

func main() {
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	flag.Usage = func() {
		fmt.Print(description)
		fmt.Println("Default flags:")
		flag.PrintDefaults()
	}
	var limit int
	var deleted bool
	flag.IntVar(&limit, "l", 10, "Maximum records to read (0 is all)")
	flag.BoolVar(&deleted, "d", false, "Include generate hash for deleted pictures")
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic("could not create CPU profile: " + err.Error())
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			panic("could not start CPU profile: " + err.Error())
		}
		defer pprof.StopCPUProfile()
	}

	err := tools.GenerateImageHash(&tools.ImageHashParameter{
		Deleted:   deleted,
		PreFilter: "",
		Limit:     limit,
	})
	fmt.Printf("Finished generating SHA1 hash for pictures with error: %v\n", err)

	defer writeMemProfile(*memprofile)
}

func writeMemProfile(file string) {
	if file != "" {
		f, err := os.Create(file)
		if err != nil {
			panic("could not create memory profile: " + err.Error())
		}
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			panic("could not write memory profile: " + err.Error())
		}
		defer f.Close()
		fmt.Println("Memory profile written")
	}

}
