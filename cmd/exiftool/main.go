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

package main

import (
	"flag"
	"fmt"

	"github.com/tknie/bitgartentools"
	"github.com/tknie/bitgartentools/tools"
	"github.com/tknie/log"
)

const description = `This tool checks extract all EXIF data out of pictures
and stores it in data field.

`

func main() {
	tools.InitLogLevelWithFile("exiftool.log")
	limit := 0
	preFilter := ""
	json := false

	flag.IntVar(&limit, "l", 50, "Maximum number of records loaded")
	flag.StringVar(&preFilter, "f", "", "Prefix of title used in search")
	flag.BoolVar(&json, "j", false, "Output in JSON format")
	flag.Usage = func() {
		fmt.Print(description)
		fmt.Println("Default flags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	bitgartentools.InitTool("exifTool", json)
	var err error
	defer bitgartentools.FinalizeTool("exifTool", json, err)

	err = tools.ExifTool(&tools.ExifToolParameter{PreFilter: preFilter, Limit: limit})
	log.Log.Debugf("Exif tool error %v", err)
}
