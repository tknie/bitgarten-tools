/*
* Copyright © 2018-2026 private, Darmstadt, Germany and/or its licensors
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
package bitgartentools

import (
	"fmt"
	"time"

	"github.com/tknie/services"
)

// TimeFormat time formating schema
const TimeFormat = "2006-01-02 15:04:05"

func InitTool(toolName string, json bool) {
	if json {
		fmt.Printf("{\"start\":\"%s\",\"tool\":\"%s\",", time.Now().Format(TimeFormat), toolName)
		return
	}
	services.ServerMessage("STARTING tool '%s'\n", toolName)
}

func FinalizeTool(toolName string, json bool, err error) {
	if err != nil {
		if json {
			fmt.Printf("\"error\":\"%v\",\"end\": \"%s\"}", err, time.Now().Format(TimeFormat))
			return
		}
		fmt.Printf("%s: CANCELED tool '%s' with error: %v\n", time.Now().Format(TimeFormat), toolName, err)
		return
	}
	if json {
		fmt.Printf("\"end\": \"%s\"}\n", time.Now().Format(TimeFormat))
		return
	}
	fmt.Printf("%s: ENDED tool '%s'\n", time.Now().Format(TimeFormat), toolName)

}
