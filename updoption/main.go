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
	"os/exec"
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

const (
	begin processStep = iota
	analyzeDoublikats
	listDuplikats
	listDuplikatsRead
	updateDuplikats
	updateDuplikatsRead
	initialize
	readStream
	delete
	deleteEnd
	end
)

var processSteps = []string{"Begin", "analyze", "list", "list read", "update", "update read", "init", "read stream", "delete", "delete ET", "end"}

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
	conn            *adabas.Connection
	read            *adabas.ReadRequest
	list            *adabas.ReadRequest
	store           *adabas.StoreRequest
	storeMovie      *adabas.StoreRequest
	delete          *adabas.DeleteRequest
	adabas          *adabas.Adabas
	limit           uint64
	deleteDuplikate bool
	step            processStep
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
	var delete bool
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	flag.StringVar(&dbidParameter, "d", "23", "Map repository Database id")
	flag.IntVar(&mapFnrParameter, "f", 4, "Map repository file number")
	flag.IntVar(&limit, "l", 10, "Maximum records to read (0 is all)")
	flag.BoolVar(&delete, "D", false, "Delete duplicate entries")
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
	c := &checker{adabas: a, limit: uint64(limit), deleteDuplikate: delete, step: initialize}
	err = c.analyzeDoublikats()
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

func (checker *checker) deleteIsn(isn adatypes.Isn) (err error) {
	checker.step = delete
	if checker.delete == nil {
		checker.delete, err = checker.conn.CreateMapDeleteRequest("PictureMetadata")
		if err != nil {
			checker.conn.Close()
			return err
		}
	}
	err = checker.delete.Delete(isn)
	if err != nil {
		return err
	}
	checker.step = deleteEnd
	return checker.delete.EndTransaction()
}

