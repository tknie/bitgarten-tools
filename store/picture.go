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

package store

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"

	"github.com/nfnt/resize"
	"github.com/tknie/log"
)

// PictureBinary definition
type PictureBinary struct {
	FileName    string `xml:"-" json:"-"`
	MetaData    *PictureMetadata
	MaxBlobSize int64 // 50000000
	Data        *PictureData
}

// PictureMetadata definition
type PictureMetadata struct {
	Index           uint64 `adabas:"#isn" json:"-"`
	Md5             string `adabas:"Md5:key"`
	PictureName     string
	PictureHost     string
	Directory       string
	Title           string
	Fill            string
	MIMEType        string
	Option          string
	Width           uint32
	Height          uint32
	ExifModel       string `adabas:"ignore"`
	ExifMake        string
	ExifTaken       string `adabas:"ignore"`
	ExifOrigTime    string
	ExifOrientation byte
	ExifXdimension  uint32
	ExifYdimension  uint32
}

// PictureData definition
type PictureData struct {
	Index             uint64 `adabas:":isn" json:"-"`
	Md5               string `adabas:"Md5:key"`
	ChecksumThumbnail string
	ChecksumPicture   string
	FileName          string `xml:"-" json:"-"`
	Media             []byte `xml:"-" json:"-"`
	Thumbnail         []byte `xml:"-" json:"-"`
}

type Available byte

const (
	NoAvailable Available = iota
	PicAvailable
	PicLocationAvailable
	BothAvailable
)

// Pictures definition
type Pictures struct {
	Index              uint64    `adabas:"#isn"`
	Directory          string    `adabas:"::DN"`
	Md5                string    `adabas:"::M5"`
	ChecksumThumbnail  string    `adabas:"::CT"`
	ChecksumPicture    string    `adabas:"::CP"`
	ChecksumPictureSHA string    `adabas:":ignore"`
	Title              string    `adabas:"::TI"`
	Fill               string    `adabas:"::FI"`
	MIMEType           string    `adabas:"::TY"`
	Option             string    `adabas:"::OP"`
	Width              uint32    `adabas:"::WI"`
	Height             uint32    `adabas:"::HE"`
	Media              []byte    `adabas:"::DP"`
	Thumbnail          []byte    `adabas:"::DT"`
	Generated          time.Time `adabas:"::GE"`
	PictureName        string    `adabas:"::PN"`
	Exif               string    `adabas:":ignore"`
	ExifModel          string    `adabas:":ignore"`
	ExifMake           string    `adabas:":ignore"`
	ExifTaken          time.Time `adabas:":ignore"`
	ExifOrigTime       time.Time `adabas:":ignore"`
	ExifXDimension     int32     `adabas:":ignore"`
	ExifYDimension     int32     `adabas:":ignore"`
	ExifOrientation    string    `adabas:":ignore"`
	GPScoordinates     string
	Available          Available `adabas:":ignore"`
	StoreAlbum         int       `adabas:":ignore"`
	// PictureLocations  []PictureLocations `adabas:"::PL"`
}

type PictureLocations struct {
	PictureName      string `adabas:"::PN"`
	PictureMd5       string `adabas:"::PM"`
	PictureHost      string `adabas:"::PH"`
	PictureDirectory string `adabas:"::PD"`
}

// LoadFile load file
func (pic *PictureBinary) LoadFile() error {
	f, err := os.Open(pic.FileName)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	pic.Data = &PictureData{}
	if fi.Size() > pic.MaxBlobSize {
		return fmt.Errorf("File tooo big %d>%d", fi.Size(), pic.MaxBlobSize)
	}
	pic.Data.Media = make([]byte, fi.Size())
	var n int
	n, err = f.Read(pic.Data.Media)
	log.Log.Debugf("Number of bytes read: %d/%d -> %v\n", n, len(pic.Data.Media), err)
	if err != nil {
		return err
	}
	pic.Data.ChecksumPicture = CreateMd5(pic.Data.Media)
	// pic.MetaData.ChecksumPicture = pic.Data.ChecksumPicture
	log.Log.Debugf("PictureBinary checksum %s size=%d len=%d", pic.Data.ChecksumPicture, fi.Size(), len(pic.Data.Media))

	return nil
}

func CreateMd5(input []byte) string {
	return fmt.Sprintf("%X", md5.Sum(input))
}

