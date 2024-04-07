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
	"bytes"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"tux-lobload/sql"

	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

type PictureHash struct {
	Checksumpicture string
	PerceptionHash  string
}

type PictureHashCount struct {
	Count          int
	PerceptionHash string
}

const DefaultLimit = 20
const DefaultMinCount = 2

// var limit = DefaultLimit
// var minCount = DefaultMinCount
// var commit = false

const readHashs = `
SELECT count(perceptionhash) AS count,
perceptionhash
   FROM picturehash p
  WHERE (EXISTS ( SELECT 1
           FROM pictures pp
          WHERE pp.checksumpicture::text = p.checksumpicture::text AND pp.markdelete = false))
  GROUP BY perceptionhash
 HAVING count(perceptionhash) > {{.Count}}
  ORDER BY (count(perceptionhash)) DESC
  LIMIT {{.Limit}}
`

const readPictureByHashs = `
select checksumpicture, title, height, width, Exifxdimension, Exifydimension,
( SELECT string_agg(DISTINCT (''''::text || pt.tagname::text) || ''''::text, ','::text) AS string_agg
           FROM picturetags pt
          WHERE pt.checksumpicture::text = p.checksumpicture::text) AS tags
from pictures p where markdelete = false and EXISTS ( SELECT 1
	FROM picturehash pp
   WHERE pp.perceptionhash = {{.}} AND pp.checksumpicture::text = p.checksumpicture::text );
`

type HashCleanParameter struct {
	Limit    int
	MinCount int
	Commit   bool
}

func HashClean(parameter *HashCleanParameter) {
	fmt.Println("Query database entries for one week not hashed commit=", parameter.Commit)
	hashList, err := parameter.queryHash()
	if err != nil {
		fmt.Println("Error query max hash:", err)
		return
	}
	for i, h := range hashList {
		if h == "0" {
			fmt.Println("Breaking found empty hash")
			break
		}
		fmt.Printf("Working on %d.Hash %s\n", i+1, h)
		err = queryPictureByHash(h, parameter.Commit)
		if err != nil {
			fmt.Println("Error query max hash:", err)
			return
		}
	}
	fmt.Println("Final end")
}

func (parameter *HashCleanParameter) queryHash() ([]string, error) {
	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return nil, err
	}
	defer id.FreeHandler()

	sql, err := templateSql(readHashs, struct {
		Limit int
		Count int
	}{parameter.Limit, parameter.MinCount})
	if err != nil {
		return nil, err
	}
	query := &common.Query{
		TableName: "picturehash",
		//Fields:     []string{"count(perceptionhash) as count", "perceptionhash"},
		DataStruct: &PictureHashCount{},
		Limit:      uint32(parameter.Limit),
		Search:     sql,
	}
	counter := uint64(0)
	hash := uint64(0)
	hashList := make([]string, 0)
	err = id.BatchSelectFct(query, func(search *common.Query, result *common.Result) error {
		ph := result.Rows
		v := ph[1].(*string)
		log.Log.Debugf("Hash found: %v - %v", ph[0], v)
		hashList = append(hashList, *v)
		counter++
		return nil
	})
	if err != nil {
		fmt.Println("Error query ...:", err)
		return nil, err
	}
	log.Log.Debugf("Query hash end: %v -> %d, hash=%d", err, counter, hash)
	return hashList, nil
}

type PictureByHash struct {
	Checksumpicture string
	Title           string
	Height          int
	Width           int
	Exifxdimension  int
	Exifydimension  int
	Tags            string
	delete          bool `flynn:":ignore"`
}

func templateSql(t string, p any) (string, error) {
	t1 := template.New("t1")
	t1 = template.Must(t1.Parse(t))
	var sql bytes.Buffer
	err := t1.Execute(&sql, p)
	if err != nil {
		return "", err
	}
	return sql.String(), nil
}

