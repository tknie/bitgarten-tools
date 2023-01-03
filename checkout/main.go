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

type processStep uint

const timeParseFormat = "2006-01-02 15:04:05 -0700 MST"
const fileTimeFormat = "20060201-150405"

const (
	begin processStep = iota
	analyzeDoublikats
	listDuplikats
	listDuplikatsRead
	initialize
	readStream
	delete
	deleteEnd
	end
)

var processSteps = []string{"Begin", "analyze", "list", "list read",
	"init", "read stream", "delete", "delete ET", "end"}

func (cc processStep) code() [2]byte {
	var code [2]byte
	codeConst := []byte(processSteps[cc])
	copy(code[:], codeConst[0:2])
	return code
}

func (cc processStep) command() string {
	return processSteps[cc]
}

type checker struct {
	conn      *adabas.Connection
	read      *adabas.ReadRequest
	list      *adabas.ReadRequest
	adabas    *adabas.Adabas
	directory string
	limit     uint64
	found     uint64
	created   uint64
	empty     uint64
	step      processStep
}

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

	err := initLogLevelWithFile("checkout.log", level)
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
	var dbidParameter string
	var mapFnrParameter int
	var limit int
	var directory string
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	flag.StringVar(&dbidParameter, "d", "23", "Map repository Database id")
	flag.IntVar(&mapFnrParameter, "f", 4, "Map repository file number")
	flag.IntVar(&limit, "l", 10, "Maximum records to read (0 is all)")
	flag.StringVar(&directory, "D", "", "Directory storing files to")
	flag.Parse()

	if directory == "" {
		fmt.Println("Please enter directory ...")
		flag.Usage()
		return
	}
	if fi, err := os.Stat(directory); err != nil {
		fmt.Println("Error opening directory ..." + directory + " : " + err.Error())
		flag.Usage()
		return
	} else {
		if !fi.IsDir() {
			fmt.Println("Please enter directory, not file ...")
			flag.Usage()
			return
		}
	}

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

	// if  {
	// 	fmt.Println("File name option is required")
	// 	flag.Usage()
	// 	return
	// }
	fmt.Printf("Connect to map repository %s/%d\n", dbidParameter, mapFnrParameter)

	id := adabas.NewAdabasID()
	a, err := adabas.NewAdabas(dbidParameter, id)
	if err != nil {
		fmt.Println("Adabas target generation error", err)
		return
	}
	adabas.AddGlobalMapRepository(a.URL, adabas.Fnr(mapFnrParameter))
	defer adabas.DelGlobalMapRepository(a.URL, adabas.Fnr(mapFnrParameter))
	c := &checker{adabas: a, limit: uint64(limit), step: initialize, directory: directory}
	err = c.checkoutOriginals()
	if err != nil {
		fmt.Println("Error anaylzing douplikats", err)
	}
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

func (checker *checker) checkoutOriginals() (err error) {
	checker.step = analyzeDoublikats
	checker.conn, err = adabas.NewConnection("acj;map")
	if err != nil {
		return err
	}
	defer checker.conn.Close()
	checker.read, err = checker.conn.CreateMapReadRequest("PictureMetadata")
	if err != nil {
		checker.conn.Close()
		return err
	}
	checker.read.Limit = checker.limit
	err = checker.read.QueryFields("PictureName,Option,ExifTaken,ExifOrigTime")
	if err != nil {
		checker.conn.Close()
		return err
	}
	counter := uint64(0)
	output := func() {
		fmt.Printf("%s Picture counter=%d created=%d found=%d empty=%d -> %s\n",
			time.Now().Format(timeFormat), counter, checker.created, checker.found, checker.empty, checker.step.command())
	}
	stop := schedule(output, 15*time.Second)
	_, err = checker.read.ReadLogicalWithStream("Option=original", func(record *adabas.Record, x interface{}) error {
		checker.step = readStream

		// fmt.Printf("quantity=%03d -> %s\n", record.Quantity, record.HashFields["ChecksumPicture"])
		err = checker.writeFile(record)
		if err != nil {
			return err
		}
		counter++
		return nil
	}, nil)
	if err != nil {
		fmt.Printf("Error checking descriptor quantity for ChecksumPicture: %v\n", err)
		panic("Read error " + err.Error())
	}
	stop <- true
	fmt.Printf("There are %06d records -> %d found and %d created, %d empty\n",
		counter, checker.found, checker.created, checker.empty)
	return nil
}

func (checker *checker) writeFile(record *adabas.Record) (err error) {
	p := checker.directory

	// new mtime
	newAtime := time.Date(1980, time.January, 1, 10, 00, 00, 0, time.UTC)
	newMtime := time.Date(1980, time.January, 1, 10, 00, 00, 0, time.UTC)

	t := strings.Trim(record.HashFields["ExifTaken"].String(), " ")
	if t != "" {

		exifTime, tErr := time.Parse(timeParseFormat, t)
		if tErr != nil {
			fmt.Println("Input:", t, "Output:", exifTime)
			fmt.Println("error", tErr)
		} else {
			newAtime = exifTime
			newMtime = exifTime
		}
		p = fmt.Sprintf("%s%s%s", p, string(os.PathSeparator), newAtime.Format(fileTimeFormat))
	} else {
		p += path.Dir(record.HashFields["PictureName"].String())
		p = strings.ReplaceAll(p, "../", "/")
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		err = os.MkdirAll(p, os.ModePerm)
		if err != nil {
			return err
		}
	}
	n := path.Base(record.HashFields["PictureName"].String())
	f := p + string(os.PathSeparator) + n
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		checker.found++
		if err != nil {
			return err
		}
		return nil
	}
	checker.step = listDuplikats
	if checker.list == nil {
		checker.list, err = checker.conn.CreateMapReadRequest(&store.PictureData{})
		if err != nil {
			checker.conn.Close()
			return
		}
		err = checker.list.QueryFields("ChecksumPicture,Media")
		if err != nil {
			checker.conn.Close()
			return
		}

	}
	result, err := checker.list.ReadISN(record.Isn)
	if err != nil {
		fmt.Printf("Error checking descriptor quantity for ChecksumPicture: %v\n", err)
		panic("Read error " + err.Error())
	}
	if len(result.Data) != 1 {
		panic("Result read of ISN")
	}
	data := result.Data[0].(*store.PictureData)
	if len(data.Media) == 0 {
		fmt.Println("Stored data empty :", record.HashFields["PictureName"].String())
		checker.empty++
		delRequest, delErr := checker.conn.CreateMapDeleteRequest("PictureMetadata")
		if delErr != nil {
			fmt.Println("Delete err", delErr)
			return nil
		}
		delRequest.Delete(record.Isn)
		delRequest.EndTransaction()
		return nil
	}
	file, err := os.OpenFile(f, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	file.Write(data.Media)

	// set new mtime
	err = os.Chtimes(f, newAtime, newMtime)
	if err != nil {
		fmt.Println(err)
		return
	}
	checker.created++

	return nil
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
