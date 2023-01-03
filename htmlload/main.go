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
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
	"tux-lobload/store"

	"github.com/tknie/adabas-go-api/adabas"
	"github.com/tknie/adabas-go-api/adatypes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var hostname string
var timeFormat = "2006-01-02 15:04:05"

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

	err := initLogLevelWithFile("picload.log", level)
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

func schedule(what func(), delay time.Duration) chan bool {
	stop := make(chan bool)

	go func() {
		for {
			what()
			select {
			case <-time.After(delay):
			case <-stop:
				return
			}
		}
	}()

	return stop
}

func main() {
	var fileName string
	var pictureDirectory string
	var dbidParameter string
	var mapFnrParameter int
	var filter string
	var deleteIsn int
	var binarySize int
	var verify bool
	var update bool
	var checksumRun bool
	var shortenName bool
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	flag.StringVar(&fileName, "p", "", "File name of picture to be imported")
	flag.StringVar(&pictureDirectory, "D", "", "Directory of picture to be imported")
	flag.StringVar(&dbidParameter, "d", "23", "Map repository Database id")
	flag.StringVar(&filter, "f", "@eadir", "Comma-separated list of parts which may excluded")
	flag.IntVar(&mapFnrParameter, "f", 4, "Map repository file number")
	flag.BoolVar(&verify, "v", false, "Verify data")
	flag.BoolVar(&update, "u", false, "Update data")
	flag.BoolVar(&shortenName, "s", false, "Shorten directory name")
	flag.BoolVar(&checksumRun, "c", false, "Checksum run, no data load")
	flag.IntVar(&deleteIsn, "r", -1, "Delete ISN image")
	flag.IntVar(&binarySize, "b", 50000000, "Maximum binary blob size")
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

	if !verify && (fileName == "" && pictureDirectory == "" && deleteIsn == -1) {
		fmt.Println("File name option is required")
		flag.Usage()
		return
	}
	fmt.Printf("Connect to map repository %s/%d\n", dbidParameter, mapFnrParameter)

	id := adabas.NewAdabasID()
	a, err := adabas.NewAdabas(dbidParameter, id)
	if err != nil {
		fmt.Println("Adabas target generation error", err)
		return
	}
	adabas.AddGlobalMapRepository(a.URL, adabas.Fnr(mapFnrParameter))
	defer adabas.DelGlobalMapRepository(a.URL, adabas.Fnr(mapFnrParameter))
	//adabas.DumpGlobalMapRepositories()

	ps, perr := store.InitStorePictureBinary(!shortenName)
	if perr != nil {
		fmt.Println("Adabas connection error", perr)
		panic("Adabas communication error")
	}
	defer ps.Close()

	ps.ChecksumRun = checksumRun
	ps.MaxBlobSize = int64(binarySize)

	if deleteIsn > 0 {
		err := ps.DeleteIsn(a, adatypes.Isn(deleteIsn))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting Isn=%d: %v", deleteIsn, err)
		} else {
			fmt.Printf("Isn=%d successfull deleted ....\n", deleteIsn)
		}
		return
	}

	if fileName != "" {
		err = filepath.Walk(fileName, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			// fmt.Println("Check", path)
			if strings.HasSuffix(strings.ToLower(path), "index.html") {
				//fmt.Println("Found index file", path)
				return ps.LoadIndex(!update, path, a)
			}
			// if strings.HasSuffix(strings.ToLower(path), ".jpg") {
			// 	fmt.Println("Load Jpeg", path)
			// 	return LoadPicture(path, a)
			// }
			// if strings.HasSuffix(strings.ToLower(path), ".m4v") {
			// 	fmt.Println("Load Movie", path)
			// 	return loadMovie(path, a)
			// }
			return nil
		})
		if err != nil {
			fmt.Println("Error walking path", err)
		}
		// fmt.Println("End of lob load")

	}
	if pictureDirectory != "" {
		output := func() {
			fmt.Printf("%s Picture directory checked=%d loaded=%d found=%d too big=%d errors=%d\n",
				time.Now().Format(timeFormat), ps.Checked, ps.Loaded, ps.Found, ps.ToBig, ps.NrErrors)
		}

		fmt.Printf("%s Loading path %s\n", time.Now().Format(timeFormat), pictureDirectory)
		stop := schedule(output, 5*time.Second)
		err = filepath.Walk(pictureDirectory, func(path string, info os.FileInfo, err error) error {
			if info == nil || info.IsDir() {
				adatypes.Central.Log.Infof("Info empty or dir: %s", path)
				return nil
			}
			suffix := path[strings.LastIndex(path, ".")+1:]
			suffix = strings.ToLower(suffix)
			switch suffix {
			case "jpg", "jpeg", "gif", "m4v", "mov", "mp4", "webm":
				adatypes.Central.Log.Debugf("Checking picture file: %s", path)
				err = ps.LoadPicture(!update, path, a)
				if err != nil {
					adatypes.Central.Log.Debugf("Loaded %s with error=%v", ps, err)
					fmt.Fprintln(os.Stderr, "Error loading picture", path, ":", err)
					if strings.HasPrefix(err.Error(), "File tooo big") {
						ps.ToBig++
					} else {
						if n, ok := ps.Errors[err.Error()]; ok {
							ps.Errors[err.Error()] = n + 1
						} else {
							ps.Errors[err.Error()] = 1
						}
						ps.NrErrors++
					}
				}
			default:
			}
			return nil
		})
		adatypes.Central.Log.Debugf("File walkd fail: %v", err)
		stop <- true
		fmt.Printf("%s Done Picture directory checked=%d loaded=%d found=%d too big=%d errors=%d\n",
			time.Now().Format(timeFormat), ps.Checked, ps.Loaded, ps.Found, ps.ToBig, ps.NrErrors)
		for e, n := range ps.Errors {
			fmt.Println(e, ":", n)
		}
	}
	if verify {
		fmt.Printf("%s Start verifying database picture content\n", time.Now().Format(timeFormat))
		err = store.VerifyPicture("PictureData", fmt.Sprintf("%s,%d", dbidParameter, mapFnrParameter))
		if err != nil {
			fmt.Printf("%s Error during verify of database picture content: %v\n", time.Now().Format(timeFormat), err)
			return
		}
		fmt.Printf("%s finished verify of database picture content\n", time.Now().Format(timeFormat))
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
