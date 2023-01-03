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
	"tux-lobload/store"

	"github.com/tknie/adabas-go-api/adabas"
	"github.com/tknie/adabas-go-api/adatypes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

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

	err := initLogLevelWithFile("thumbnail.log", level)
	if err != nil {
		panic(err)
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
		panic(err)
	}
	cfg.Level.SetLevel(level)
	cfg.OutputPaths = []string{name}
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	sugar := logger.Sugar()

	sugar.Infof("Start logging with level", level)
	adatypes.Central.Log = sugar

	return
}

func main() {
	var fileName string
	var dbidParameter string
	var mapFnrParameter int
	var verify bool
	flag.StringVar(&fileName, "p", "", "File name of picture to be imported")
	flag.StringVar(&dbidParameter, "d", "23", "Map repository Database id")
	flag.IntVar(&mapFnrParameter, "f", 4, "Map repository file number")
	flag.BoolVar(&verify, "v", false, "Verify data")
	flag.Parse()

	//	adabas.AddGlobalMapRepository(dbidParameter, adabas.Fnr(mapFnrParameter))
	adabas.AddGlobalMapRepositoryReference(fmt.Sprintf("%s,%d", dbidParameter, mapFnrParameter))

	con, err := adabas.NewConnection("acj;map")
	if err != nil {
		fmt.Println("Error connection", err)
		return
	}
	defer con.Close()
	readRequest, rerr := con.CreateMapReadRequest((*store.Album)(nil))
	if rerr != nil {
		fmt.Println("Read request", rerr)
		return
	}
	readRequest.Limit = 0
	err = readRequest.QueryFields("Title,Thumbnail,Pictures,Date")
	if err != nil {
		fmt.Println("Read fields error", err)
		return
	}
	result, readErr := readRequest.ReadLogicalBy("Date")
	if readErr != nil {
		fmt.Println("Read error", readErr)
		return
	}
	for _, d := range result.Data {
		a := d.(*store.Album)
		fmt.Println(a.Title, a.Thumbnail, a.Pictures[0].Md5)

	}
}
