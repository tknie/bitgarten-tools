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
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"slices"
	"strings"
	"text/template"

	"github.com/tknie/bitgarten-tools/sql"
	"github.com/tknie/bitgarten-tools/store"

	"github.com/corona10/goimagehash"
	"github.com/tknie/flynn/common"
	"github.com/tknie/goheif"
	"github.com/tknie/log"
)

var DefaultHash = 1
var Hashes = []string{"averageHash", "perceptHash", "diffHash", "waveletHash"}

const searchHash = `{{if not .Deleted -}} markdelete = false AND {{end}}
mimetype LIKE 'image/%' {{.Filter}}
AND NOT EXISTS(SELECT 1 FROM picturehash ph 
	WHERE ph.checksumpicture = tn.checksumpicture and 
		ph.updated_at < current_date + interval '1 week')`

type hashData struct {
	Checksumpicture string
	Hash            uint64
	Averagehash     uint64
	PerceptionHash  uint64
	DifferenceHash  uint64
	Kind            byte
}

type ImageHashParameter struct {
	Limit     int
	PreFilter string
	Deleted   bool
	HashType  string
}

func ImageHash(parameter *ImageHashParameter) {

	if parameter.PreFilter != "" {
		parameter.PreFilter = fmt.Sprintf(" AND LOWER(title) LIKE '%s%%'", parameter.PreFilter)
	}

	if !slices.Contains(Hashes, parameter.HashType) {
		fmt.Println("Incorrect hash parameter given:", parameter.HashType, "not in", Hashes)
		return
	}

	// Prepare template
	t1 := template.New("t1")
	t1 = template.Must(t1.Parse(searchHash))
	var sqlCmd bytes.Buffer
	t1.Execute(&sqlCmd, struct {
		Deleted bool
		Filter  string
	}{parameter.Deleted, parameter.PreFilter})

	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return
	}
	fmt.Println("Query database entries for one week not hashed")
	log.Log.Debugf("Execute query:\n%s\n", sqlCmd.String())
	query := &common.Query{
		TableName:  "pictures",
		Fields:     []string{"ChecksumPicture", "title", "mimetype", "media"},
		DataStruct: &store.Pictures{},
		Limit:      uint32(parameter.Limit),
		Search:     sqlCmd.String(),
	}
	counter := uint64(0)
	processed := uint64(0)
	_, err = id.Query(query, func(search *common.Query, result *common.Result) error {
		counter++
		p := result.Data.(*store.Pictures)
		buffer := bytes.NewBuffer(p.Media)
		var hd *hashData
		switch strings.ToLower(p.MIMEType) {
		case "image/heic":
			hd, err = hashHeic(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		case "image/jpeg", "image/jpg":
			hd, err = hashJpeg(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		case "image/png":
			hd, err = hashPng(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		case "image/gif":
			hd, err = hashGif(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		default:
			fmt.Printf("Error unknown image format for %s/%s: %s\n", p.Title, p.ChecksumPicture, p.MIMEType)
			log.Log.Errorf("Error unknown image format for %s/%s: %s\n", p.Title, p.ChecksumPicture, p.MIMEType)
			return nil
		}
		hd.Checksumpicture = p.ChecksumPicture
		hd.Hash = hd.PerceptionHash
		fmt.Printf("%s -> %s\n", p.Title, hd.Checksumpicture)
		err = insertHash(p, hd)
		if err == nil {
			processed++
		}
		return nil
	})
	if err != nil {
		fmt.Println("Query error:", err)
	}
	fmt.Printf("Found %d pictures where %d pictures are hashed", counter, processed)
	fmt.Println()
}

func insertHash(p *store.Pictures, ph *hashData) error {
	wid, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return nil
	}
	insert := &common.Entries{
		Fields:     []string{"checksumpicture", "hash", "averagehash", "perceptionHash", "differenceHash", "kind"},
		DataStruct: ph,
		// Values:     [][]any{{p.ChecksumPicture, h.GetHash(), byte(h.GetKind())}},
		Values: [][]any{{ph}},
	}
	_, err = wid.Insert("picturehash", insert)
	if err != nil {
		fmt.Printf("Error inserting %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
		return err
	}
	return nil
}

func hashHeic(f io.Reader) (*hashData, error) {
	i, err := goheif.Decode(f)
	if err != nil {
		return nil, err
	}
	return generateHash(i)
}

func generateHash(i image.Image) (*hashData, error) {
	ph := &hashData{}
	h, err := hash(i, Hashes[0])
	if err != nil {
		return nil, err
	}
	ph.Averagehash = h.GetHash()
	h, err = hash(i, Hashes[1])
	if err != nil {
		return nil, err
	}
	ph.PerceptionHash = h.GetHash()
	h, err = hash(i, Hashes[2])
	if err != nil {
		return nil, err
	}
	ph.DifferenceHash = h.GetHash()
	return ph, nil
}

func hash(i image.Image, hType string) (*goimagehash.ImageHash, error) {
	switch hType {
	case Hashes[0]:
		return goimagehash.AverageHash(i)
	case Hashes[1]:
		return goimagehash.PerceptionHash(i)
	case Hashes[2]:
		return goimagehash.DifferenceHash(i)
	case Hashes[3]:
		fmt.Println("Wavelet not yet support by system")
	}
	return nil, fmt.Errorf("unknown hashType")
}

func hashJpeg(f io.Reader) (*hashData, error) {
	i, err := jpeg.Decode(f)
	if err != nil {
		return nil, err
	}
	return generateHash(i)
}

func hashPng(f io.Reader) (*hashData, error) {
	i, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return generateHash(i)
}

func hashGif(f io.Reader) (*hashData, error) {
	i, err := gif.Decode(f)
	if err != nil {
		return nil, err
	}
	return generateHash(i)
}
