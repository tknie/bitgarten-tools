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

	"github.com/tknie/adabas-go-api/adatypes"
	"github.com/tknie/flynn"
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
		adatypes.Central.SetDebugLevel(true)
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("exifclean.log", level)
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

type exif struct {
	Checksumpicture string
	Exifmodel       string
	Exifmake        string
	Exiftaken       time.Time
}

func main() {
	tableName := ""
	flag.StringVar(&tableName, "t", "pictures", "Table name to search in")
	flag.Parse()

	log.Log.Debugf("Start exifclean")

	url := os.Getenv("POSTGRES_URL")

	ref, passwd, err := common.NewReference(url)
	if err != nil {
		fmt.Println("URL error:", err)
		return
	}

	id, err := flynn.RegisterDatabase(ref, passwd)
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return
	}
	query := &common.Query{
		TableName:  tableName,
		DataStruct: &exif{},
		Fields:     []string{"exifmodel", "exifmake", "exiftaken", "checksumpicture"},
	}
	count := int64(0)
	r, err := id.Query(query, func(search *common.Query, result *common.Result) error {
		x := result.Data.(*exif)
		if strings.HasPrefix(x.Exifmodel, "\"") || strings.HasPrefix(x.Exifmodel, "<") ||
			strings.HasPrefix(x.Exifmake, "\"") || strings.HasPrefix(x.Exifmake, "<") {
			toModel := strings.Trim(x.Exifmodel, "\"")
			toModel = strings.Trim(toModel, "<>")
			toModel = strings.Trim(toModel, " ")
			fmt.Printf("MODEL: %s: <%s> -> <%s>\n", x.Checksumpicture, x.Exifmodel, toModel)
			x.Exifmodel = toModel
			toModel = strings.Trim(x.Exifmake, "\"")
			toModel = strings.Trim(toModel, "<>")
			toModel = strings.Trim(toModel, " ")
			fmt.Printf("MAKE : %s: <%s> -> <%s>\n", x.Checksumpicture, x.Exifmake, toModel)
			x.Exifmake = toModel
			list := [][]any{{x}}
			update := &common.Entries{Fields: []string{"exifmodel", "exifmake"},
				DataStruct: x,
				Values:     list,
				Update:     []string{"checksumpicture = '" + x.Checksumpicture + "'"},
			}
			n, err := id.Update(tableName, update)
			if err != nil {
				fmt.Println("Error updating record:", err)
				return err
			}
			err = id.Commit()
			if err != nil {
				fmt.Println("Erro commiting record:", err)
				return err
			}
			count += n
		}
		return nil
	})
	fmt.Println("Updates: ", count, r.Counter)
	if err != nil {
		fmt.Println("Aborted with error:", err)
	}
}
