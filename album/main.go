/*
* Copyright © 2018-2019 private, Darmstadt, Germany and/or its licensors
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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tux-lobload/store"
)

// Directory to scan
var Directory string

func main() {
	var nrWorker int
	flag.StringVar(&Directory, "d", "", "Directory to search for album")
	flag.StringVar(&store.AlbumName, "a", "Album", "Album map name")
	flag.StringVar(&store.Credentials, "c", "admin:manage", "REST server user id and password")
	flag.StringVar(&store.PictureName, "p", "Picture", "Picture map name")
	flag.IntVar(&nrWorker, "t", 2, "Number of workers")
	flag.StringVar(&store.URL, "u", "http://localhost:8130/rest/map/", "URL of RESTful server")
	flag.Parse()

	if Directory == "" {
		fmt.Println("Need to add source flag")
		flag.Usage()
		return
	}
	w := store.InitWorker(nrWorker, evaluateWorker)

	filepath.Walk(Directory, func(path string, info os.FileInfo, err error) error {
		if info != nil {
			if !info.IsDir() && strings.Contains(path, "index.htm") {
				fmt.Println("===============\nWork on", path)
				w.Jobs <- path
			}
		}
		return nil
	})
	w.WaitEnd()
}

func evaluateWorker(job string) {
	store.EvaluateIndex(job)
}
