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
	wid, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("Error connect ...:", err)
		return
	}
	gid = id
	q := &common.Query{TableName: "Pictures",
		DataStruct:   &store.Pictures{},
		Fields:       []string{"MIMEType", "checksumpicture", "Media"},
		FctParameter: wid,
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
			log.Log.Errorf("Error video title query: %v", err)
			fmt.Println("Error video title query ...:", err)
			return
		}
	} else {
		prefix := "MIMEType LIKE 'video%'"
		if parameter.ChkSum != "" {
			cprefix := fmt.Sprintf("checksumpicture = '%s' AND ", parameter.ChkSum)
			prefix = cprefix + prefix
		}
		q.Search = prefix
		_, err = id.Query(q, generateQueryVideoThumbnail)
		if err != nil {
			log.Log.Errorf("Error video query: %v", err)
			fmt.Println("Error video query ...:", err)
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

func generateVideoThumbnail(wid common.RegDbID, pic *store.Pictures) error {
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
		log.Log.Errorf("Error preparing storage: %v", err)
		fmt.Printf("Error preparing storage %s: %v\n", pic.ChecksumPicture, err)
		return nil
	}

	if pic.Thumbnail == nil && len(pic.Thumbnail) == 0 {
		log.Log.Fatalf("Thumbnail empty")
	}
	log.Log.Debugf("TLEN: %d", len(pic.Thumbnail))
	list := [][]any{{pic.Thumbnail}}
	input := &common.Entries{
		Fields: []string{"Thumbnail"},
		//			DataStruct: &store.Pictures{},
		Values: list,
	}
	input.Update = []string{fmt.Sprintf("checksumpicture = '%s'",
		pic.ChecksumPicture)}
	_, n, err := wid.Update("Pictures", input)
	if err != nil {
		log.Log.Errorf("Update problem: %v", err)
		return err
	}
	fmt.Println("Update n=", n)
	err = wid.Commit()
	if err != nil {
		return err
	}
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
	log.Log.Infof("Store " + pic.Title)
	logpath := os.Getenv("LOGPATH")
	if logpath == "" {
		logpath = "."
	}
	for _, sec := range []string{"4", "2", "1"} {
		log.Log.Debugf("Thumbnail generated with second " + sec)
		// c := exec.Command(
		// 	"ffmpeg", "-i", "file.mp4",
		// 	"-vf", "select='eq(pict_type, I)'", "-vsync", "vfr", "%d.jpg",
		// )
		c := exec.Command(
			"ffmpeg", "-ss", sec, "-i", "file.mp4", "-vf", "scale=iw*sar:ih",
			"-frames:v", "1", logpath+"/"+chksum+"-%03d.jpg",
		)
		var cBuffer bytes.Buffer
		// c.Stdout = os.Stdout
		// c.Stderr = os.Stderr
		c.Stdout = &cBuffer
		c.Stderr = &cBuffer
		err := c.Run()
		if err != nil {
			fmt.Println(cBuffer.String())
			log.Log.Errorf("Error starting ffmpeg")
			log.Log.Debugf("Output: %s", cBuffer.String())
			continue
		}
		log.Log.Debugf("Thumbnail finally generated with second " + sec)
		imgb, err := os.Open(logpath + "/" + chksum + "-001.jpg")
		if err != nil {
			log.Log.Errorf("Error opening ffmpeg image")
			log.Log.Debugf("Output: %s", cBuffer.String())
			continue
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
		err = os.Remove(logpath + "/" + chksum + "-001.jpg")
		log.Log.Errorf("Remove state: %v", err)
		log.Log.Debugf("Thumbnail generated...")
		return nil
	}
	fmt.Println("Error Thumbnail generated...")
	log.Log.Debugf("Error Thumbnail generated...")
	return fmt.Errorf("no Thumbnail generated")
}
