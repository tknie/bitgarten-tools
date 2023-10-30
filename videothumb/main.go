/*
* Copyright © 2023 private, Darmstadt, Germany and/or its licensors
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

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"tux-lobload/sql"
	"tux-lobload/store"

	"github.com/tknie/adabas-go-api/adatypes"
	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

func init() {
	level := zapcore.ErrorLevel
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
		adatypes.Central.SetDebugLevel(true)
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("videothumb.log", level)
	if err != nil {
		fmt.Println("Error initialize logging")
		os.Exit(255)
	}
}

func initLogLevelWithFile(fileName string, level zapcore.Level) (err error) {
	p := os.Getenv("LOGPATH")
	if p == "" {
		p = "."
	}
	name := p + string(os.PathSeparator) + fileName

	rawJSON := []byte(`{
		"level": "error",
		"encoding": "console",
		"outputPaths": [ "loadpicture.log"],
		"errorOutputPaths": ["stderr"],
		"encoderConfig": {
		  "messageKey": "message",
		  "levelKey": "level",
		  "levelEncoder": "lowercase"
		}
	  }`)

	var cfg zap.Config
	if err := json.Unmarshal(rawJSON, &cfg); err != nil {
		fmt.Println("Error initialize logging (json)")
		os.Exit(255)
	}
	cfg.Level.SetLevel(level)
	cfg.OutputPaths = []string{name}
	logger, err := cfg.Build()
	if err != nil {
		fmt.Println("Error initialize logging (build)")
		os.Exit(255)
	}
	defer logger.Sync()

	sugar := logger.Sugar()

	sugar.Infof("Start logging with level %v", level)
	adatypes.Central.Log = sugar
	log.Log = sugar
	log.SetDebugLevel(level == zapcore.DebugLevel)

	return nil
}

func main() {
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
	var chksum string
	var title string
	flag.StringVar(&chksum, "c", "", "Search for picture id checksum")
	flag.StringVar(&title, "a", "", "Search for album title")
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic("could not create CPU profile: " + err.Error())
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			panic("could not start CPU profile: " + err.Error())
		}
		defer pprof.StopCPUProfile()
	}
	defer writeMemProfile(*memprofile)

	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		fmt.Println("Set POSTGRES_URL and/or POSTGRES_PASSWORD to define remote database")
		return
	}
	ref, passwd, err := common.NewReference(url)
	if err != nil {
		fmt.Println("Error parsing URL", err)
		return
	}
	// fmt.Println("Got passwd <", passwd, ">")
	if passwd == "" {
		passwd = os.Getenv("POSTGRES_PASSWORD")
	}
	id, err := flynn.RegisterDatabase(ref, passwd)
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
	if title != "" {
		// prefix = searchTitle(title, id)
		prefix := fmt.Sprintf(SELECT_ALBUM, title)
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
		if chksum != "" {
			cprefix := fmt.Sprintf("title = %s AND ", title)
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
	n, err := id.Update("Pictures", input)
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

	wmb, _ := os.Open("watermark.png")
	watermark, _ := png.Decode(wmb)
	defer wmb.Close()

	offset := image.Pt(1, 1)
	b := img.Bounds()
	m := image.NewRGBA(b)
	draw.Draw(m, b, img, image.ZP, draw.Src)
	draw.Draw(m, watermark.Bounds().Add(offset), watermark, image.ZP, draw.Over)

	var buffer bytes.Buffer
	jpeg.Encode(&buffer, m, &jpeg.Options{jpeg.DefaultQuality})
	pic.Thumbnail = buffer.Bytes()
	return nil
}

func writeMemProfile(file string) {
	if file != "" {
		f, err := os.Create(file)
		if err != nil {
			panic("could not create memory profile: " + err.Error())
		}
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			panic("could not write memory profile: " + err.Error())
		}
		defer f.Close()
		fmt.Println("Memory profile written")
	}

}
