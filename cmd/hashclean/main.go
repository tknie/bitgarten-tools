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

	"github.com/tknie/bitgarten-tools/sql"
	"github.com/tknie/bitgarten-tools/tools"
)

const description = `This tool checks creates similar hash for all types
and all entries not updated.
In addition the tool does cleanup HEIC scaled photos.

`

func main() {
	tools.InitLogLevelWithFile("hashclean.log")
	var limit int
	var minCount int
	var heicclean bool
	var commit bool

	flag.IntVar(&limit, "l", tools.DefaultLimit, "Maximum number of records loaded")
	flag.IntVar(&minCount, "m", tools.DefaultMinCount, "Minimum number of count per hash")
	//	flag.StringVar(&hashType, "h", "", "Hash type to use, valid are (averageHash,perceptHash,diffHash,waveletHash), default perceptHash")
	flag.BoolVar(&commit, "c", false, "Enable commit to database")
	flag.BoolVar(&sql.ExitOnError, "E", false, "Exit if an error happens")
	flag.BoolVar(&heicclean, "H", false, "Cleanup heic images")
	flag.Usage = func() {
		fmt.Print(description)
		fmt.Println("Default flags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if heicclean {
		tools.HeicClean(&tools.HashCleanParameter{Limit: limit, MinCount: minCount, Commit: commit})
	} else {
		tools.HashClean(&tools.HashCleanParameter{Limit: limit, MinCount: minCount, Commit: commit})
	}
}
