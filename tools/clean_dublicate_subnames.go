/*
* Copyright © 2024 private, Darmstadt, Germany and/or its licensors
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
	"math/big"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tknie/bitgartentools/sql"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

const checkQuery = `
with titleTable (title,
subtitle) as (
select
	title,
	replace(replace(replace(title ,
	'_4_5005_c.jpeg',
	''),'_1_105_c.jpeg',''),'_1_201_a.heic','')
from
	pictures
where
	(title like '%_4_5005_c.jpeg%' or title like '%_1_105_c.jpeg%' or title like '%_1_201_a.heic%'){{.}})
select * from
(select
	pt.checksumpicture,
	pt.markdelete,
	pt.title,
	pt.width ,
	pt.height ,
	tt.subtitle,
	ph.perceptionhash,
	string_agg(a.tagname::text, ','::text ORDER BY (a.tagname::text)) AS tags,
	count (*) over (partition by ph.perceptionhash order by ph.perceptionhash) as cnt,
	row_number () over (partition by ph.perceptionhash order by ph.perceptionhash) as r
from
	titletable tt,
	pictures pt,
	picturehash ph
	LEFT JOIN picturetags a USING (checksumpicture)
where
	pt.title like concat(tt.subtitle,
	'%')
	and pt.checksumpicture = ph.checksumpicture and markdelete=false
group by
	ph.perceptionhash,
	pt.checksumpicture,
	pt.markdelete,
	pt.title,
	pt.width ,
	pt.height ,
	tt.subtitle
order by
	perceptionhash,
	width desc) where cnt > 1;` // and r > 1;`

type NameCleanParameter struct {
	Limit    int
	MinCount int
	Title    string
	Commit   bool
	Json     bool
}

var did common.RegDbID

func NameClean(parameter *NameCleanParameter) error {
	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return err
	}
	defer id.FreeHandler()

	did, err = sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return err
	}
	defer did.FreeHandler()

	search := ""
	if parameter.Title != "" {
		search = "AND title like '" + parameter.Title + "%'"
	}
	search, _ = templateSql(checkQuery, search)
	log.Log.Debugf("Search : %s", search)

	query := &common.Query{
		TableName: "picturehash",
		Limit:     strconv.Itoa(parameter.Limit),
		Search:    search,
	}
	counter := uint64(0)
	deleted := uint64(0)
	var currentHash pgtype.Numeric
	subtitle := ""
	err = id.BatchSelectFct(query, func(search *common.Query, result *common.Result) error {
		index := result.GetRowValueByName("r").(int64)
		tags := result.GetRowValueByName("tags")
		if index == 1 {
			currentHash = result.GetRowValueByName("perceptionhash").(pgtype.Numeric)
			subtitle = result.GetRowValueByName("subtitle").(string)
		} else {
			if currentHash.Int.Cmp(big.NewInt(0)) > 0 {
				recordHash := result.GetRowValueByName("perceptionhash").(pgtype.Numeric)
				s := result.GetRowValueByName("subtitle").(string)
				if strings.HasPrefix(s, subtitle) {
					if recordHash.Int.Cmp(currentHash.Int) == 0 {
						if tags == nil {
							deleted++
							if parameter.Commit {
								checksumPicture := result.GetRowValueByName("checksumPicture").(string)
								markDelete(checksumPicture)
							}
						}
						if !parameter.Json {
							fmt.Println(subtitle, "Deleted:", deleted, "Tags:", tags, "Hash:", currentHash)
						}
					} else {
						if !parameter.Json {
							fmt.Println("Record hash differences:", recordHash, currentHash)
						}
					}
				}
			}
		}
		counter++
		return nil
	})
	if err != nil {
		return err
	}
	if parameter.Json {
		fmt.Printf("\"deleted\":%d,\"counter\":%d,", deleted, counter)
	} else {
		fmt.Println("Total count of records deleted:", deleted)
		fmt.Println("Total count of records found:", counter)
	}
	return nil
}

func markDelete(checksumPicture string) error {
	update := &common.Entries{
		Fields: []string{"markdelete"},
		Values: [][]any{{true}},
		Update: []string{"checksumpicture='" + checksumPicture + "'"},
	}
	_, n, err := did.Update("pictures", update)
	if err != nil {
		fmt.Println("Error mark delete", n, ":", err)
		fmt.Println("Pic:", checksumPicture)
		return err
	}
	fmt.Println(checksumPicture, "mark deleted")
	return nil
}
