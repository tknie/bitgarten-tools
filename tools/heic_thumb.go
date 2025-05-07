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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tknie/bitgartentools/sql"
	"github.com/tknie/bitgartentools/store"

	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

//var wid common.RegDbID
// var storeData bool

// var ref *common.Reference
//var passwd string

type HeicThumbParameter struct {
	Commit          bool
	CreateThumbnail bool
	WriteHandler    common.RegDbID
	Title           string
	ChkSum          string
	FromDate        string
	ToDate          string
	ScaleRange      int
}

func (parameter *HeicThumbParameter) HeicThumb() error {
	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return err
	}
	defer id.FreeHandler()

	fmt.Println("Start generating heic thumbnails")
	if parameter.Commit {
		wid, err := sql.DatabaseHandler()
		if err != nil {
			fmt.Println("Error connect write ...:", err)
			return err
		}
		parameter.WriteHandler = wid
		defer wid.FreeHandler()
	}

	q := &common.Query{TableName: "Pictures",
		DataStruct:   &store.Pictures{},
		Fields:       []string{"MIMEType", "checksumpicture", "title", "Media", "exiforigtime"},
		FctParameter: id,
	}

	prefix := "markdelete=false"
	if parameter.CreateThumbnail {
		prefix += " AND LOWER(title) LIKE '%heic'"
	} else {
		prefix += " AND (LOWER(title) LIKE '%heic' OR LOWER(title) LIKE '%mov')"
	}
	if parameter.ChkSum != "" {
		prefix += fmt.Sprintf(" AND checksumpicture = '%s'", parameter.ChkSum)
	}
	if parameter.Title != "" {
		prefix += fmt.Sprintf(" AND title = %s", parameter.Title)
	}
	if parameter.FromDate != "" {
		prefix += " AND created > '" + parameter.FromDate + "'"
	}
	if parameter.ToDate != "" {
		prefix += " AND created < '" + parameter.ToDate + " 23:59:59'"
	}
	q.Search = prefix
	q.FctParameter = parameter
	_, err = id.Query(q, generateQueryImageThumbnail)
	if err != nil {
		fmt.Println("Error query ...:", err)
		log.Log.Errorf("Error query thumbnail image: %v", err)
		return err
	}
	log.Log.Debugf("Generate of thumbnails done")
	if parameter.Commit {
		log.Log.Debugf("Commiting changes...")
		fmt.Println("Commiting changes ...")
		err = parameter.WriteHandler.Commit()
		if err != nil {
			fmt.Println("Error commiting data...", err)
			return err
		}
	}
	fmt.Println("Finished HEIC thumbnails generated")
	return nil
}

func generateQueryImageThumbnail(search *common.Query, result *common.Result) error {
	//id := search.FctParameter.(common.RegDbID)
	pic := result.Data.(*store.Pictures)
	parameter := search.FctParameter.(*HeicThumbParameter)
	return parameter.generateImageThumbnail(pic)
}

func (parameter *HeicThumbParameter) generateImageThumbnail(pic *store.Pictures) error {
	if parameter.CreateThumbnail {
		fmt.Println("Found and generate", pic.ChecksumPicture, pic.Title, pic.ExifOrigTime)
		err := pic.CreateThumbnail()
		if err != nil {
			fmt.Printf("Error creating thumbnail %s(%s): %v\n", pic.ChecksumPicture, pic.Title, err)
			return nil
		}
		fmt.Printf("%s -> %v\n", pic.Title, pic.ExifOrigTime)
		return parameter.storeThumb(pic)
	} else {
		parameter.searchSimilarEntries(pic.Title)
	}
	return nil
}

func (parameter *HeicThumbParameter) storeThumb(pic *store.Pictures) error {
	update := &common.Entries{
		Fields: []string{"exif", "Thumbnail",
			"exifmodel", "exifmake", "exiftaken", "exiforigtime",
			"exifxdimension", "exifydimension", "exiforientation",
			"GPScoordinates", "GPSlatitude", "GPSlongitude"},
		DataStruct: pic,
		Values:     [][]any{{pic}},
		Update:     []string{"checksumpicture='" + pic.ChecksumPicture + "'"},
	}
	if parameter.Commit {
		_, n, err := parameter.WriteHandler.Update("pictures", update)
		if err != nil {
			fmt.Println("Error updating", n, ":", err)
			fmt.Println("Pic:", pic.ChecksumPicture)
			fmt.Println(pic.Exif)
			return err
		}
		log.Log.Debugf("Update done, commiting now....")
		err = parameter.WriteHandler.Commit()
		if err != nil {
			fmt.Println("Error commiting", n, ":", err)
			return err
		}
	}
	return nil
}

