/*
* Copyright © 2023-2024 private, Darmstadt, Germany and/or its licensors
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

	"github.com/tknie/bitgarten-tools/tools"

	"github.com/tknie/log"
)

const description = `This tool checks additional characters like '<','>' or " are
included in the exif nameing.

`

func main() {

	tools.InitLogLevelWithFile("exifclean.log")

	tableName := ""
	flag.StringVar(&tableName, "t", "pictures", "Table name to search in")
	flag.Usage = func() {
		fmt.Print(description)
		fmt.Println("Default flags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	log.Log.Debugf("Start exifclean")
	tools.CleanExif(tableName)
}