func queryPictureByHash(hash string, commit bool) error {
	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return err
	}
	defer id.FreeHandler()

	sqlCmd, err := templateSql(readPictureByHashs, hash)
	if err != nil {
		return err
	}

	picturesByHash := make([]*PictureByHash, 0)

	query := &common.Query{
		TableName:  "pictures",
		Fields:     []string{"*"},
		DataStruct: &PictureByHash{},
		Search:     sqlCmd,
	}
	counter := uint64(0)
	err = id.BatchSelectFct(query, func(search *common.Query, result *common.Result) error {
		ph := result.Data.(*PictureByHash)
		log.Log.Debugf("Picture found: %#v", ph)
		newPH := &PictureByHash{}
		*newPH = *ph
		picturesByHash = append(picturesByHash, newPH)
		counter++
		return nil
	})
	if err != nil {
		fmt.Println("Error query ...:", err)
		return err
	}

	sort.SliceStable(picturesByHash, func(x, y int) bool {
		return picturesByHash[x].Width > picturesByHash[y].Width
	})
	fmt.Printf("Found %d picture hash entries\n", len(picturesByHash))
	var firstFound *PictureByHash
	tagMap := make(map[string]bool)
	for _, pbh := range picturesByHash {
		if strings.HasSuffix(strings.ToLower(pbh.Title), ".heic") {
			if firstFound == nil {
				firstFound = pbh
			} else {
				if firstFound.Width <= pbh.Width {
					if !strings.Contains(firstFound.Tags, "'bitgarten'") {
						firstFound.delete = true
					}
					firstFound = pbh
				}
			}
		} else {
			if !strings.Contains(pbh.Tags, "'bitgarten'") {
				if firstFound == nil {
					firstFound = pbh
				} else {
					pbh.delete = true
				}
				if pbh.Tags != "" {
					tags := strings.Split(pbh.Tags, ",")
					for _, t := range tags {
						tagMap[t] = true
					}
				}
			} else {
				if firstFound != nil && firstFound.Width == pbh.Width {
					firstFound.delete = true
					firstFound = pbh
				}
				if firstFound == nil {
					firstFound = pbh
				}
				if pbh.Tags != "" {
					tags := strings.Split(pbh.Tags, ",")
					for _, t := range tags {
						if t != "'bitgarten'" {
							tagMap[t] = true
						}
					}
				}
			}

			log.Log.Debugf("Find picture hash %#v", pbh)
		}
	}
	if firstFound == nil {
		fmt.Printf("No first found out of %d\n", len(picturesByHash))
	} else {
		fmt.Printf("Start cleanup for %s entries=%d\n", hash, len(picturesByHash))
		err = cleanUpPictures(commit, tagMap, firstFound, picturesByHash)
		if err != nil {
			fmt.Println("Error cleanup pictures:", err)
			return err
		}
	}
	log.Log.Debugf("Picture by hash end: %v -> %d", err, counter)
	return nil
}

func cleanUpPictures(commit bool, tagMap map[string]bool, firstFound *PictureByHash, picturesByHash []*PictureByHash) error {
	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return err
	}
	defer id.FreeHandler()

	err = id.BeginTransaction()
	if err != nil {
		return err
	}

	newTags := KeysString(tagMap)
	log.Log.Debugf("First: %#v -> %#v", firstFound, newTags)
	if newTags != firstFound.Tags {
		oldTagMap := KeysMap(firstFound.Tags)
		newTagMap := KeysMap(newTags)
		changed := false
		for k := range newTagMap {
			if k != "" {
				if !oldTagMap[k] {
					oldTagMap[k] = true
					changed = true

					fmt.Printf("Insert tag for %s to <%s> (%s)\n", firstFound.Checksumpicture, k, strings.Trim(k, "'"))
					input := &common.Entries{
						Fields: []string{"tagname", "checksumpicture"},
						Update: []string{"checksumpicture='" + firstFound.Checksumpicture + "'"},
						Values: [][]any{{strings.Trim(k, "'"), firstFound.Checksumpicture}},
					}
					_, err := id.Insert("picturetags", input)
					if err != nil {
						log.Log.Debugf("Error insert tag name %s for %s", k, firstFound.Checksumpicture)
						return err
					}
				}
			}
		}
		if changed {
			fmt.Println("Updated tag for", firstFound.Checksumpicture, "to", newTags, "from", firstFound.Tags)
		}
	}

	for _, pbh := range picturesByHash {
		log.Log.Debugf("Cleanup picture %#v", pbh)
		if pbh.delete {
			if pbh.Tags != "" {
				fmt.Println("Need to delete all tags for -> ", pbh.Checksumpicture, "tags=", pbh.Tags)
				log.Log.Debugf("Need to delete all tags for -> %s", pbh.Checksumpicture)
				dr, err := id.Delete("picturetags", &common.Entries{Criteria: "checksumpicture='" + pbh.Checksumpicture + "'"})
				if err != nil {
					return err
				}
				if dr == 0 {
					fmt.Printf("error deleting picture tags for %s: no entry deleted\n", pbh.Checksumpicture)
				}
				log.Log.Debugf("%d entries deleted", dr)

			}
			fmt.Println("Need to mark delete -> ", pbh.Checksumpicture)
			input := &common.Entries{
				Fields: []string{"markdelete"},
				Update: []string{"checksumpicture='" + pbh.Checksumpicture + "'"},
				Values: [][]any{{true}},
			}
			_, ra, err := id.Update("pictures", input)
			if err != nil {
				return nil
			}
			if ra != 1 {
				return fmt.Errorf("incorrect update mark delete of %s: %d", pbh.Checksumpicture, ra)
			}
			log.Log.Debugf("%d entries updated", ra)
		}
	}

	if commit {
		err = id.Commit()
		if err != nil {
			return err
		}
	} else {
		err = id.Rollback()
		if err != nil {
			return err
		}

	}

	return nil
}

func KeysMap(tags string) map[string]bool {
	keysMap := make(map[string]bool)
	for _, k := range strings.Split(tags, ",") {
		keysMap[k] = true
	}
	return keysMap
}

func KeysString(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ",")
}
