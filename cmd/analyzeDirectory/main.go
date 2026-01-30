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
)

const description = `This tool checks found files in directory and analyze number of new or registered
media in the databases using the checksum.

`

func main() {
	var limit int
	json := false

	err := tools.InitLogLevelWithFile("analyzeDirectory.log")
	if err != nil {
		fmt.Printf("Error initialzing logging: %v\n", err)
		return
	}
	flag.Usage = func() {
		fmt.Print(description)
		fmt.Println("Default flags:")
		flag.PrintDefaults()
	}
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
	flag.IntVar(&limit, "l", 10, "Maximum records to read (0 is all)")
	flag.BoolVar(&json, "j", false, "Output in JSON format")
	flag.Parse()

	bitgartentools.InitTool("analyzeDirectory", json)
	defer bitgartentools.FinalizeTool("analyzeDirectory", json, err)

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
	defer writeMemProfile(*memprofile)

	directories := flag.Args()
	if len(directories) == 0 {
		directories, err = tools.EvaluatePictureDirectories()
		if err != nil {
			fmt.Println("Picture directory option is required")
			flag.Usage()
			return
		}
	}

	fmt.Println("Analyze directories:", directories)
	err = tools.AnalyzeDirectories(directories)
	log.Log.Debugf("Result analyzing directories: %v", err)
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
