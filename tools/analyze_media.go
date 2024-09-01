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

	"github.com/tknie/bitgarten-tools/sql"
	"github.com/tknie/bitgarten-tools/store"
	"github.com/tknie/log"
)

var noAvailable = uint64(0)
var available = uint64(0)
var countAll = uint64(0)
var countErrors = uint64(0)
var countEmpty = uint64(0)

func AnalyzeDirectories(directories []string) {
	checker, err := sql.CreateConnection()
	if err != nil {
		log.Log.Fatalf("Database connection not established: %v", err)
	}
	defer checker.Close()
	for _, pictureDirectory := range directories {
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
				loadFile(checker, path)
			default:
			}

			return nil
		})
		if err != nil {
			fmt.Println("Error working in directories:", err)
			return
		}
	}

	fmt.Printf("%-20s: %d\n", "New pictures", noAvailable)
	fmt.Printf("%-20s: %d\n", "Pictures registered", available)
	fmt.Printf("%-20s: %d\n", "Pictures found", countAll)
	fmt.Printf("%-20s: %d\n", "Pictures errors", countErrors)
	fmt.Printf("%-20s: %d\n", "Pictures empty", countEmpty)
}

func loadFile(db *sql.DatabaseInfo, fileName string) error {
	countAll++
	pic := &store.Pictures{}
	f, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Open file error", fileName, ":", err)
		countErrors++
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		countErrors++
		return err
	}
	if fi.Size() == 0 {
		countEmpty++
		return fmt.Errorf("file empty %s", fileName)
	}
	pic.Media = make([]byte, fi.Size())
	var n int
	n, err = f.Read(pic.Media)
	log.Log.Debugf("Number of bytes read: %d/%d -> %v\n", n, len(pic.Media), err)
	if err != nil {
		countErrors++
		return err
	}
	pic.ChecksumPicture = store.CreateMd5(pic.Media)
	pic.ChecksumPictureSHA = store.CreateSHA(pic.Media)

	db.CheckExists(pic)
	if pic.Available == store.NoAvailable {
		noAvailable++
		return nil
	}
	available++
	return nil
}
