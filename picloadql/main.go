/*
* Copyright © 2018-2019 private, Darmstadt, Germany and/or its licensors
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
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
	"tux-lobload/sql"

	"github.com/tknie/adabas-go-api/adatypes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const timeFormat = "2006-01-02 15:04:05"

var hostname string

func init() {
	hostname, _ = os.Hostname()
	level := zapcore.ErrorLevel
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
		adatypes.Central.SetDebugLevel(true)
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("picloadql.log", level)
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

	sugar.Infof("Start logging with level", level)
	adatypes.Central.Log = sugar

	return
}

func main() {
	var pictureDirectory string
	var filter string
	var binarySize int64
	var insertAlbum bool
	var shortenPath bool
	var threadNr int
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	flag.StringVar(&pictureDirectory, "D", "", "Directory of picture to be imported")
	flag.IntVar(&threadNr, "T", 5, "Threads storing of pictures")
	flag.StringVar(&filter, "F", ".*@eadir.*,.*/._[^/]*", "Comma-separated list of regular expression which may excluded")
	flag.BoolVar(&insertAlbum, "A", false, "Insert Albums")
	flag.BoolVar(&shortenPath, "s", false, "Shortend directory to last name only")
	flag.Int64Var(&binarySize, "b", 50000000, "Maximum binary blob size")
	flag.Parse()

	if pictureDirectory == "" {
		fmt.Println("Picture directory option is required")
		flag.Usage()
		return
	}

	for i := 0; i < threadNr; i++ {
		go sql.InsertWorker()
	}
	MaxBlobSize = binarySize
	ShortPath = shortenPath

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
	regs := make([]*regexp.Regexp, 0)
	for _, r := range strings.Split(filter, ",") {
		reg, err := regexp.Compile(r)
		if err != nil {
			log.Fatalf("Regular expression error (%s): %v", r, err)
		}
		regs = append(regs, reg)
	}

	defer writeMemProfile(*memprofile)
	if pictureDirectory != "" {
		sql.StartStats()
		start := time.Now()

		con, err := sql.CreateConnection()
		if err != nil {
			fmt.Println("Error storing file", err)
			return
		}
		fmt.Println("Connecting ....")

		directory := path.Base(pictureDirectory)
		id := 1
		if insertAlbum {
			id, err = con.InsertNewAlbum(directory)
			if err != nil {
				fmt.Println("Error inserting album:", err)
				return
			}
		}
		fmt.Println("Load pictures for Album ID", id)

		fmt.Printf("%s Loading path %s\n", time.Now().Format(timeFormat), pictureDirectory)
		err = filepath.Walk(pictureDirectory, func(path string, info os.FileInfo, err error) error {
			if info == nil || info.IsDir() {
				adatypes.Central.Log.Infof("Info empty or dir: %s", path)
				return nil
			}
			for _, reg := range regs {
				if !checkQueryPath(reg, path) {
					return nil
				}
			}

			ti := sql.IncChecked()
			suffix := path[strings.LastIndex(path, ".")+1:]
			suffix = strings.ToLower(suffix)
			switch suffix {
			case "jpg", "jpeg", "gif", "m4v", "mov", "mp4", "webm":
				err = storeFile(con, path, id)
				if err != nil {
					// return fmt.Errorf("error storing file: %v", err)
					sql.IncErrorFile(err, path)
				}
			default:
				adatypes.Central.Log.Debugf("Suffix unknown: %s", suffix)
			}
			ti.IncDone()
			return nil
		})
		if err != nil {
			fmt.Println("Abort/Error during file walk:", err)
			return
		}

		sql.EndStats()
		fmt.Printf("%s used %v\n", time.Now().Format(timeFormat), time.Since(start))
	}
}

func checkQueryPath(reg *regexp.Regexp, path string) bool {
	return !reg.MatchString(path)
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
