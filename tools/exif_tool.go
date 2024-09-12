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
	"strconv"
	"strings"

	"github.com/tknie/bitgarten-tools/sql"
	"github.com/tknie/bitgarten-tools/store"

	"github.com/tknie/flynn/common"
)

type ExifToolParameter struct {
	PreFilter string
	Limit     int
}

func ExifTool(parameter *ExifToolParameter) {

	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return
	}
	wid, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return
	}
	if parameter.PreFilter != "" {
		parameter.PreFilter = fmt.Sprintf(" AND LOWER(title) LIKE '%s%%'", parameter.PreFilter)
	}
	count := uint64(0)
	skipped := uint64(0)
	query := &common.Query{
		TableName:  "pictures",
		Fields:     []string{"ChecksumPicture", "title", "mimetype", "media"},
		DataStruct: &store.Pictures{},
		Limit:      strconv.Itoa(parameter.Limit),
		Search:     "mimetype LIKE 'image/%' AND GPScoordinates IS NULL" + parameter.PreFilter,
	}
	_, err = id.Query(query, func(search *common.Query, result *common.Result) error {
		p := result.Data.(*store.Pictures)
		if (skipped+count)%100 == 0 {
			fmt.Printf("Extract and store exif on %d records, skipped are %d\r", count, skipped)
		}
		err := p.ExifReader()
		if err != nil {
			skipped++
			return nil
		}
		p.Exif = strings.ReplaceAll(p.Exif, "\\", "\\\\")
		count++
		insert := &common.Entries{
			Fields:     []string{"exif", "GPScoordinates", "GPSlatitude", "GPSlongitude"},
			DataStruct: p,
			Values:     [][]any{{p}},
			Update:     []string{"checksumpicture='" + p.ChecksumPicture + "'"},
		}
		_, n, err := wid.Update("pictures", insert)
		if err != nil {
			fmt.Println("Error inserting", n, ":", err)
			fmt.Println("Pic:", p.ChecksumPicture)
			fmt.Println(p.Exif)
			return err
		}
		return nil
	})
	if err != nil {
		fmt.Println("Query error:", err)
	}
	fmt.Println()
	fmt.Printf("Finally worked on %d records and %d are skipped\n", count, skipped)
}
