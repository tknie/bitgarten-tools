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
	"encoding/json"
	"flag"
	"fmt"

	"github.com/tknie/bitgartentools"
	"github.com/tknie/bitgartentools/store"
	"github.com/tknie/bitgartentools/tools"
)

const description = `This tool generates image hashs.

`

type jsonInfo struct {
	Checksum string
	Status   string
}

func main() {
	tools.InitLogLevelWithFile("imagehash.log")

	limit := 10
	preFilter := ""
	deleted := false
	all := false
	hashType := tools.Hashes[tools.DefaultHash]
	jsonResult := false
	commit := false

	flag.IntVar(&limit, "l", 50, "Maximum number of records loaded")
	flag.StringVar(&preFilter, "f", "", "Prefix of title used in search")
	flag.BoolVar(&deleted, "D", false, "Scan deleted pictures as well")
	flag.BoolVar(&all, "A", false, "Scan all pictures (no limit to one week)")
	flag.BoolVar(&jsonResult, "j", false, "return output in JSON format")
	flag.BoolVar(&commit, "C", false, "commit all changes")
	flag.StringVar(&hashType, "h", tools.Hashes[tools.DefaultHash], "Hash type to use, valid are (averageHash,perceptHash,diffHash,waveletHash), default perceptHash")
	flag.Usage = func() {
		fmt.Print(description)
		fmt.Println("Default flags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	bitgartentools.InitTool("imageHash", jsonResult)
	var err error
	defer bitgartentools.FinalizeTool("imageHash", jsonResult, err)

	infoMap := make(map[string]any)
	list := make([]*jsonInfo, 0)
	if jsonResult {
		tools.InitHash(func(pic *store.Pictures, status string) {
			if pic != nil {
				list = append(list, &jsonInfo{Checksum: pic.ChecksumPicture, Status: status})
				infoMap["picture"] = list
			} else {
				infoMap["status"] = status
			}
		}, infoMap)
	} else {
		fmt.Println("Start Bitgarten hash generator to find doublikates of pictures")
		tools.InitHash(func(pic *store.Pictures, status string) {
			fmt.Println(status)
			if pic == nil {
				fmt.Println()
			}
		}, infoMap)
	}

	err = tools.ImageHash(&tools.ImageHashParameter{Limit: limit, HashType: hashType,
		Deleted: deleted, All: all, PreFilter: preFilter, Json: jsonResult, Commit: commit})
	if err != nil {
		fmt.Printf("Error generating image hash: %v\n", err)
	}
	if jsonResult {
		out, jerr := json.Marshal(infoMap)
		if jerr != nil {
			fmt.Printf("Marhsall JSON error: %v\n", jerr)
			return
		}
		fmt.Print("\"Result\":", string(out), ",")
	}
}
