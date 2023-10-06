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
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"tux-lobload/sql"

	"github.com/tknie/adabas-go-api/adatypes"
	"github.com/tknie/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const timeFormat = "2006-01-02 15:04:05"

var hostname string
var insertAlbum = false
var albumid = 1

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

	err := initLogLevelWithFile("syncAlbum.log", level)
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

func main() {
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
	title := ""
	listSource := false
	listDest := false
	flag.BoolVar(&insertAlbum, "A", false, "Insert Albums")
	flag.BoolVar(&listSource, "l", false, "List source Albums")
	flag.BoolVar(&listDest, "L", false, "List destination Albums")
	flag.StringVar(&title, "a", "", "Search Albums title")
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
	var a *sql.Albums
	// sourceUrl := "postgres://admin@localhost:5432/bitgarten"
	sourceUrl := os.Getenv("POSTGRES_SOURCE_URL")
	pwd := os.Getenv("POSTGRES_SOURCE_PASSWORD")
	connSource, err := sql.Connect(sourceUrl, pwd)
	if err != nil {
		fmt.Println("Error creating connection:", err)
		return
	}
	//	destUrl := "postgres://admin@bear:5433/bitgarten"
	destUrl := os.Getenv("POSTGRES_DESTINATION_URL")
	pwd = os.Getenv("POSTGRES_DESTINATION_PASSWORD")
	destSource, err := sql.Connect(destUrl, pwd)
	if err != nil {
		fmt.Println("Error creating connection:", err)
		return
	}
	switch {
	case listSource:
		err = connSource.ListAlbums()
		if err != nil {
			fmt.Println("List albums error:", err)
			return
		}
		return
	case listDest:
		err = destSource.ListAlbums()
		if err != nil {
			fmt.Println("List albums error:", err)
			return
		}
		return
	case title != "":
		a, err = connSource.ReadAlbum(title)
		if err != nil {
			fmt.Println("Error reading album:", err)
			return
		}
	default:
	}
	if a != nil {
		a.Display()
		for _, p := range a.Pictures {
			f, err := destSource.CheckPicture(p.ChecksumPicture)
			if err != nil {
				fmt.Println("Error checking picature:", err)
				return
			}
			if !f {
				fmt.Println("Not in destination database, picture", p.ChecksumPicture, f)
				err := copyPicture(connSource, destSource, p.ChecksumPicture)
				if err != nil {
					fmt.Println("Error copying picture:", err)
					return
				}
			}
		}
		err = destSource.WriteAlbum(a)
		if err != nil {
			fmt.Println("Error writing album:", err)
			return
		}
		for _, ap := range a.Pictures {
			err = destSource.WriteAlbumPictures(ap)
			if err != nil {
				fmt.Println("Error writing album pictures:", err)
				return
			}
		}
	}
}

func copyPicture(connSource, destSource *sql.DatabaseInfo, checksum string) error {
	p, err := connSource.ReadPicture(checksum)
	if err != nil {
		return err
	}
	c := fmt.Sprintf("%X", md5.Sum(p.Media))
	if p.ChecksumPicture != c {
		return fmt.Errorf("checksum mismatch: %s", p.ChecksumPicture)
	}
	fmt.Println("Successful read picture", p.ChecksumPicture, p.Created)
	err = destSource.WritePicture(p)
	if err != nil {
		fmt.Println("Error writing picture:", err)
		return err
	}
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