func resizePicture(media []byte, max int) ([]byte, uint32, uint32, error) {
	var buffer bytes.Buffer
	buffer.Write(media)
	srcImage, _, err := image.Decode(&buffer)
	if err != nil {
		log.Log.Debugf("Decode image for thumbnail error %v", err)
		return nil, 0, 0, err
	}
	maxX := uint(0)
	maxY := uint(0)
	b := srcImage.Bounds()
	width := uint32(b.Max.X)
	height := uint32(b.Max.Y)
	if width > height {
		maxX = uint(max)
	} else {
		maxY = uint(max)
	}
	//fmt.Println("Original size: ", height, width, "to", max, "window", maxX, maxY)
	//dstImageFill := imaging.Fill(srcImage, 100, 100, imaging.Center, imaging.Lanczos)
	newImage := resize.Resize(maxX, maxY, srcImage, resize.Lanczos3)
	b = newImage.Bounds()
	// width = uint32(b.Max.X)
	// height = uint32(b.Max.Y)
	//fmt.Println("New size: ", height, width)
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, newImage, nil)
	if err != nil {
		// fmt.Println("Error generating thumbnail", err)
		log.Log.Debugf("Encode image for thumbnail error %v", err)
		return nil, 0, 0, err
	}
	return buf.Bytes(), width, height, nil
}

// ExtractExif extract EXIF data
func (pic *PictureBinary) ExtractExif() error {
	buffer := bytes.NewBuffer(pic.Data.Media)
	x, err := exif.Decode(buffer)
	if err != nil {
		// fmt.Println("Exif error: ", buffer.Len(), err)
		return err
	}
	// fmt.Println(x)
	// var p Printer
	// x.Walk(p)
	camModel, err := x.Get(exif.Model) // normally, don't ignore errors!
	if err != nil {
		fmt.Println(err)
	} else {
		model, _ := camModel.StringVal()
		pic.MetaData.ExifModel = model
	}

	m, merr := x.Get(exif.Make)
	if merr == nil {
		ms, _ := m.StringVal()
		pic.MetaData.ExifMake = ms
	}

	// Two convenience functions exist for date/time taken and GPS coords:
	tm, tmerr := x.DateTime()
	if tmerr == nil {
		pic.MetaData.ExifTaken = tm.String()
	}

	tmo, tmoerr := x.Get(exif.DateTimeOriginal)
	if tmoerr == nil {
		pic.MetaData.ExifOrigTime = tmo.String()
	}

	o, oerr := x.Get(exif.Orientation)
	if oerr == nil {
		v, _ := o.Int(0)
		pic.MetaData.ExifOrientation = byte(v)
	}

	xd, xderr := x.Get(exif.PixelXDimension)
	if xderr == nil {
		v, _ := xd.Int(0)
		pic.MetaData.ExifXdimension = uint32(v)
	}
	yd, yderr := x.Get(exif.PixelYDimension)
	if yderr == nil {
		v, _ := yd.Int(0)
		pic.MetaData.ExifYdimension = uint32(v)
	}
	return nil
}

// CreateThumbnail create thumbnail
func (pic *PictureBinary) CreateThumbnail() error {
	if strings.HasPrefix(pic.MetaData.MIMEType, "image") {
		// thmb, w, h, err := resizePicture(pic.Data.Media, 1280)
		// if err != nil {
		// 	fmt.Println("Error generating thumbnail", err)
		// 	return err
		// }
		// pic.Data.Media = thmb
		// pic.MetaData.Width = w
		// pic.MetaData.Height = h
		thmb, w, h, err := resizePicture(pic.Data.Media, 200)
		if err != nil {
			fmt.Println("Error generating thumbnail", pic.MetaData.MIMEType, err)
			return err
		}
		pic.Data.Thumbnail = thmb
		pic.MetaData.Width = w
		pic.MetaData.Height = h
		pic.Data.ChecksumThumbnail = CreateMd5(pic.Data.Thumbnail)
		log.Log.Debugf("Thumbnail checksum %s", pic.Data.ChecksumThumbnail)
	} else {
		fmt.Println("No image, skip thumbnail generation ....")
	}
	return nil

}

func NewPictures(fileName string) *Pictures {
	return &Pictures{Directory: filepath.Dir(fileName), PictureName: filepath.Base(fileName)}
}

// CreateThumbnail create thumbnail
func (pic *Pictures) CreateThumbnail() error {
	if strings.HasPrefix(pic.MIMEType, "image") {
		thmb, w, h, err := resizePicture(pic.Media, 200)
		if err != nil {
			fmt.Println("Error generating thumbnail", pic.PictureName, ":", err)
			return err
		}
		pic.Thumbnail = thmb
		pic.Width = w
		pic.Height = h
		pic.ChecksumThumbnail = CreateMd5(pic.Thumbnail)
		pic.Md5 = pic.ChecksumThumbnail
		log.Log.Debugf("Thumbnail checksum %s", pic.ChecksumThumbnail)

		err = pic.ExifReader()
		if err != nil && err != io.EOF {
			return err
		}

	}
	return nil
}
