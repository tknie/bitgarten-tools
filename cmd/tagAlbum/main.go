/*
* Copyright © 2018-2023 private, Darmstadt, Germany and/or its licensors
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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"tux-lobload/sql"

	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	level := zapcore.ErrorLevel
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("tagAlbum.log", level)
	if err != nil {
		fmt.Println("Error initialize logging")
		os.Exit(255)
	}
}

func initLogLevelWithFile(fileName string, level zapcore.Level) (err error) {
	p := os.Getenv("LOGPATH")
	if p == "" {
		p = "."
	}
	name := p + string(os.PathSeparator) + fileName

	rawJSON := []byte(`{
		 "level": "error",
		 "encoding": "console",
		 "outputPaths": [ "loadpicture.log"],
		 "errorOutputPaths": ["stderr"],
		 "encoderConfig": {
		   "messageKey": "message",
		   "levelKey": "level",
		   "levelEncoder": "lowercase"
		 }
	   }`)

	var cfg zap.Config
	if err := json.Unmarshal(rawJSON, &cfg); err != nil {
		fmt.Println("Error initialize logging (json)")
		os.Exit(255)
	}
	cfg.Level.SetLevel(level)
	cfg.OutputPaths = []string{name}
	logger, err := cfg.Build()
	if err != nil {
		fmt.Println("Error initialize logging (build)")
		os.Exit(255)
	}
	defer logger.Sync()

	sugar := logger.Sugar()

	sugar.Infof("Start logging with level %s", level)
	log.Log = sugar
	log.SetDebugLevel(level == zapcore.DebugLevel)

	return
}

func main() {
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
	listSource := false
	flag.BoolVar(&listSource, "l", false, "List source Albums")
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
	sourceUrl := os.Getenv("POSTGRES_URL")
	pwd := os.Getenv("POSTGRES_PASSWORD")
	connSource, err := sql.Connect(sourceUrl, pwd)
	if err != nil {
		fmt.Println("Error creating connection:", err)
		fmt.Println("Set POSTGRES_URL and/or POSTGRES_PASSWORD to define remote database")
		return
	}

	if listSource {
		err = connSource.ListAlbums()
		if err != nil {
			fmt.Println("List albums error:", err)
			return
		}
		return
	}

	albums, err := connSource.GetAlbums()
	if err != nil {
		fmt.Println("Error reading albums:", err)
		return
	}
	log.Log.Debugf("Received Albums count = %d", len(albums))
	for _, a := range albums {
		log.Log.Debugf("Work on Album -> %s", a.Title)
		if a.Title != "New Album" {
			a, err = connSource.ReadAlbum(a.Title)
			if err != nil {
				fmt.Println("Error reading album:", err)
				return
			}
			a.Display()
			id, err := connSource.Open()
			if err != nil {
				fmt.Println("Error opening:", err)
				return
			}
			for _, p := range a.Pictures {
				fmt.Println(p.Description + " " + p.ChecksumPicture)
				list := [][]any{{
					p.ChecksumPicture,
					"bitgarten",
				}}
				input := &common.Entries{
					Fields: []string{
						"checksumpicture",
						"tagname",
					},
					Values: list}
				err = id.Insert("picturetags", input)
				if err != nil {
					fmt.Println("Error inserting:", err)
					return
				}
			}

		}
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
