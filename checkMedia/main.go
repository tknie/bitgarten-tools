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
	"sync"
	"tux-lobload/sql"

	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var hostname string
var checkPictureChannel = make(chan *sql.Picture, 10)
var stop = make(chan bool)
var wgCheck sync.WaitGroup

func init() {
	hostname, _ = os.Hostname()
	level := zapcore.ErrorLevel
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("checkMedia.log", level)
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
	var dbidParameter string
	var mapFnrParameter int
	var limit int
	var delete bool
	var validate bool
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	flag.StringVar(&dbidParameter, "d", "23", "Map repository Database id")
	flag.IntVar(&mapFnrParameter, "f", 4, "Map repository file number")
	flag.IntVar(&limit, "l", 10, "Maximum records to read (0 is all)")
	flag.BoolVar(&delete, "D", false, "Delete duplicate entries")
	flag.BoolVar(&validate, "V", false, "Validate large object entries")
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

	initCheck()
	sourceUrl := os.Getenv("POSTGRES_URL")
	pwd := os.Getenv("POSTGRES_PASSWORD")
	fmt.Println("Connect : " + sourceUrl)
	connSource, err := sql.Connect(sourceUrl, pwd)
	if err != nil {
		fmt.Printf("Error connecting URL: %v", err)
		return
	}
	counter := uint64(0)
	err = connSource.CheckMedia(func(search *common.Query, result *common.Result) error {
		p := &sql.Picture{}
		pic := result.Data.(*sql.Picture)
		*p = *pic
		counter++
		log.Log.Debugf("Received record %s %s", pic.ChecksumPicture, pic.Sha256checksum)
		checkPicture(p)
		// fmt.Println(pic.ChecksumPicture)
		return nil
	})
	if err != nil {
		fmt.Println("Got return check media", err)
	}
	wgCheck.Wait()
	fmt.Printf("Working ended successfully, checked %d\n", counter)
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

func initCheck() {
	for i := 0; i < 4; i++ {
		go pictureChecker()
	}
}

func checkPicture(pic *sql.Picture) {
	wgCheck.Add(1)
	checkPictureChannel <- pic
}

func pictureChecker() {
	for {
		select {
		case pic := <-checkPictureChannel:
			log.Log.Debugf("Checking record %s %s", pic.ChecksumPicture, pic.Sha256checksum)

			switch {
			case len(pic.Media) == 0:
				fmt.Println(pic.ChecksumPicture + " Media empty")
				log.Log.Debugf("Error record len %s %s", pic.ChecksumPicture, pic.Sha256checksum)
			case sql.CreateMd5(pic.Media) != pic.ChecksumPicture:
				fmt.Println(pic.ChecksumPicture + " md5 error")
				log.Log.Debugf("Error md5  %s", sql.CreateMd5(pic.Media))
			case sql.CreateSHA(pic.Media) != pic.Sha256checksum:
				fmt.Println(pic.ChecksumPicture + " sha error")
				log.Log.Debugf("Error sha  %s", sql.CreateSHA(pic.Media))
			}
			wgCheck.Done()
		case <-stop:
			return
		}
	}
}
