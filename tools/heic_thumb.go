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
	"path/filepath"
	"strings"
	"tux-lobload/sql"
	"tux-lobload/store"

	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

//var wid common.RegDbID
// var storeData bool

var ref *common.Reference
var passwd string

type HeicThumbParameter struct {
	Commit       bool
	WriteHandler common.RegDbID
	Title        string
	ChkSum       string
}

func HeicThumb(parameter *HeicThumbParameter) {
	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return
	}
	defer id.FreeHandler()

	fmt.Println("Start generating heic thumbnails")
	if parameter.Commit {
		wid, err := sql.DatabaseHandler()
		if err != nil {
			fmt.Println("Error connect write ...:", err)
			return
		}
		defer wid.FreeHandler()
	}

	q := &common.Query{TableName: "Pictures",
		DataStruct:   &store.Pictures{},
		Fields:       []string{"MIMEType", "checksumpicture", "title", "Media", "exiforigtime"},
		FctParameter: id,
	}

	prefix := "title LIKE '%heic' AND markdelete=false"
	if parameter.ChkSum != "" {
		prefix += fmt.Sprintf(" AND checksumpicture = '%s'", parameter.ChkSum)
	}
	if parameter.Title != "" {
		prefix += fmt.Sprintf(" AND title = %s", parameter.Title)
	}
	q.Search = prefix
	q.FctParameter = parameter
	_, err = id.Query(q, generateQueryImageThumbnail)
	if err != nil {
		fmt.Println("Error query ...:", err)
		return
	}
	if parameter.Commit {
		fmt.Println("Commiting changes ...")
		err = parameter.WriteHandler.Commit()
		if err != nil {
			fmt.Println("Error commiting data...", err)
			return
		}
	}
	fmt.Println("Finished heic thumbnails generated")
}

func generateQueryImageThumbnail(search *common.Query, result *common.Result) error {
	id := search.FctParameter.(common.RegDbID)
	pic := result.Data.(*store.Pictures)
	parameter := search.FctParameter.(*HeicThumbParameter)
	return parameter.generateImageThumbnail(id, pic)
}

func (parameter *HeicThumbParameter) generateImageThumbnail(id common.RegDbID, pic *store.Pictures) error {
	if parameter.Commit {
		fmt.Println("Found and generate", pic.ChecksumPicture, pic.Title, pic.ExifOrigTime)
		err := pic.CreateThumbnail()
		if err != nil {
			fmt.Println("Error creating thumbnail:", err)
			return err
		}
		fmt.Printf("%s -> %v\n", pic.Title, pic.ExifOrigTime)
		return parameter.storeThumb(pic)
	} else {
		fmt.Println("Found", pic.ChecksumPicture, pic.Title, pic.ExifOrigTime)
		searchSimilarEntries(pic.Title)
	}
	return nil
}

func (parameter *HeicThumbParameter) storeThumb(pic *store.Pictures) error {
	update := &common.Entries{
		Fields:     []string{"exif", "Thumbnail", "exifmodel", "exifmake", "exiftaken", "exiforigtime", "exifxdimension", "exifydimension", "exiforientation", "GPScoordinates", "GPSlatitude", "GPSlongitude"},
		DataStruct: pic,
		Values:     [][]any{{pic}},
		Update:     []string{"checksumpicture='" + pic.ChecksumPicture + "'"},
	}
	_, n, err := parameter.WriteHandler.Update("pictures", update)
	if err != nil {
		fmt.Println("Error updating", n, ":", err)
		fmt.Println("Pic:", pic.ChecksumPicture)
		fmt.Println(pic.Exif)
		return err
	}

	return nil
}

func searchSimilarEntries(title string) {
	sid, err := flynn.Handler(ref, passwd)
	if err != nil {
		fmt.Println("Error connect ...:", err)
		return
	}
	defer sid.FreeHandler()
	log.Log.Debugf("SID ID:%d", sid)
	did, err := flynn.Handler(ref, passwd)
	if err != nil {
		fmt.Println("Error connect ...:", err)
		return
	}
	defer did.FreeHandler()
	log.Log.Debugf("DID ID:%d", did)
	xTitle := strings.TrimSuffix(title, filepath.Ext(title))

	q := &common.Query{TableName: "Pictures",
		DataStruct: &store.Pictures{},
		Fields:     []string{"MIMEType", "checksumpicture", "title", "exiforigtime"},
	}
	q.Search = "title LIKE '" + xTitle + "%' and markdelete=false"
	_, err = sid.Query(q, func(search *common.Query, result *common.Result) error {
		pic := result.Data.(*store.Pictures)
		if pic.Title != title {
			if filepath.Ext(pic.Title) == ".jpeg" {
				fmt.Println("  Deleting ", pic.ChecksumPicture, pic.Title, pic.MIMEType, pic.ExifOrigTime)
				update := &common.Entries{
					Fields: []string{"markdelete"},
					Values: [][]any{{true}},
					Update: []string{"checksumpicture='" + pic.ChecksumPicture + "'"},
				}
				_, n, err := did.Update("pictures", update)
				if err != nil {
					fmt.Println("Error mark delete", n, ":", err)
					fmt.Println("Pic:", pic.ChecksumPicture)
					return err
				}
			} else {
				fmt.Println("  ", pic.ChecksumPicture, pic.Title, pic.MIMEType, pic.ExifOrigTime)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error query ...:", err)
	}
	log.Log.Debugf("End query similar")
}
