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
	"strings"
	"time"

	"github.com/tknie/bitgartentools"
	"github.com/tknie/bitgartentools/sql"
	"github.com/tknie/bitgartentools/store"
	"github.com/tknie/log"
)

type scanStat struct {
	directory   string
	start       time.Time
	end         time.Time
	noAvailable uint64
	available   uint64
	countAll    uint64
	countErrors uint64
	countEmpty  uint64
}

var currentDirectory = "<not defined>"

func analyzeOutput(s time.Time, parameter interface{}) {
	scan := parameter.(*scanStat)
	fmt.Printf("Analyze files in %s started at %v started at %v\n", currentDirectory,
		time.Now().Format(timeFormat), s.Format(timeFormat))
	fmt.Printf("%-22s: %5d / %5d\n", "New pictures", scan.noAvailable, scan.countAll)
	fmt.Printf("%-22s: %5d / %5d\n", "Pictures registered", scan.available, scan.countAll)
	fmt.Printf("%-22s: %5d / %5d\n\n", "Pictures errors/empty", scan.countErrors, scan.countEmpty)
}

func AnalyzeDirectories(directories []string) error {
	checker, err := sql.CreateConnection()
	if err != nil {
		log.Log.Fatalf("Database connection not established: %v", err)
	}
	defer checker.Close()
	scans := make([]*scanStat, 0)
	for _, pictureDirectory := range directories {
		scan := &scanStat{directory: pictureDirectory, start: time.Now()}
		bitgartentools.ScheduleParameter(analyzeOutput, scan, 30*time.Second)
		currentDirectory = pictureDirectory
		err := filepath.Walk(pictureDirectory, func(path string, info os.FileInfo, err error) error {
			if info == nil || info.IsDir() {
				log.Log.Infof("Info empty or dir: %s", path)
				return nil
			}
			suffix := path[strings.LastIndex(path, ".")+1:]
			suffix = strings.ToLower(suffix)
			if err != nil {
				// return fmt.Errorf("error storing file: %v", err)
				sql.IncErrorFile(err, path)
			}
			switch suffix {
			case "jpg", "jpeg", "tif", "png", "heic", "gif", "m4v", "mov", "avi", "mp4", "webm":
				loadFile(checker, scan, path)
			default:
			}

			return nil
		})
		if err != nil {
			fmt.Println("Error working in directories:", err)
			return err
		}
		bitgartentools.StopScheduler()
		scan.end = time.Now()
		fmt.Printf("Finished Analyze files ended at %v\n", time.Now().Format(timeFormat))
		scans = append(scans, scan)
	}
	fmt.Printf("\nSummary:\n")
	for _, s := range scans {
		analyzeOutput(s.start, s)
		fmt.Printf("Finished Analyze files ended at %v\n", s.end.Format(timeFormat))
	}
	return nil
}

func loadFile(db *sql.DatabaseInfo, scan *scanStat, fileName string) error {
	scan.countAll++
	pic := &store.Pictures{}
	f, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Open file error", fileName, ":", err)
		scan.countErrors++
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		scan.countErrors++
		return err
	}
	if fi.Size() == 0 {
		scan.countEmpty++
		return fmt.Errorf("file empty %s", fileName)
	}
	pic.Media = make([]byte, fi.Size())
	var n int
	n, err = f.Read(pic.Media)
	log.Log.Debugf("Number of bytes read: %d/%d -> %v\n", n, len(pic.Media), err)
	if err != nil {
		scan.countErrors++
		return err
	}
	pic.ChecksumPicture = store.CreateMd5(pic.Media)
	pic.ChecksumPictureSHA = store.CreateSHA(pic.Media)

	db.CheckExists(pic)
	if pic.Available == store.NoAvailable {
		scan.noAvailable++
		return nil
	}
	scan.available++
	return nil
}
