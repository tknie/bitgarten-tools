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
	"strings"

	"github.com/tknie/bitgarten-tools/sql"
	"github.com/tknie/bitgarten-tools/tools"

	"github.com/docker/go-units"
)

const description = `Load picture into SQL database. The given directory parameter
defines the location to be loaded.

`

func main() {
	tools.InitLogLevelWithFile("picloadql.log")
	var filter string
	var binarySize string
	var shortenPath bool
	var nrThreadReader int
	var nrThreadStorer int
	var fileName string
	var albumid int
	var insertAlbum bool
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	flag.IntVar(&nrThreadReader, "t", 5, "Threads preparing pictures")
	flag.IntVar(&nrThreadStorer, "T", 5, "Threads storing pictures")
	flag.StringVar(&filter, "F", ".*@eadir.*,.*/._[^/]*", "Comma-separated list of regular expression which may excluded")
	flag.BoolVar(&insertAlbum, "A", false, "Insert Albums")
	flag.IntVar(&albumid, "a", 1, "Album ID to add pictures")
	flag.BoolVar(&shortenPath, "s", false, "Shortend directory to last name only")
	flag.StringVar(&fileName, "i", "", "File name for single picture store")
	flag.StringVar(&binarySize, "b", "500MB", "Maximum binary blob size")
	flag.BoolVar(&sql.ExitOnError, "E", false, "Exit if an error happens")
	flag.Usage = func() {
		fmt.Print(description)
		fmt.Println("Default flags:")
		flag.PrintDefaults()
	}
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

	directories := flag.Args()
	sz, err := units.FromHumanSize(binarySize)
	if err != nil {
		fmt.Printf("Picture size option is not valid: %s\n", binarySize)
		flag.Usage()
		return
	}

	if len(directories) == 0 {
		e := os.Getenv("BITGARTEN_DIRECTORIES")
		if e != "" {
			directories = strings.Split(e, ",")
		}
	}

	if len(directories) == 0 && fileName == "" {
		fmt.Println("Picture directory option is required")
		flag.Usage()
		return
	}
	fmt.Println("Directories:", directories)
	tools.PicLoad(&tools.PicLoadParameter{NrThreadReader: nrThreadReader,
		NrThreadStorer: nrThreadStorer, MaxBlobSize: sz, Filter: filter,
		AlbumId: albumid, InsertAlbum: insertAlbum,
		ShortenPath: shortenPath, FileName: fileName,
		Directories: directories})
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
