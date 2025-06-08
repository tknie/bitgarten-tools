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

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/tknie/bitgartentools/sql"
	"github.com/tknie/bitgartentools/store"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

const exportTimeFormat = "2006-01-02"

type ExportMediaParameter struct {
	Directory  string
	Limit      int
	MarkDelete bool
}

type stat struct {
	wrote     uint64
	processed uint64
	found     uint64
}

var statCount = &stat{}
var exportParameter *ExportMediaParameter

var picChannel chan *store.Pictures
var stop = make(chan bool)

var wgWrite sync.WaitGroup

func StartExport(workers int) {
	picChannel = make(chan *store.Pictures, workers)

	for range workers {
		go writerMediaFile()
	}
}

func ExportMedia(parameter *ExportMediaParameter) error {
	if parameter.Directory == "" {
		parameter.Directory = "./"
	}
	exportParameter = parameter
	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("Error connect ...:", err)
		return err
	}
	defer id.FreeHandler()

	limit := "ALL"
	if parameter.Limit > 0 {
		limit = strconv.Itoa(parameter.Limit)
	}
	search := ""
	if !parameter.MarkDelete {
		search = "markdelete = false"
	}
	q := &common.Query{TableName: "Pictures",
		DataStruct:   &store.Pictures{},
		Search:       search,
		Limit:        limit,
		FctParameter: parameter,
		Fields: []string{"MIMEType", "title", "exiforigtime",
			"checksumpicture", "Media"},
	}
	log.Log.Debugf("Call batch ...")
	_, err = id.Query(q, writeMediaFile)
	if err != nil {
		log.Log.Errorf("Error video title query: %v", err)
		fmt.Println("Error exporting media query ...:", err)
		return err
	}
	wgWrite.Wait()
	stop <- true
	log.Log.Debugf("Call batch done ...")
	fmt.Println("Processed:", statCount.processed)
	fmt.Println("Found    :", statCount.found)
	fmt.Println("Wrote    :", statCount.wrote)
	return nil
}

func writeMediaFile(search *common.Query, result *common.Result) error {
	pic := result.Data.(*store.Pictures)
	//	var p store.Pictures
	p := *pic
	picChannel <- &p
	wgWrite.Add(1)
	return nil
}

func writerMediaFile() {
	for {
		select {
		case pic := <-picChannel:
			atomic.AddUint64(&statCount.processed, 1)
			filename := fmt.Sprintf("%s/%s/%c/%s/%s-%s", exportParameter.Directory,
				pic.ExifOrigTime.Format(exportTimeFormat), pic.Title[0], pic.Title,
				pic.ChecksumPicture, pic.Title)
			dirname := filepath.Dir(filename)
			log.Log.Debugf("Create directory: %s", dirname)
			if stat, err := os.Stat(filename); err == nil {
				fmt.Println(filename, "exist", stat.Size(), " -> ", len(pic.Media))
				if stat.Size() != int64(len(pic.Media)) {
					fmt.Println("Size test of filename fails", filename, ":", err)
					os.Exit(1)
				}
				data, err := os.ReadFile(filename)
				if err != nil {
					fmt.Println("Check of filename fails", filename, ":", err)
					os.Exit(1)
				}
				md5 := store.CreateMd5(data)
				if md5 != pic.ChecksumPicture {
					fmt.Println("Compare of filename fails", filename, md5, "!=", pic.ChecksumPicture)
					os.Exit(1)
				}
				atomic.AddUint64(&statCount.found, 1)
			} else {
				md5 := store.CreateMd5(pic.Media)
				if md5 != pic.ChecksumPicture {
					fmt.Println("Compare of pic data fails", filename, md5, "!=", pic.ChecksumPicture)
					os.Exit(1)
				}
				if _, err := os.Stat(dirname); os.IsNotExist(err) {
					os.MkdirAll(dirname, 0700)
				}
				err := os.WriteFile(filename, pic.Media, 0644)
				if err == nil {
					atomic.AddUint64(&statCount.wrote, 1)
					// fmt.Printf("Write Media file %s\n", filename)
				} else {
					fmt.Printf("Error writing Media file %s: %v\n", filename, err)
					os.Exit(1)
				}
			}
			wgWrite.Done()
		case <-stop:
			return
		}
	}
}