func (checker *checker) analyzeDoublikats() (err error) {
	checker.step = analyzeDoublikats
	checker.conn, err = adabas.NewConnection("acj;map")
	if err != nil {
		return err
	}
	defer checker.conn.Close()
	if checker.read == nil {
		checker.read, err = checker.conn.CreateMapReadRequest("PictureMetadata")
		if err != nil {
			checker.conn.Close()
			return err
		}
		checker.read.Limit = checker.limit
		err = checker.read.QueryFields("ChecksumPicture,PictureName,PictureHost,MIMEType,Option")
		if err != nil {
			checker.conn.Close()
			return err
		}
	}
	counter := uint64(0)
	output := func() {
		fmt.Printf("%s Picture counter=%d -> %s\n",
			time.Now().Format(timeFormat), counter, checker.step.command())
	}
	stop := schedule(output, 15*time.Second)
	result, err := checker.read.ReadLogicalByStream("ChecksumPicture", func(record *adabas.Record, x interface{}) error {
		checker.step = readStream
		if strings.Trim(record.HashFields["ChecksumPicture"].String(), " ") == "" {
			fmt.Println("Checksum picture missing: ", record.Isn, " removing ...")
			return checker.deleteIsn(record.Isn)
		}
		if record.HashFields["PictureHost"].String() == store.Hostname && !strings.HasPrefix(record.HashFields["MIMEType"].String(), "image/") {
			fmt.Println("---> tag info video")
			checker.tagInfoVideo(record.Isn, record.HashFields["PictureName"].String())
		}
		if strings.Trim(record.HashFields["Option"].String(), " ") == "" {
			fmt.Println("Empty option found at", record.Isn)
		}

		// fmt.Printf("quantity=%03d -> %s\n", record.Quantity, record.HashFields["ChecksumPicture"])
		err = checker.listDuplikats(record.HashFields["ChecksumPicture"].String())
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
	fmt.Printf("There are %06d records -> %d\n", counter, result.NrRecords())
	return nil
}

func (checker *checker) listDuplikats(checksum string) (err error) {
	checker.step = listDuplikats
	if checker.list == nil {
		checker.list, err = checker.conn.CreateMapReadRequest("PictureMetadata")
		if err != nil {
			checker.conn.Close()
			return
		}
		err = checker.list.QueryFields("PictureHost,PictureName,MIMEType,Option")
		if err != nil {
			checker.conn.Close()
			return
		}

	}
	cursor, err := checker.list.ReadLogicalWithCursoring("ChecksumPicture=" + checksum)
	if err != nil {
		fmt.Printf("Error checking descriptor quantity for ChecksumPicture: %v\n", err)
		panic("Read error " + err.Error())
	}
	first := true
	lastName := ""
	for cursor.HasNextRecord() {
		checker.step = listDuplikatsRead
		record, recErr := cursor.NextRecord()
		if recErr != nil {
			panic("Read error " + recErr.Error())
		}
		currentOption := strings.Trim(record.HashFields["Option"].String(), " ")
		if first {
			lastName = record.HashFields["PictureName"].String()
			switch currentOption {
			case "":
				err = checker.updateOption(record, "original")
				if err != nil {
					panic("Update error" + err.Error())
				}
			case "original":
			default:
				fmt.Println(record.HashFields["PictureName"], currentOption, "should be original, but was "+currentOption)
				err = checker.updateOption(record, "original")
				if err != nil {
					panic("Update error" + err.Error())
				}
			}
			first = false
		} else {
			switch currentOption {
			case "":
				err = checker.updateOption(record, "duplicate")
				if err != nil {
					panic("Update error" + err.Error())
				}
			case "duplicate":
			default:
				fmt.Println(record.HashFields["PictureName"], currentOption, "should be duplicate of", lastName)
				err = checker.updateOption(record, "duplicate")
				if err != nil {
					panic("Update error" + err.Error())
				}
			}
		}
		if err != nil {
			return err
		}
		// fmt.Printf("  ISN=%06d %s -> %s\n", record.Isn, record.HashFields["PictureName"].String(), record.HashFields["Option"])
	}
	return checker.conn.EndTransaction()
}

func (checker *checker) tagInfoVideo(isn adatypes.Isn, filename string) error {
	fmt.Println("Check picture", isn, "at", filename)
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-show_format", filename)
	x, err := cmd.Output()
	if err != nil {
		return err
	}
	var v interface{}
	err = json.Unmarshal(x, &v)
	if err != nil {
		return err
	}
	data := v.(map[string]interface{})
	t, ok := data["format"]
	if !ok {
		return nil
	}
	tags := t.(map[string]interface{})
	ctag, ok := tags["tags"]
	if !ok {
		return nil
	}
	ct := ctag.(map[string]interface{})
	cts, ok := ct["creation_time"]
	if !ok {
		return nil
	}
	ctime := cts.(string)
	createTime, tErr := time.Parse(time.RFC3339, ctime)
	if tErr != nil {
		return tErr
	}
	up := &store.PictureMetadata{Index: uint64(isn), ExifTaken: createTime.String()}
	err = checker.updateMovieTime(up)
	if err != nil {
		fmt.Println("Update movie error", err)
	}
	return err
}

func (checker *checker) updateMovieTime(up *store.PictureMetadata) (err error) {
	if checker.storeMovie == nil {
		checker.storeMovie, err = checker.conn.CreateMapStoreRequest(up)
		if err != nil {
			fmt.Println("Map Store error...", up.Index, err)
			return err
		}
		err = checker.storeMovie.StoreFields("ExifTaken")
		if err != nil {
			return err
		}
	}
	err = checker.storeMovie.UpdateData(up)
	fmt.Println("Update", up.Index)
	return err
}

func (checker *checker) updateOption(record *adabas.Record, option string) error {
	checker.step = updateDuplikatsRead
	// fmt.Println("Updateing...", record.Isn, record.HashFields["PictureName"], record.HashFields["Option"], option)
	err := record.SetValue("Option", option)
	if err != nil {
		return err
	}
	if checker.store == nil {
		checker.store, err = checker.conn.CreateMapStoreRequest("PictureMetadata")
		if err != nil {
			fmt.Println("Map Store error...", record.Isn, record.HashFields["PictureName"], record.HashFields["Option"], err)
			return err
		}
	}
	err = checker.store.Update(record)
	if err != nil {
		fmt.Println("Update error...", record.Isn, record.HashFields["PictureName"], record.HashFields["Option"], err)
		return err
	}
	checker.step = updateDuplikats
	err = checker.store.EndTransaction()
	// fmt.Println("End transaction...", record.Isn, record.HashFields["PictureName"], record.HashFields["Option"], option)
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
