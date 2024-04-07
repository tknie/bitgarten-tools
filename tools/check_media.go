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
	"sync"
	"tux-lobload/sql"

	"github.com/tknie/log"
)

var checkPictureChannel = make(chan *sql.Picture, 10)
var stopCheck = make(chan bool)
var wgCheck sync.WaitGroup

func InitCheck() {
	for i := 0; i < 4; i++ {
		go pictureChecker()
	}
}

func CheckPicture(pic *sql.Picture) {
	wgCheck.Add(1)
	checkPictureChannel <- pic
}

func CheckMediaWait() {
	wgCheck.Wait()
}

func pictureChecker() {
	for {
		select {
		case pic := <-checkPictureChannel:
			log.Log.Debugf("Checking record %s %s", pic.ChecksumPicture, pic.Sha256checksum)

			switch {
			case len(pic.Media) == 0:
				fmt.Println(pic.ChecksumPicture + " Media empty")
				log.Log.Debugf("Error record len %s %s", pic.ChecksumPicture, pic.Sha256checksum)
			case sql.CreateMd5(pic.Media) != pic.ChecksumPicture:
				fmt.Println(pic.ChecksumPicture + " md5 error")
				log.Log.Debugf("Error md5  %s", sql.CreateMd5(pic.Media))
			case sql.CreateSHA(pic.Media) != pic.Sha256checksum:
				fmt.Println(pic.ChecksumPicture + " sha error")
				log.Log.Debugf("Error sha  %s", sql.CreateSHA(pic.Media))
			}
			wgCheck.Done()
		case <-stopCheck:
			return
		}
	}
}
