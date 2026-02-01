/*
* Copyright © 2018-2026 private, Darmstadt, Germany and/or its licensors
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
package tools

import (
	"fmt"
	"os"
	"time"

	"github.com/tknie/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var level = zapcore.ErrorLevel

func init() {
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
	case "2":
		level = zapcore.InfoLevel
	}
}

func InitLogLevelWithFile(fileName string) (err error) {
	p := os.Getenv("LOGPATH")
	if p == "" {
		p = "."
	} else {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			err := os.Mkdir(p, os.ModePerm)
			if err != nil {
				fmt.Printf("Error creating log path '%s': %v\n", p, err)
				os.Exit(255)
			}
		}
	}

	name := p + string(os.PathSeparator) + fileName
	lumberLog := &lumberjack.Logger{
		Filename:   name, // Location of the log file
		MaxSize:    10,   // Maximum file size (in MB)
		MaxBackups: 3,    // Maximum number of old files to retain
		MaxAge:     28,   // Maximum number of days to retain old files
		Compress:   true, // Whether to compress/archive old files
		LocalTime:  true, // Use local time for timestamps
	}
	writer := zapcore.AddSync(lumberLog)
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "timestamp"
	cfg.EncodeTime = zapcore.TimeEncoderOfLayout(time.DateTime)

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		writer,
		level,
	)
	logger := zap.New(core)
	defer logger.Sync() // Flush any buffered log entries

	sugar := logger.Sugar()

	sugar.Infof("Start logging with level %s", level)
	log.Log = sugar
	log.SetDebugLevel(level == zapcore.DebugLevel)

	return
}
