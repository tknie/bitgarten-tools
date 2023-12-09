/*
* Copyright © 2023 private, Darmstadt, Germany and/or its licensors
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
	"strings"
	"time"
	"tux-lobload/sql"
	"tux-lobload/store"

	"github.com/tknie/log"

	"github.com/tknie/adabas-go-api/adabas"
	"github.com/tknie/adabas-go-api/adatypes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// var cpMap sync.Map
var adabasTarget = ""
var adabasAlbumFile = 5
var adabasPictureFile = 6

func init() {
	level := zapcore.ErrorLevel
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
		adatypes.Central.SetDebugLevel(true)
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("converter.log", level)
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
	adatypes.Central.Log = sugar
	log.Log = sugar
	log.SetDebugLevel(level == zapcore.DebugLevel)

	return
}

func convertAlbum() error {
	connection, cerr := adabas.NewConnection(fmt.Sprintf("acj;inmap=%s,%d", adabasTarget, adabasAlbumFile))
	if cerr != nil {
		return cerr
	}
	defer connection.Close()
	request, err := connection.CreateMapReadRequest(&store.Album{})
	if err != nil {
		return err
	}
	err = request.QueryFields("*")
	if err != nil {
		fmt.Println("Query fields error:", err)
		return err
	}
	response, rerr := request.ReadPhysicalWithCursoring()
	if rerr != nil {
		fmt.Println("Read fields error:", rerr)
		return rerr
	}

	db, err := sql.CreateConnection()
	if err != nil {
		fmt.Println("Connection error:", rerr)
		return err
	}
	n := time.Now()
	count := 0
	var sumAdabasUsed time.Duration
	for response.HasNextRecord() {
		record, err := response.NextData()
		if err != nil {
			fmt.Println("Read error:", err)
			return err
		}
		sumAdabasUsed += time.Since(n)
		a := record.(*store.Album)
		for _, p := range a.Pictures {
			if m, ok := sql.Md5Map.Load(p.Md5); ok {
				fmt.Printf("Map md5=<%s> use instead <%s>\n", p.Md5, m.(string))
				p.Md5 = m.(string)
				fmt.Printf("Replace  <%s> -> <%s>\n", p.Md5, m.(string))

			} else {
				log.Log.Fatalf("Md5 hash missing for %s -> %s", p.Name, p.Md5)
			}
		}

		fmt.Println("Found entry and insert ", a.Title, time.UnixMilli(int64(a.Date*1000)))
		err = db.InsertAlbum(a)
		if err != nil {
			fmt.Println("SQL insert error:", err)
			return err
		}
		count++
		if count%100 == 0 {
			fmt.Println("Read adabas records", count)
		}
		n = time.Now()
	}
	fmt.Println("Used adabas", sumAdabasUsed)

	fmt.Printf("Load %d albums...\n", count)
	return nil
}

func convertPictures(nr int) error {
	connection, cerr := adabas.NewConnection(fmt.Sprintf("acj;inmap=%s,%d", adabasTarget, adabasPictureFile))
	if cerr != nil {
		return cerr
	}
	defer connection.Close()
	fmt.Printf("Created Adabas connection...\n")
	request, err := connection.CreateMapReadRequest(&store.Pictures{})
	if err != nil {
		return err
	}
	err = request.QueryFields("*")
	if err != nil {
		fmt.Println("Query fields error:", err)
		return err
	}
	request.Multifetch = 2
	request.Limit = 10
	response, rerr := request.ReadPhysicalWithCursoring()
	if rerr != nil {
		fmt.Println("Read fields error:", rerr)
		return rerr
	}
	descrRequest, err := connection.CreateMapReadRequest(&store.Pictures{})
	if err != nil {
		return err
	}
	err = descrRequest.QueryFields("M5,CP")
	if err != nil {
		return err
	}
	for i := 0; i < nr; i++ {
		go sql.InsertWorker()
	}
	count := 0
	var sumAdabasUsed time.Duration
	for response.HasNextRecord() {
		n := time.Now()
		record, err := response.NextData()
		if err != nil {
			fmt.Println("Read error:", err)
			return err
		}
		sumAdabasUsed += time.Since(n)
		pic := record.(*store.Pictures)
		pic.ExifReader()
		pic.Md5 = strings.Trim(pic.Md5, " ")
		pic.ChecksumPicture = strings.Trim(pic.ChecksumPicture, " ")
		cp := strings.Trim(pic.ChecksumPicture, " ")
		sql.Md5Map.Store(pic.Md5, cp)
		sql.StorePictures(pic)
		count++
		if count%100 == 0 {
			fmt.Println("Read adabas records", count)
		}
	}
	fmt.Println("Used adabas", sumAdabasUsed)
	fmt.Printf("Load %d pictures, waiting ...\n", count)
	sql.WaitStored()
	fmt.Printf("Stop all worker...\n")
	for i := 0; i < nr; i++ {
		sql.StopWorker()
	}
	return nil
}

func createHashMap() error {
	connection, cerr := adabas.NewConnection(fmt.Sprintf("acj;inmap=%s,%d", adabasTarget, adabasPictureFile))
	if cerr != nil {
		return cerr
	}
	defer connection.Close()
	fmt.Printf("Created Adabas connection...\n")
	request, err := connection.CreateMapReadRequest(&store.Pictures{})
	if err != nil {
		return err
	}
	err = request.QueryFields("CP,M5")
	if err != nil {
		fmt.Println("Query fields error:", err)
		return err
	}
	request.Multifetch = 2
	request.Limit = 10
	response, rerr := request.ReadPhysicalWithCursoring()
	if rerr != nil {
		fmt.Println("Read fields error:", rerr)
		return rerr
	}
	for response.HasNextRecord() {
		record, err := response.NextData()
		if err != nil {
			fmt.Println("Read error:", err)
			return err
		}
		pic := record.(*store.Pictures)
		cp := strings.Trim(pic.ChecksumPicture, " ")
		sql.Md5Map.Store(pic.Md5, cp)
	}
	return nil
}

func main() {
	verify := false
	skip := false
	picOnly := false

	adaTarget := os.Getenv("ADA_TARGET")
	if adaTarget == "" {
		adaTarget = "adatcp://lion.fritz.box:64150"
	}

	workerNr := 1
	flag.IntVar(&workerNr, "w", 4, "Nr of workers for sql insert")
	flag.BoolVar(&verify, "v", false, "Verify data")
	flag.BoolVar(&picOnly, "p", false, "Load picture data only")
	flag.BoolVar(&skip, "s", false, "Skip picture load")
	flag.StringVar(&adabasTarget, "T", adaTarget, "Adabas target")
	flag.IntVar(&adabasAlbumFile, "A", 5, "Adabas Album file number")
	flag.IntVar(&adabasPictureFile, "P", 6, "Adabas Picture file number")
	flag.Parse()

	fmt.Printf("Number of worker   : %d\n", workerNr)
	fmt.Printf("Skip picture load  : %v\n", skip)
	fmt.Printf("Load picture only  : %v\n", picOnly)
	fmt.Printf("Verify picture data: %v\n", verify)

	err := sql.Display()
	if err != nil {
		fmt.Println("SQL display error", err)
		return
	}
	if !skip || picOnly {
		fmt.Println("Convert pictures ...")
		err = convertPictures(workerNr)
		if err != nil {
			fmt.Println("SQL convert pictures error", err)
			return
		}
	} else {
		createHashMap()
	}
	fmt.Println("Convert albums ...")
	if !picOnly {
		err = convertAlbum()
		if err != nil {
			fmt.Println("SQL convert album error", err)
			return
		}
	}
}
