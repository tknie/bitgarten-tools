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
package tools

import (
	"fmt"
	"strings"

	"github.com/tknie/bitgartentools/sql"

	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

type exif struct {
	Checksumpicture string
	Exifmodel       string
	Exifmake        string
}

func CleanExif(tableName string) error {

	log.Log.Debugf("Start exifclean")

	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return err
	}
	wid, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return err
	}
	query := &common.Query{
		TableName:  tableName,
		DataStruct: &exif{},
		Fields:     []string{"exifmodel", "exifmake", "checksumpicture"},
	}
	count := int64(0)
	r, err := id.Query(query, func(search *common.Query, result *common.Result) error {
		x := result.Data.(*exif)
		if strings.HasPrefix(x.Exifmodel, "\"") || strings.HasPrefix(x.Exifmodel, "<") ||
			strings.HasPrefix(x.Exifmake, "\"") || strings.HasPrefix(x.Exifmake, "<") {
			toModel := strings.Trim(x.Exifmodel, "\"")
			toModel = strings.Trim(toModel, "<>")
			toModel = strings.Trim(toModel, " ")
			fmt.Printf("MODEL: %s: <%s> -> <%s>\n", x.Checksumpicture, x.Exifmodel, toModel)
			x.Exifmodel = toModel
			toModel = strings.Trim(x.Exifmake, "\"")
			toModel = strings.Trim(toModel, "<>")
			toModel = strings.Trim(toModel, " ")
			fmt.Printf("MAKE : %s: <%s> -> <%s>\n", x.Checksumpicture, x.Exifmake, toModel)
			x.Exifmake = toModel
			list := [][]any{{x}}
			update := &common.Entries{Fields: []string{"exifmodel", "exifmake"},
				DataStruct: x,
				Values:     list,
				Update:     []string{"checksumpicture = '" + x.Checksumpicture + "'"},
			}
			_, n, err := wid.Update(tableName, update)
			if err != nil {
				fmt.Println("Error updating record:", err)
				return err
			}
			err = wid.Commit()
			if err != nil {
				fmt.Println("Error commiting record:", err)
				return err
			}
			count += n
		}
		return nil
	})
	if err != nil {
		fmt.Println("Aborted with error:", err)
		return err
	}
	fmt.Println("Updates: ", count, r.Counter)
	return nil
}
