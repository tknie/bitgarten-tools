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

	"github.com/tknie/bitgarten-tools/tools"
)

func main() {
	tools.InitLogLevelWithFile("imagehash.log")

	limit := 10
	preFilter := ""
	deleted := false
	hashType := tools.Hashes[tools.DefaultHash]

	flag.IntVar(&limit, "l", 50, "Maximum number of records loaded")
	flag.StringVar(&preFilter, "f", "", "Prefix of title used in search")
	flag.BoolVar(&deleted, "D", false, "Scan deleted pictures as well")
	flag.StringVar(&hashType, "h", tools.Hashes[tools.DefaultHash], "Hash type to use, valid are (averageHash,perceptHash,diffHash,waveletHash), default perceptHash")
	flag.Parse()

	fmt.Println("Start Bitgarten hash generator to find doublikates of pictures")

	tools.ImageHash(&tools.ImageHashParameter{Limit: limit, HashType: hashType,
		Deleted: deleted, PreFilter: preFilter})

}
