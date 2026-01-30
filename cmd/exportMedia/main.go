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
)

const description = `This tool exports all files into a directory
 `

func main() {
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	err := tools.InitLogLevelWithFile("exportMedia.log")
	if err != nil {
		fmt.Printf("Error initialzing logging: %v\n", err)
		return
	}
	flag.Usage = func() {
		fmt.Print(description)
		fmt.Println("Default flags:")
		flag.PrintDefaults()
	}
	var limit int
	json := false
	directory := ""
	markDelete := false
	workers := 2
	flag.IntVar(&limit, "l", 10, "Maximum records to read (0 is all)")
	flag.IntVar(&workers, "t", 2, "Maximum number of workers writing media")
	flag.BoolVar(&json, "j", false, "Output in JSON format")
	flag.BoolVar(&markDelete, "D", false, "Search include mark deleted")
	flag.StringVar(&directory, "d", "", "Write files to directory")
	flag.Parse()

	bitgartentools.InitTool("checkMedia", json)
	defer bitgartentools.FinalizeTool("checkMedia", json, err)

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

	tools.StartExport(workers)

	err = tools.ExportMedia(&tools.ExportMediaParameter{Limit: limit, MarkDelete: markDelete,
		Directory: directory})
	if err != nil {
		fmt.Println("Export Media error:", err)
	}
	fmt.Println("Export Media done")
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
