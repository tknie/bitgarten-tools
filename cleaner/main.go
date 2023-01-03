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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"runtime/pprof"
	"time"
	"tux-lobload/store"

	"github.com/tknie/adabas-go-api/adabas"
	"github.com/tknie/adabas-go-api/adatypes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var hostname string

type deleter struct {
	re            *regexp.Regexp
	deleteRequest *adabas.DeleteRequest
	test          bool
	counter       uint64
}

var timeFormat = "2006-01-02 15:04:05"

type elementCounter struct {
	counter uint64
}

type validater struct {
	conn            *adabas.Connection
	read            *adabas.ReadRequest
	delete          *adabas.DeleteRequest
	list            *adabas.ReadRequest
	limit           uint64
	elementMap      map[int]*elementCounter
	checkedPicture  uint64
	okPictures      uint64
	failurePictures uint64
	emptyPictures   uint64
	unique          uint64
	deleteDuplikate uint64
	deleteEmpty     uint64
	test            bool
}

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

	err := initLogLevelWithFile("cleaner.log", level)
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
	var test bool
	var validate bool
	var query string
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	flag.StringVar(&dbidParameter, "d", "23", "Map repository Database id")
	flag.IntVar(&mapFnrParameter, "f", 4, "Map repository file number")
	flag.IntVar(&limit, "l", 10, "Maximum records to read (0 is all)")
	flag.BoolVar(&test, "t", false, "Dry run, don't change")
	flag.BoolVar(&validate, "v", false, "Validate uniquness of media content")
	flag.StringVar(&query, "q", "", "Filter for regexp query used to clean up")
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

	if query == "" && !validate {
		fmt.Println("Need to give exclude mask or enable validation!!!")
		return
	}

	if test {
		fmt.Println("Test mode ENABLED")
	}

	id := adabas.NewAdabasID()
	a, err := adabas.NewAdabas(dbidParameter, id)
	if err != nil {
		fmt.Println("Adabas target generation error", err)
		return
	}
	adabas.AddGlobalMapRepository(a.URL, adabas.Fnr(mapFnrParameter))
	defer adabas.DelGlobalMapRepository(a.URL, adabas.Fnr(mapFnrParameter))

	fmt.Printf("Connect to map repository %s/%d\n", dbidParameter, mapFnrParameter)
	if query != "" {
		d := &deleter{test: test}
		fmt.Println("Clear using exclude mask with: " + query)
		re, err := regexp.Compile(query)
		if err != nil {
			fmt.Println("Query error regexp:", err)
			return
		}
		d.re = re

		err = removeQueries(a, d, uint64(limit))
		if err != nil {
			fmt.Println("Error anaylzing douplikats", err)
		}
	}
	if validate {
		val := &validater{limit: uint64(limit), test: test, elementMap: make(map[int]*elementCounter)}
		val.analyzeDoublikats()
	}
}

func removeQuery(record *adabas.Record, x interface{}) error {
	fn := record.HashFields["PictureName"].String()
	de := x.(*deleter)
	found := de.re.MatchString(fn)
	if found {
		fmt.Println("Found :" + fn)
		if !de.test {
			de.deleteRequest.Delete(record.Isn)
			de.counter++
			if de.counter%100 == 0 {
				err := de.deleteRequest.EndTransaction()
				return err
			}
		}
	} else {
		//	fmt.Println("Ignore :" + fn)
	}
	return nil
}