func (parameter *HeicThumbParameter) searchSimilarEntries(title string) {
	if strings.HasPrefix(strings.ToUpper(title), "IMG") {
		return
	}
	ref, passwd, err := sql.DatabaseLocation()
	if err != nil {
		fmt.Println("Error getting connection reference:", err)
		return
	}
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
	extension := filepath.Ext(title)
	xTitle := strings.TrimSuffix(title, extension)

	q := &common.Query{TableName: "Pictures",
		DataStruct: &store.Pictures{},
		Fields:     []string{"MIMEType", "checksumpicture", "title", "exiforigtime"},
	}
	q.Search = "LOWER(title) LIKE '" + xTitle + "%' and markdelete=false"
	first := true
	_, err = sid.Query(q, func(search *common.Query, result *common.Result) error {
		pic := result.Data.(*store.Pictures)
		if pic.Title != title {
			if first {
				fmt.Println("Found", title)
			}
			first = false
			switch filepath.Ext(pic.Title) {
			case ".jpeg", ".heic":
				fmt.Println("  Deleting ", pic.Title, pic.ChecksumPicture, pic.MIMEType, pic.ExifOrigTime)
				if parameter.Commit {
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
				}
			case ".mov":
				fmt.Println("  Similar Movie ", pic.Title, pic.ChecksumPicture, pic.MIMEType, pic.ExifOrigTime)
			default:
				fmt.Println("  Similar ", pic.Title, pic.ChecksumPicture, pic.MIMEType, pic.ExifOrigTime)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error query ...:", err)
	}
	log.Log.Debugf("End query similar")
}

func (parameter *HeicThumbParameter) HeicScale() error {
	if parameter.Title == "" {
		fmt.Println("Album not set")
		return errors.New("album not given")
	}
	log.Log.Debugf("Scale Albums %s", parameter.Title)
	connSource, err := sql.DatabaseConnect()
	if err != nil {
		return err
	}

	a, err := connSource.ReadAlbum(parameter.Title)
	if err != nil {
		fmt.Println("Error reading album:", err)
		return err
	}
	a.Display()
	id, err := connSource.Open()
	if err != nil {
		fmt.Println("Error opening:", err)
		return err
	}
	defer id.FreeHandler()
	if parameter.Commit {
		err = id.BeginTransaction()
		if err != nil {
			fmt.Println("Error starting transaction:", err)
			return err
		}
	}
	for _, albumPicture := range a.Pictures {
		fmt.Println("Scale", albumPicture.Name+" "+albumPicture.Description+" "+albumPicture.ChecksumPicture+" "+albumPicture.MimeType)
		if !strings.HasPrefix(albumPicture.MimeType, "video/") {
			pic, err := connSource.ReadPicture(albumPicture.ChecksumPicture)
			if err != nil {
				fmt.Println("Error reading picture")
				id.Rollback()
				return err
			}
			err = pic.Resize(1280)
			if err != nil {
				fmt.Println("Resize of picture fails:", err)
				id.Rollback()
				return err
			}
			log.Log.Debugf("Resize picture %s->%s to %d,%d", pic.ChecksumPicture, albumPicture.ChecksumPicture, pic.Width, pic.Height)
			fmt.Printf("Resize picture %s->%s to %d,%d\n", pic.ChecksumPicture, albumPicture.ChecksumPicture, pic.Width, pic.Height)
			if parameter.Commit {
				// Store picture
				connSource.WritePictureTransaction(id, pic)

				albumPicture.ChecksumPicture = pic.ChecksumPicture
				albumPicture.Width = pic.Width
				albumPicture.Height = pic.Height
				// Store AlbumPicture
				list := [][]any{{albumPicture}}
				input := &common.Entries{
					Fields: []string{
						"checksumpicture", "width", "height",
					},
					DataStruct: albumPicture,
					Update: []string{fmt.Sprintf("index = %d AND albumid = %d",
						albumPicture.Index, albumPicture.AlbumId)},
					Criteria: fmt.Sprintf("index = %d and albumid = %d",
						albumPicture.Index, albumPicture.AlbumId),
					Values: list}
				_, _, err = id.Update("AlbumPictures", input)
				if err != nil {
					fmt.Println("Error inserting:", err)
					id.Rollback()
					return err
				}
				list = [][]any{{
					pic.ChecksumPicture,
					"bitgarten",
				}}
				input = &common.Entries{
					Fields: []string{
						"checksumpicture",
						"tagname",
					},
					Values: list}
				_, err = id.Insert("picturetags", input)
				if err != nil {
					fmt.Println("Error inserting:", err)
					id.Rollback()
					return err
				}
			}
		}
	}
	err = id.Commit()
	if err != nil {
		fmt.Println("Error commiting album:", err)
		return err
	}
	a.Display()
	return nil
}
