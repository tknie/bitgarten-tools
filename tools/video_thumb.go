/*
* Copyright © 2023-2024 private, Darmstadt, Germany and/or its licensors
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
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"

	"github.com/tknie/bitgarten-tools/sql"
	"github.com/tknie/bitgarten-tools/store"

	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

type Albums struct {
	Id          uint64
	Type        string
	Key         string
	Directory   string
	Title       string
	Description string
}

var gid = common.RegDbID(0)

const SELECT_ALBUM = `with albumIdSelect(Id) as ( SELECT Id FROM Albums WHERE Title = '%s'), checksumSelect as (  
	SELECT ChecksumPicture FROM AlbumPictures, albumIdSelect WHERE albumid = albumIdSelect.Id AND MIMEType LIKE 'video%%')
	SELECT Pictures.ChecksumPicture,MIMEType,Media FROM Pictures, checksumSelect WHERE Pictures.checksumpicture = checksumSelect.ChecksumPicture`

type VideoThumbParameter struct {
	Title  string
	ChkSum string
}

func VideoThumb(parameter *VideoThumbParameter) {
	id, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("Error connect ...:", err)
		return
	}
	gid = id
	q := &common.Query{TableName: "Pictures",
		DataStruct:   &store.Pictures{},
		Fields:       []string{"MIMEType", "checksumpicture", "Media"},
		FctParameter: id,
	}
	if parameter.Title != "" {
		// prefix = searchTitle(title, id)
		prefix := fmt.Sprintf(SELECT_ALBUM, parameter.Title)
		if prefix == "" {
			log.Log.Fatal("Error evaluating album id...", prefix)
		}
		q.Search = prefix
		err = id.BatchSelectFct(q, generateQueryVideoThumbnail)
		if err != nil {
			fmt.Println("Error query ...:", err)
			return
		}
	} else {
		prefix := "MIMEType LIKE 'video%'"
		if parameter.ChkSum != "" {
			cprefix := fmt.Sprintf("title = %s AND ", parameter.Title)
			prefix = cprefix + prefix
		}
		q.Search = prefix
		_, err = id.Query(q, generateQueryVideoThumbnail)
		if err != nil {
			fmt.Println("Error query ...:", err)
			return
		}
	}
	fmt.Println("video thumbnail generated")
}

func generateQueryVideoThumbnail(search *common.Query, result *common.Result) error {
	id := search.FctParameter.(common.RegDbID)
	pic := result.Data.(*store.Pictures)
	return generateVideoThumbnail(id, pic)
}

func generateVideoThumbnail(id common.RegDbID, pic *store.Pictures) error {
	fmt.Println("MIMEtype", pic.MIMEType, pic.ChecksumPicture)
	err := os.Remove("input.mp4")
	if err != nil && !os.IsNotExist(err) {
		fmt.Println("Error removing:", err)
		return err
	}
	err = os.WriteFile("file.mp4", pic.Media, 0644)
	if err != nil {
		fmt.Println("Error removing:", err)
		return err
	}
	err = storeThumb(pic.ChecksumPicture, pic)
	if err != nil {
		if err == io.EOF {
			return nil
		}
		fmt.Println("Error preparing storage:", err)
		return err
	}

	if pic.Thumbnail == nil && len(pic.Thumbnail) == 0 {
		log.Log.Fatalf("Thumbnail empty")
	}
	fmt.Println("TLEN:", len(pic.Thumbnail))
	list := [][]any{{pic.Thumbnail}}
	input := &common.Entries{
		Fields: []string{"Thumbnail"},
		//			DataStruct: &store.Pictures{},
		Values: list,
	}
	input.Update = []string{fmt.Sprintf("checksumpicture = '%s'",
		pic.ChecksumPicture)}
	_, n, err := id.Update("Pictures", input)
	if err != nil {
		return err
	}
	fmt.Println("Update n=", n)

	return nil
}

func searchTitle(title string, id common.RegDbID) string {
	q := &common.Query{TableName: "Albums",
		DataStruct: &sql.Albums{},
		Fields:     []string{"Id"},
		Search:     fmt.Sprintf("Title = '%s'", title),
	}
	aid := uint64(0)
	_, err := id.Query(q, func(search *common.Query, result *common.Result) error {
		a := result.Data.(*sql.Albums)
		aid = a.Id
		return nil
	})
	fmt.Println("AID: ", aid)
	if err != nil {
		return ""
	}
	q = &common.Query{TableName: "AlbumPictures",
		DataStruct: &sql.AlbumPictures{},
		Fields:     []string{"AlbumId", "ChecksumPicture"},
		Search:     fmt.Sprintf("albumid = %d AND MIMEType LIKE 'video%%'", aid),
	}
	pictureMDs := make([]string, 0)
	_, err = id.Query(q, func(search *common.Query, result *common.Result) error {
		ap := result.Data.(*sql.AlbumPictures)
		pictureMDs = append(pictureMDs, ap.ChecksumPicture)
		return nil
	})
	if err != nil {
		return ""
	}
	result := "checksumpicture IN ("
	for i, md5 := range pictureMDs {
		if i != 0 {
			result += ","
		}
		result += "'" + md5 + "'"
	}
	result += ")"
	return result
}

func storeThumb(chksum string, pic *store.Pictures) error {

	// c := exec.Command(
	// 	"ffmpeg", "-i", "file.mp4",
	// 	"-vf", "select='eq(pict_type, I)'", "-vsync", "vfr", "%d.jpg",
	// )
	c := exec.Command(
		"ffmpeg", "-ss", "4", "-i", "file.mp4", "-vf", "scale=iw*sar:ih",
		"-frames:v", "1", chksum+"%03d.jpg",
	)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	if err != nil {
		return err
	}
	imgb, err := os.Open(chksum + "001.jpg")
	if err != nil {
		fmt.Println("Chksum error:", err)
		return io.EOF
	}
	img, _ := jpeg.Decode(imgb)
	defer imgb.Close()

	wmb, err := os.Open("watermark.png")
	if err != nil {
		log.Log.Fatalf("Error opening watermark")
	}
	watermark, err := png.Decode(wmb)
	defer wmb.Close()
	if err != nil {
		log.Log.Fatalf("Error decoding watermark")
	}

	offset := image.Pt(1, 1)
	b := img.Bounds()
	m := image.NewRGBA(b)
	draw.Draw(m, b, img, image.ZP, draw.Src)
	draw.Draw(m, watermark.Bounds().Add(offset), watermark, image.ZP, draw.Over)

	var buffer bytes.Buffer
	err = jpeg.Encode(&buffer, m, &jpeg.Options{jpeg.DefaultQuality})
	if err != nil {
		log.Log.Fatalf("Error encoding with watermark")
	}
	pic.Thumbnail = buffer.Bytes()
	return nil
}