func removeQueries(a *adabas.Adabas, de *deleter, limit uint64) error {
	conn, err := adabas.NewConnection("acj;map")
	if err != nil {
		return err
	}
	defer conn.Close()
	readCheck, rerr := conn.CreateMapReadRequest("PictureMetadata")
	if rerr != nil {
		conn.Close()
		return rerr
	}
	de.deleteRequest, err = conn.CreateMapDeleteRequest("PictureMetadata")
	if err != nil {
		conn.Close()
		return err
	}
	readCheck.Limit = limit
	rerr = readCheck.QueryFields("PictureName")
	if rerr != nil {
		conn.Close()
		return rerr
	}
	_, err = readCheck.ReadPhysicalSequenceStream(removeQuery, de)
	if err != nil {
		fmt.Printf("Error checking descriptor quantity for ChecksumPicture: %v\n", err)
		de.deleteRequest.BackoutTransaction()
		panic("Read error " + err.Error())
	}
	err = de.deleteRequest.EndTransaction()
	return err
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

func (validater *validater) analyzeDoublikats() (err error) {
	validater.conn, err = adabas.NewConnection("acj;map")
	if err != nil {
		return err
	}
	defer validater.conn.Close()
	if validater.read == nil {
		validater.read, err = validater.conn.CreateMapReadRequest("PictureMetadata")
		if err != nil {
			validater.conn.Close()
			return err
		}
		validater.read.Limit = validater.limit
		err = validater.read.QueryFields("ChecksumPicture,PictureName")
		if err != nil {
			validater.conn.Close()
			return err
		}
	}
	counter := uint64(0)
	output := func() {
		fmt.Printf("%s Picture counter=%d checked=%d ok=%d unique=%d failure=%d empty=%d del Dupli=%d del Empty=%d\n",
			time.Now().Format(timeFormat), counter, validater.checkedPicture,
			validater.okPictures, validater.unique, validater.failurePictures,
			validater.emptyPictures, validater.deleteDuplikate, validater.deleteEmpty)
	}
	stop := schedule(output, 15*time.Second)
	cursor, err := validater.read.HistogramByCursoring("ChecksumPicture")
	// result, err := validater.read.ReadLogicalByStream("ChecksumPicture", func(record *adabas.Record, x interface{}) error {
	// 	// fmt.Printf("quantity=%03d -> %s\n", record.Quantity, record.HashFields["ChecksumPicture"])
	// 	err = validater.listDuplikats(record.HashFields["ChecksumPicture"].String())
	// 	if err != nil {
	// 		return err
	// 	}
	// 	counter++
	// 	return nil
	// }, nil)
	if err != nil {
		fmt.Printf("Error checking descriptor quantity for ChecksumPicture: %v\n", err)
		panic("Read error " + err.Error())
	}
	for cursor.HasNextRecord() {
		counter++
		record, err := cursor.NextRecord()
		if err != nil {
			fmt.Printf("Error getting next record cursor: %v\n", err)
			panic("Cursor error " + err.Error())
		}
		// fmt.Println("Quantity: ", record.Quantity)
		if record.Quantity > 1 {
			err = validater.listDuplikats(record.HashFields["ChecksumPicture"].String())
			if err != nil {
				fmt.Printf("Error checking duplikats ChecksumPicture: %v\n", err)
				panic("Duplikat error " + err.Error())
			}
		}
		if validater.limit != 0 && counter >= validater.limit {
			break
		}
	}
	stop <- true
	fmt.Printf("%s Picture counter=%d checked=%d ok=%d unique=%d failure=%d empty=%d del Dupli=%d del Empty=%d\n",
		time.Now().Format(timeFormat), counter, validater.checkedPicture,
		validater.okPictures, validater.unique, validater.failurePictures,
		validater.emptyPictures, validater.deleteDuplikate, validater.deleteEmpty)
	fmt.Printf("There are %06d unique records\n", counter)
	for c, ce := range validater.elementMap {
		fmt.Println("Elements of ", c, " = ", ce.counter, "occurence")
	}
	return nil
}

func (validater *validater) listDuplikats(checksum string) (err error) {
	if validater.list == nil {
		validater.list, err = validater.conn.CreateMapReadRequest(&store.PictureData{})
		if err != nil {
			validater.conn.Close()
			return
		}
		err = validater.list.QueryFields("Media")
		if err != nil {
			validater.conn.Close()
			return
		}
		validater.list.Multifetch = 1
		validater.list.Limit = 1
	}
	cursor, err := validater.list.ReadLogicalWithCursoring("ChecksumPicture=" + checksum)
	if err != nil {
		fmt.Printf("Error checking descriptor quantity for ChecksumPicture: %v (%s)\n", err, checksum)
		panic("Read error " + err.Error())
	}
	validater.unique++
	first := true
	var data []byte
	var baseIsn uint64
	counter := 0
	for cursor.HasNextRecord() {
		validater.checkedPicture++
		counter++
		record, recErr := cursor.NextData()
		if recErr != nil {
			panic("Read error " + recErr.Error())
		}
		curPicture := record.(*store.PictureData)
		if first {
			data = curPicture.Media
			if len(data) == 0 {
				fmt.Println("Main record media is empty", checksum)
				validater.emptyPictures++
			} else {
				validater.okPictures++
			}
			baseIsn = curPicture.Index
			first = false
		} else {
			if data != nil {
				if len(curPicture.Media) == 0 {
					fmt.Println("Second record media is empty", checksum)
					validater.emptyPictures++
					fmt.Println("Delete empty ISN:", curPicture.Index, " of ", baseIsn)
					err = validater.Delete(curPicture.Index)
					if err != nil {
						return err
					}
					validater.deleteEmpty++
				} else if bytes.Compare(data, curPicture.Media) != 0 {
					fmt.Println("Record entry differ to first", checksum)
					validater.failurePictures++
				} else {
					validater.okPictures++
					fmt.Println("Delete duplikate ISN:", curPicture.Index, " of ", baseIsn)
					err = validater.Delete(curPicture.Index)
					if err != nil {
						return err
					}
					validater.deleteDuplikate++
				}
			} else {
				fmt.Println("First record is empty")
			}
		}
		if err != nil {
			return err
		}
		// fmt.Printf("  ISN=%06d %s -> %s\n", record.Isn, record.HashFields["PictureName"].String(), record.HashFields["Option"])
	}
	if c, ok := validater.elementMap[counter]; ok {
		c.counter++
	} else {
		validater.elementMap[counter] = &elementCounter{counter: 1}
	}
	if !validater.test {
		return validater.conn.EndTransaction()
	}
	return nil
}

func (validater *validater) Delete(isn uint64) (err error) {
	if !validater.test {
		if validater.delete == nil {
			validater.delete, err = validater.conn.CreateMapDeleteRequest("PictureMetadata")
			if err != nil {
				validater.conn.Close()
				return
			}
		}

		validater.delete.Delete(adatypes.Isn(isn))
	}
	return nil
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
