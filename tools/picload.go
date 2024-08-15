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
	"regexp"
	"strings"
	"time"

	"github.com/tknie/bitgarten-tools/sql"

	"github.com/docker/go-units"
	"github.com/tknie/log"
)

const timeFormat = "2006-01-02 15:04:05"

// var insertAlbum = false
// var albumid = 1

type PicLoadParameter struct {
	AlbumId        int
	FileName       string
	NrThreadReader int
	NrThreadStorer int
	MaxBlobSize    int64
	Filter         string
	ShortenPath    bool
	Directories    []string
	InsertAlbum    bool
}

func PicLoad(parameter *PicLoadParameter) {

	StoreWorker(parameter.NrThreadReader)
	sql.InsertWorker(parameter.NrThreadStorer)
	MaxBlobSize = parameter.MaxBlobSize
	fmt.Println("Max lob size:", units.HumanSize(float64(MaxBlobSize)))
	ShortPath = parameter.ShortenPath

	regs := make([]*regexp.Regexp, 0)
	for _, r := range strings.Split(parameter.Filter, ",") {
		reg, err := regexp.Compile(r)
		if err != nil {
			log.Log.Fatalf("Regular expression error (%s): %v", r, err)
		}
		regs = append(regs, reg)
	}

	sql.StartStats()
	start := time.Now()
	switch {
	case parameter.FileName != "":
		fmt.Printf("Store file '%s' to album id %d\n", parameter.FileName, parameter.AlbumId)
		suffix := parameter.FileName[strings.LastIndex(parameter.FileName, ".")+1:]
		suffix = strings.ToLower(suffix)
		parameter.storeFile(parameter.FileName, suffix)
		time.Sleep(1 * time.Minute)
	case len(parameter.Directories) > 0:
		for _, pictureDirectory := range parameter.Directories {
			parameter.storeDirectory(pictureDirectory, regs)
		}
	}
	log.Log.Debugf("Wait wgstore")
	wgStore.Wait()

	sql.EndStats()
	fmt.Printf("%s used %v\n", time.Now().Format(timeFormat), time.Since(start))
	for i := 0; i < parameter.NrThreadReader; i++ {
		sql.StopWorker()
	}
}

func (parameter *PicLoadParameter) storeDirectory(pictureDirectory string, regs []*regexp.Regexp) {
	if pictureDirectory != "" {

		if parameter.InsertAlbum {
			di, err := sql.CreateConnection()
			if err != nil {
				fmt.Println("Error connecting:", err)
				return
			}
			dir := filepath.Base(pictureDirectory)
			parameter.AlbumId, err = di.InsertNewAlbum(dir)
			if err != nil {
				fmt.Println("Error inserting album:", err)
				log.Log.Fatal("Error creating Album")
			}
		}
		fmt.Printf("%s Loading path %s\n", time.Now().Format(timeFormat), pictureDirectory)
		err := filepath.Walk(pictureDirectory, func(path string, info os.FileInfo, err error) error {
			if info == nil || info.IsDir() {
				log.Log.Infof("Info empty or dir: %s", path)
				return nil
			}
			for _, reg := range regs {
				if !checkQueryPath(reg, path) {
					return nil
				}
			}

			ti := sql.IncChecked()
			suffix := path[strings.LastIndex(path, ".")+1:]
			suffix = strings.ToLower(suffix)
			if err != nil {
				// return fmt.Errorf("error storing file: %v", err)
				sql.IncErrorFile(err, path)
			}
			switch suffix {
			case "jpg", "jpeg", "tif", "png", "heic", "gif", "m4v", "mov", "avi", "mp4", "webm":
				queueStoreFileInAlbumID(path, parameter.AlbumId)
				if err != nil {
					// return fmt.Errorf("error storing file: %v", err)
					sql.IncErrorFile(err, path)
				}
				ti.IncDone()
			default:
				log.Log.Infof("Suffix not supported: %s\n", suffix)
				sql.IncSkipped()
			}
			return nil
		})
		if err != nil {
			fmt.Println("Abort/Error during file walk:", err)
			return
		}
	}

}

func (parameter *PicLoadParameter) storeFile(path, suffix string) error {
	ti := sql.IncChecked()
	switch suffix {
	case "jpg", "jpeg", "tif", "png", "heic", "gif", "m4v", "mov", "mpg", "avi", "mp4", "webm":
		queueStoreFileInAlbumID(path, parameter.AlbumId)
		ti.IncDone()
	default:
		log.Log.Infof("Suffix not supported: %s\n", suffix)
		sql.IncSkipped()
	}
	return nil
}

func checkQueryPath(reg *regexp.Regexp, path string) bool {
	return !reg.MatchString(path)
}
