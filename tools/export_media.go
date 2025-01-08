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
	"os"
	"path/filepath"
	"strconv"

	"github.com/tknie/bitgartentools/sql"
	"github.com/tknie/bitgartentools/store"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

const exportTimeFormat = "2006-01-02"

type ExportMediaParameter struct {
	Directory string
	Limit     int
}

func ExportMedia(parameter *ExportMediaParameter) error {
	if parameter.Directory == "" {
		parameter.Directory = "./"
	}
	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("Error connect ...:", err)
		return err
	}
	defer id.FreeHandler()

	limit := "ALL"
	if parameter.Limit > 0 {
		limit = strconv.Itoa(parameter.Limit)
	}

	q := &common.Query{TableName: "Pictures",
		DataStruct:   &store.Pictures{},
		Limit:        limit,
		FctParameter: parameter,
		Fields: []string{"MIMEType", "title", "exiforigtime",
			"checksumpicture", "Media"},
	}
	log.Log.Debugf("Call batch ...")
	_, err = id.Query(q, writeMediaFile)
	if err != nil {
		log.Log.Errorf("Error video title query: %v", err)
		fmt.Println("Error exporting media query ...:", err)
		return err
	}
	log.Log.Debugf("Call batch done ...")
	return nil
}

func writeMediaFile(search *common.Query, result *common.Result) error {
	parameter := search.FctParameter.(*ExportMediaParameter)
	pic := result.Data.(*store.Pictures)
	fmt.Printf("Write Media file %s/%s/%s\n", pic.ExifOrigTime.Format(exportTimeFormat), pic.Title,
		pic.ChecksumPicture)
	filename := fmt.Sprintf("%s/%s/%s/%s", parameter.Directory,
		pic.ExifOrigTime.Format(exportTimeFormat), pic.Title,
		pic.ChecksumPicture)
	dirname := filepath.Dir(filename)
	fmt.Println("Create directory:", dirname)
	os.MkdirAll(dirname, 0700)
	err := os.WriteFile(filename, pic.Media, 0644)

	return err
}
