/*
* Copyright © 2018-2024 private, Darmstadt, Germany and/or its licensors
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
	"tux-lobload/sql"
	"tux-lobload/store"

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

	err := initLogLevelWithFile("exiftool.log", level)
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

	log.Log = sugar
	log.Log.Infof("Start logging with level %s", level)
	log.SetDebugLevel(level == zapcore.DebugLevel)

	return
}

func main() {

	limit := 0
	preFilter := ""

	flag.IntVar(&limit, "l", 50, "Maximum number of records loaded")
	flag.StringVar(&preFilter, "f", "", "Prefix of title used in search")
	flag.Parse()

	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return
	}
	wid, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return
	}
	if preFilter != "" {
		preFilter = fmt.Sprintf(" AND title LIKE '%s%%'", preFilter)
	}
	count := uint64(0)
	skipped := uint64(0)
	query := &common.Query{
		TableName:  "pictures",
		Fields:     []string{"ChecksumPicture", "title", "mimetype", "media"},
		DataStruct: &store.Pictures{},
		Limit:      uint32(limit),
		Search:     "mimetype LIKE 'image/%' AND GPScoordinates IS NULL" + preFilter,
	}
	_, err = id.Query(query, func(search *common.Query, result *common.Result) error {
		p := result.Data.(*store.Pictures)
		if (skipped+count)%100 == 0 {
			fmt.Printf("Extract and store exif on %d records, skipped are %d\r", count, skipped)
		}
		err := p.ExifReader()
		if err != nil {
			skipped++
			return nil
		}
		p.Exif = strings.ReplaceAll(p.Exif, "\\", "\\\\")
		count++
		insert := &common.Entries{
			Fields:     []string{"exif", "GPScoordinates", "GPSlatitude", "GPSlongitude"},
			DataStruct: p,
			Values:     [][]any{{p}},
			Update:     []string{"checksumpicture='" + p.ChecksumPicture + "'"},
		}
		_, n, err := wid.Update("pictures", insert)
		if err != nil {
			fmt.Println("Error inserting", n, ":", err)
			fmt.Println("Pic:", p.ChecksumPicture)
			fmt.Println(p.Exif)
			return err
		}
		return nil
	})
	if err != nil {
		fmt.Println("Query error:", err)
	}
	fmt.Println()
	fmt.Printf("Finally worked on %d records and %d are skipped\n", count, skipped)
}
