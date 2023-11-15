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
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
	"tux-lobload/sql"

	"github.com/tknie/adabas-go-api/adatypes"
	"github.com/tknie/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const timeFormat = "2006-01-02 15:04:05"

var hostname string
var insertAlbum = false
var albumid = 1

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

	adatypes.Central.Log = sugar
	log.Log = sugar
	log.Log.Infof("Start logging with level %s", level)

	return
}

func main() {
	var pictureDirectory string
	var filter string
	var binarySize int64
	var shortenPath bool
	var nrThreadReader int
	var nrThreadStorer int
	var fileName string
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	flag.StringVar(&pictureDirectory, "D", "", "Directory of picture to be imported")
	flag.IntVar(&nrThreadReader, "t", 5, "Threads preparing pictures")
	flag.IntVar(&nrThreadStorer, "T", 5, "Threads storing pictures")
	flag.StringVar(&filter, "F", ".*@eadir.*,.*/._[^/]*", "Comma-separated list of regular expression which may excluded")
	flag.BoolVar(&insertAlbum, "A", false, "Insert Albums")
	flag.IntVar(&albumid, "a", 1, "Album ID to add pictures")
	flag.BoolVar(&shortenPath, "s", false, "Shortend directory to last name only")
	flag.StringVar(&fileName, "i", "", "File name for single picture store")
	flag.Int64Var(&binarySize, "b", 50000000, "Maximum binary blob size")
	flag.Parse()

	if pictureDirectory == "" && fileName == "" {
		fmt.Println("Picture directory option is required")
		flag.Usage()
		return
	}

	for i := 0; i < nrThreadReader; i++ {
		go StoreWorker()
	}
	for i := 0; i < nrThreadStorer; i++ {
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
			log.Log.Fatalf("Regular expression error (%s): %v", r, err)
		}
		regs = append(regs, reg)
	}

	defer writeMemProfile(*memprofile)
	sql.StartStats()
	start := time.Now()
	switch {
	case fileName != "":
		fmt.Printf("Store file '%s' to album id %d\n", fileName, albumid)
		suffix := fileName[strings.LastIndex(fileName, ".")+1:]
		suffix = strings.ToLower(suffix)
		storeFile(fileName, suffix)
		time.Sleep(1 * time.Minute)
	case pictureDirectory != "":
		storeDirectory(pictureDirectory, regs)
	}
	wgStore.Wait()

	sql.EndStats()
	fmt.Printf("%s used %v\n", time.Now().Format(timeFormat), time.Since(start))
	for i := 0; i < nrThreadReader; i++ {
		sql.StopWorker()
	}
}

func storeDirectory(pictureDirectory string, regs []*regexp.Regexp) {
	if pictureDirectory != "" {

		if insertAlbum {
			di, err := sql.CreateConnection()
			if err != nil {
				fmt.Println("Error connecting:", err)
				return
			}
			dir := filepath.Base(pictureDirectory)
			albumid, err = di.InsertNewAlbum(dir)
			if err != nil {
				fmt.Println("Error inserting album:", err)
				return
			}
		}
		fmt.Printf("%s Loading path %s\n", time.Now().Format(timeFormat), pictureDirectory)
		err := filepath.Walk(pictureDirectory, func(path string, info os.FileInfo, err error) error {
			if info == nil || info.IsDir() {
				log.Log.Infof("Info empty or dir: %s", path)
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
			if err != nil {
				// return fmt.Errorf("error storing file: %v", err)
				sql.IncErrorFile(err, path)
			}
			switch suffix {
			case "jpg", "jpeg", "gif", "m4v", "mov", "mp4", "webm":
				queueStoreFileInAlbumID(path, albumid)
				if err != nil {
					// return fmt.Errorf("error storing file: %v", err)
					sql.IncErrorFile(err, path)
				}
				ti.IncDone()
			default:
				log.Log.Debugf("Suffix unknown: %s", suffix)
				sql.IncSkipped()
			}
			return nil
		})
		if err != nil {
			fmt.Println("Abort/Error during file walk:", err)
			return
		}
	}

}

func storeFile(path, suffix string) error {
	ti := sql.IncChecked()
	switch suffix {
	case "jpg", "jpeg", "gif", "m4v", "mov", "mp4", "webm":
		queueStoreFileInAlbumID(path, albumid)
		ti.IncDone()
	default:
		log.Log.Debugf("Suffix unknown: %s", suffix)
		sql.IncSkipped()
	}
	return nil
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
