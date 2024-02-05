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
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"tux-lobload/store"

	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Albums struct {
	Id          uint64
	Type        string
	Key         string
	Directory   string
	Title       string
	Description string
}

var wid common.RegDbID
var storeData bool

var ref *common.Reference
var passwd string

func init() {
	level := zapcore.ErrorLevel
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("heicthumb.log", level)
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
		"outputPaths": [ "heicthumb.log"],
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

	sugar.Infof("Start logging with level %v", level)
	log.Log = sugar
	log.SetDebugLevel(level == zapcore.DebugLevel)

	return nil
}

func main() {
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
	var chksum string
	var title string
	flag.StringVar(&chksum, "c", "", "Search for picture id checksum")
	flag.StringVar(&title, "t", "", "Search for picture title")
	flag.BoolVar(&storeData, "S", false, "Store data to database")
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

	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		fmt.Println("Set POSTGRES_URL and/or POSTGRES_PASSWORD to define remote database")
		return
	}
	fmt.Println("Start generating heic thumbnails")
	var err error
	ref, passwd, err = common.NewReference(url)
	if err != nil {
		fmt.Println("Error parsing URL", err)
		return
	}
	// fmt.Println("Got passwd <", passwd, ">")
	if passwd == "" {
		passwd = os.Getenv("POSTGRES_PASSWORD")
	}
	id, err := flynn.Handler(ref, passwd)
	if err != nil {
		fmt.Println("Error connect ...:", err)
		return
	}
	defer id.FreeHandler()
	if storeData {
		wid, err = flynn.Handler(ref, passwd)
		if err != nil {
			fmt.Println("Error connect write ...:", err)
			return
		}
		defer wid.FreeHandler()
	}

	q := &common.Query{TableName: "Pictures",
		DataStruct:   &store.Pictures{},
		Fields:       []string{"MIMEType", "checksumpicture", "title", "Media", "exiforigtime"},
		FctParameter: id,
	}

	prefix := "title LIKE '%heic'"
	if chksum != "" {
		prefix += fmt.Sprintf(" AND checksumpicture = '%s'", chksum)
	}
	if title != "" {
		prefix += fmt.Sprintf(" AND title = %s", title)
	}
	q.Search = prefix
	_, err = id.Query(q, generateQueryImageThumbnail)
	if err != nil {
		fmt.Println("Error query ...:", err)
		return
	}
	if storeData {
		fmt.Println("Commiting changes ...")
		err = wid.Commit()
		if err != nil {
			fmt.Println("Error commiting data...", err)
			return
		}
	}
	fmt.Println("Finished heic thumbnails generated")
}

func generateQueryImageThumbnail(search *common.Query, result *common.Result) error {
	id := search.FctParameter.(common.RegDbID)
	pic := result.Data.(*store.Pictures)
	return generateImageThumbnail(id, pic)
}

func generateImageThumbnail(id common.RegDbID, pic *store.Pictures) error {
	if storeData {
		fmt.Println("Found and generate", pic.ChecksumPicture, pic.Title, pic.ExifOrigTime)
		err := pic.CreateThumbnail()
		if err != nil {
			fmt.Println("Error creating thumbnail:", err)
			return err
		}
		fmt.Printf("%s -> %v\n", pic.Title, pic.ExifOrigTime)
		return storeThumb(pic)
	} else {
		fmt.Println("Found", pic.ChecksumPicture, pic.Title, pic.ExifOrigTime)
		searchSimilarEntries(pic.Title)
	}
	return nil
}

func storeThumb(pic *store.Pictures) error {
	update := &common.Entries{
		Fields:     []string{"exif", "Thumbnail", "exifmodel", "exifmake", "exiftaken", "exiforigtime", "exifxdimension", "exifydimension", "exiforientation", "GPScoordinates", "GPSlatitude", "GPSlongitude"},
		DataStruct: pic,
		Values:     [][]any{{pic}},
		Update:     []string{"checksumpicture='" + pic.ChecksumPicture + "'"},
	}
	n, err := wid.Update("pictures", update)
	if err != nil {
		fmt.Println("Error updating", n, ":", err)
		fmt.Println("Pic:", pic.ChecksumPicture)
		fmt.Println(pic.Exif)
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

func searchSimilarEntries(title string) {
	sid, err := flynn.Handler(ref, passwd)
	if err != nil {
		fmt.Println("Error connect ...:", err)
		return
	}
	defer sid.FreeHandler()
	log.Log.Debugf("SID ID:%d", sid)
	did, err := flynn.Handler(ref, passwd)
	if err != nil {
		fmt.Println("Error connect ...:", err)
		return
	}
	defer did.FreeHandler()
	log.Log.Debugf("DID ID:%d", did)
	xTitle := strings.TrimSuffix(title, filepath.Ext(title))

	q := &common.Query{TableName: "Pictures",
		DataStruct: &store.Pictures{},
		Fields:     []string{"MIMEType", "checksumpicture", "title", "exiforigtime"},
	}
	q.Search = "title LIKE '" + xTitle + "%' and markdelete!=true"
	_, err = sid.Query(q, func(search *common.Query, result *common.Result) error {
		pic := result.Data.(*store.Pictures)
		if pic.Title != title {
			if filepath.Ext(pic.Title) == ".jpeg" {
				fmt.Println("  Deleting ", pic.ChecksumPicture, pic.Title, pic.MIMEType, pic.ExifOrigTime)
				update := &common.Entries{
					Fields: []string{"markdelete"},
					Values: [][]any{{true}},
					Update: []string{"checksumpicture='" + pic.ChecksumPicture + "'"},
				}
				n, err := did.Update("pictures", update)
				if err != nil {
					fmt.Println("Error mark delete", n, ":", err)
					fmt.Println("Pic:", pic.ChecksumPicture)
					return err
				}
			} else {
				fmt.Println("  ", pic.ChecksumPicture, pic.Title, pic.MIMEType, pic.ExifOrigTime)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error query ...:", err)
	}
	log.Log.Debugf("End query similar")
}
