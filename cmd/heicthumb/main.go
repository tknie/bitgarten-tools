/*
* Copyright © 2023-2024 private, Darmstadt, Germany and/or its licensors
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
	"tux-lobload/tools"
)

func main() {

	tools.InitLogLevelWithFile("heicthumb.log")
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
	var chksum string
	var storeData bool
	var title string
	var fromDate string
	var toDate string
	var createThumbnail bool
	var album string
	var scale bool
	var scaleRange int

	flag.StringVar(&chksum, "c", "", "Search for picture id checksum")
	flag.StringVar(&title, "t", "", "Search for picture title")
	flag.StringVar(&album, "a", "", "Search for album title")
	flag.StringVar(&fromDate, "F", "", "Search for picture created from this date (format 2001-12-30)")
	flag.StringVar(&toDate, "T", "", "Search for picture created before this date including (format 2001-12-30)")
	flag.IntVar(&scaleRange, "m", 1280, "Max width or height image size")
	flag.BoolVar(&storeData, "S", false, "Store data to database")
	flag.BoolVar(&createThumbnail, "C", false, "Create thumbnails instead of search for similarity")
	flag.BoolVar(&scale, "s", false, "Scale for album")
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
	defer writeMemProfile(*memprofile)

	p := &tools.HeicThumbParameter{Commit: storeData, ChkSum: chksum,
		CreateThumbnail: createThumbnail, FromDate: fromDate, ToDate: toDate}
	if scale {
		p.Title = album
		p.ScaleRange = scaleRange
		p.HeicScale()
	} else {
		p.Title = title
		p.HeicThumb()
	}

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
