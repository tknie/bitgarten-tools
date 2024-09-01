/*
* Copyright © 2018-2024 private, Darmstadt, Germany and/or its licensors
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

	"github.com/tknie/bitgarten-tools/sql"
	"github.com/tknie/bitgarten-tools/tools"

	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

const description = `This tool checks checksum of all picture entries and compares 
it with the database checksumpicture data content.

`

func main() {
	var limit int
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	err := tools.InitLogLevelWithFile("checkMedia.log")
	if err != nil {
		fmt.Printf("Error initialzing logging: %v\n", err)
		return
	}
	flag.Usage = func() {
		fmt.Print(description)
		fmt.Println("Default flags:")
		flag.PrintDefaults()
	}
	flag.IntVar(&limit, "l", 10, "Maximum records to read (0 is all)")
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
	errCount := uint32(0)
	tools.InitCheck(func(pic *sql.Picture, status string) {
		fmt.Println(status)
		errCount++
	})
	connSource, err := sql.DatabaseConnect()
	if err != nil {
		fmt.Printf("Error connecting URL: %v", err)
		return
	}
	counter := uint64(0)
	err = connSource.ReadMedia(uint32(limit), func(search *common.Query, result *common.Result) error {
		p := &sql.Picture{}
		pic := result.Data.(*sql.Picture)
		*p = *pic
		counter++
		log.Log.Debugf("Received record %s %s", pic.ChecksumPicture, pic.Sha256checksum)
		tools.CheckPicture(p)
		if counter%1000 == 0 {
			fmt.Printf("Mediacheck working, checked %10d entries\n", counter)
		}
		// fmt.Println(pic.ChecksumPicture)
		return nil
	})
	if err != nil {
		fmt.Println("Got return check media", err)
	}
	tools.CheckMediaWait()
	if errCount > 0 {
		fmt.Printf("Working ended with errors/warnings, checked %d\n", counter)
	} else {
		fmt.Printf("Working ended successfully, checked %d\n", counter)
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
