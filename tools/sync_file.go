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
	"os"

	"github.com/tknie/bitgartentools/sql"
	"github.com/tknie/flynn/common"
)

// SyncTableParameter synchronize/copy one table data to another
type SyncTableParameter struct {
	SourceTable      string
	DestTable        string
	ListSourceTables bool
	ListDestTables   bool
	Commit           bool
}

func SyncTable(parameter *SyncTableParameter) error {
	connSource, err := sql.DatabaseConnect()
	if err != nil {
		fmt.Println("Error creating connection:", err)
		fmt.Println("Set POSTGRES_URL and/or POSTGRES_PASSWORD to define remote database")
		return err
	}

	if parameter.ListSourceTables {
		fmt.Println("List source tables...", connSource.Reference)
		_, _ = connSource.ListTables()
		return nil
	}

	destUrl := os.Getenv("POSTGRES_DESTINATION_URL")
	pwd := os.Getenv("POSTGRES_DESTINATION_PASSWORD")
	destSource, err := sql.Connect(destUrl, pwd)
	if err != nil {
		fmt.Println("Error creating connection:", err)
		fmt.Println("Set POSTGRES_DESTINATION_URL and/or POSTGRES_DESTINATION_PASSWORD to define remote database")
		return err
	}

	if parameter.ListDestTables {
		fmt.Println("List destination tables...", destSource.Reference)
		_, _ = destSource.ListTables()
		return nil
	}

	sourceFields, err := getTableFields(connSource, parameter.SourceTable)
	if err != nil {
		fmt.Println("Error getting table fields from source:", err)
		return err
	}

	fmt.Println("Get source column names:", sourceFields)
	destFields, err := getTableFields(destSource, parameter.DestTable)
	if err != nil {
		fmt.Println("Error getting table fields from destination:", err)
		return err
	}
	fmt.Println("Get source column names:", destFields)

	search := os.Getenv("SYNC_FILE_SEARCH")
	if search != "" {
		fmt.Println("Used search ...:", search)
	}

	q := &common.Query{TableName: parameter.SourceTable,
		Search: search,
		Fields: sourceFields,
	}

	count := 0
	_, err = connSource.Query(q, func(search *common.Query, result *common.Result) error {
		// fmt.Println("Entry:", result.Rows)
		count++

		if count%1000 == 0 {
			fmt.Println("Copied records:", count)
		}

		if parameter.Commit {
			input := &common.Entries{Fields: sourceFields,
				Values: [][]any{result.Rows}}
			_, err = destSource.Insert(parameter.DestTable, input)
			return err
		}
		return nil
	})
	if err != nil {
		fmt.Println("Query error:", err)
	}
	fmt.Println("Query reads records:", count)
	return nil
}

func getTableFields(conn *sql.DatabaseInfo, name string) ([]string, error) {
	id, err := conn.Open()
	if err != nil {
		fmt.Println("Error opening source:", err)
		return nil, err
	}
	fields, err := id.GetTableColumn(name)
	if err != nil {
		fmt.Println("Error opening source:", err)
		return nil, err
	}
	return fields, nil
}
