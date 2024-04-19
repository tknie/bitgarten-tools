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

package sql

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"sort"
	"strings"
	"time"
	"tux-lobload/store"

	"github.com/nfnt/resize"
	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

// Picture picture data
type Picture struct {
	Id              uint64
	ChecksumPicture string
	Sha256checksum  string
	Thumbnail       []byte
	Media           []byte
	Title           string
	Fill            string
	Mimetype        string
	Height          uint64
	Width           uint64
	Exifmodel       string
	Exifmake        string
	Exiftaken       string
	Exiforigtime    string
	Exifxdimension  uint64
	Exifydimension  uint64
	Exiforientation string
	Created         time.Time
	Updated_at      time.Time
}

// AlbumPictures pciture information
type AlbumPictures struct {
	Index           uint64
	AlbumId         uint64
	Name            string
	Description     string
	ChecksumPicture string
	MimeType        string
	SkipTime        uint64
	Height          uint64
	Width           uint64
}

// Album album information
type Albums struct {
	Id            uint64
	Type          string
	Key           string
	Directory     string
	Title         string
	Description   string
	Option        string
	ThumbnailHash string
	Published     time.Time
	// Created       time.Time
	//	Updated_At    time.Time
	Pictures []*AlbumPictures `db:":ignore" flynn:":ignore"`
}

func Connect(url, pwd string) (*DatabaseInfo, error) {
	ref, passwd, err := common.NewReference(url)
	if err != nil {
		return nil, err
	}
	if passwd == "" {
		passwd = pwd
	}
	log.Log.Infof("Connecting to .... %s", ref.Host)
	return &DatabaseInfo{0, ref, passwd, 0, 0}, nil
}

func (di *DatabaseInfo) Open() (common.RegDbID, error) {
	id, err := flynn.Handler(di.Reference, di.passwd)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (di *DatabaseInfo) ListAlbums() error {
	id, err := di.Open()
	if err != nil {
		return err
	}
	fmt.Println("List Album titles:")
	q := &common.Query{TableName: "Albums",
		DataStruct: Albums{},
		Fields:     []string{"Title", "Id", "Published"},
	}
	count := 0
	_, err = id.Query(q, func(search *common.Query, result *common.Result) error {
		album := result.Data.(*Albums)
		count++
		fmt.Printf("%03d - %3d: %-35s %v\n", count, album.Id, album.Title, album.Published)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (di *DatabaseInfo) GetAlbums() ([]*Albums, error) {
	id, err := di.Open()
	if err != nil {
		return nil, err
	}
	fmt.Println("List Album titles:")
	albums := make([]*Albums, 0)
	q := &common.Query{TableName: "Albums",
		DataStruct: Albums{},
		Fields:     []string{"Title", "Id", "Published"},
	}
	_, err = id.Query(q, func(search *common.Query, result *common.Result) error {
		album := result.Data.(*Albums)
		copyAlbum := &Albums{}
		*copyAlbum = *album
		albums = append(albums, copyAlbum)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return albums, nil
}

func (di *DatabaseInfo) ReadAlbum(albumTitle string) (*Albums, error) {
	var album *Albums
	id, err := di.Open()
	if err != nil {
		return nil, err
	}
	fmt.Println("Search Album title:", strings.Replace(albumTitle, "'", "\\'", -1), albumTitle)
	q := &common.Query{TableName: "Albums",
		DataStruct: Albums{},
		Fields:     []string{"*"},
		Search:     fmt.Sprintf("Title = '%s'", strings.Replace(albumTitle, "'", "''", -1)),
	}
	_, err = id.Query(q, func(search *common.Query, result *common.Result) error {
		album = result.Data.(*Albums)
		return nil
	})
	if err != nil {
		return nil, err
	}
	album.Pictures, err = di.ReadAlbumPictures(id, int(album.Id))
	if err != nil {
		return nil, err
	}
	return album, nil
}

func (di *DatabaseInfo) ReadAlbumPictures(id common.RegDbID, albumid int) ([]*AlbumPictures, error) {
	q := &common.Query{TableName: "AlbumPictures",
		DataStruct: AlbumPictures{},
		Fields:     []string{"*"},
		Search:     fmt.Sprintf("albumid = %d", albumid),
	}
	pictures := make([]*AlbumPictures, 0)
	_, err := id.Query(q, func(search *common.Query, result *common.Result) error {
		ap := result.Data.(*AlbumPictures)
		apic := &AlbumPictures{}
		*apic = *ap
		pictures = append(pictures, apic)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(pictures, func(i, j int) bool {
		return pictures[i].Index < pictures[j].Index
	})
	return pictures, nil
}

func (di *DatabaseInfo) CheckPicture(checksum string) (bool, error) {
	id, err := di.Open()
	if err != nil {
		return false, err
	}
	defer id.FreeHandler()
	q := &common.Query{TableName: "Pictures",
		DataStruct: Picture{},
		Fields:     []string{"checksumpicture"},
		Search:     fmt.Sprintf("checksumpicture = '%s'", checksum),
	}
	found := false
	_, err = id.Query(q, func(search *common.Query, result *common.Result) error {
		found = true
		return nil
	})
	if err != nil {
		return false, err
	}

	return found, nil
}

func (di *DatabaseInfo) ReadPicture(checksum string) (*Picture, error) {
	id, err := di.Open()
	if err != nil {
		return nil, err
	}
	defer id.FreeHandler()
	q := &common.Query{TableName: "Pictures",
		DataStruct: Picture{},
		Fields:     []string{"*"},
		Search:     fmt.Sprintf("checksumpicture = '%s'", checksum),
	}
	found := false
	res, err := id.Query(q, func(search *common.Query, result *common.Result) error {
		found = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("picture not found")
	}

	return res.Data.(*Picture), nil
}

func (a *Albums) Display() {
	fmt.Printf("Found album %d: %s\n", a.Id, a.Title)
	for _, p := range a.Pictures {
		fmt.Printf("   Found picture %d: %s\n", p.Index, p.Description)
	}
}

func (di *DatabaseInfo) CheckAlbum(album *Albums) (uint64, error) {
	id, err := di.Open()
	if err != nil {
		return 0, err
	}
	defer id.FreeHandler()
	q := &common.Query{TableName: "Albums",
		DataStruct: Albums{},
		Fields:     []string{"id", "Title"},
		Search:     fmt.Sprintf("title = '%s'", strings.Replace(album.Title, "'", "''", -1)),
	}
	found := uint64(0)
	_, err = id.Query(q, func(search *common.Query, result *common.Result) error {
		a := result.Data.(*Albums)
		found = a.Id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return found, nil
}

func (di *DatabaseInfo) CheckAlbumPictures(albumPic *AlbumPictures) (bool, error) {
	id, err := di.Open()
	if err != nil {
		return false, err
	}
	defer id.FreeHandler()
	q := &common.Query{TableName: "AlbumPictures",
		DataStruct: AlbumPictures{},
		Fields:     []string{"index", "albumid"},
		Search: fmt.Sprintf("index = %d AND albumid = %d",
			albumPic.Index, albumPic.AlbumId),
	}
	found := false
	_, err = id.Query(q, func(search *common.Query, result *common.Result) error {
		found = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

func (di *DatabaseInfo) CheckMedia(f common.ResultFunction) error {

	id, err := di.Open()
	if err != nil {
		return err
	}
	defer id.FreeHandler()
	q := &common.Query{TableName: "Pictures",
		DataStruct: &Picture{},
		Fields:     []string{"ChecksumPicture", "Sha256checksum", "Media"},
	}

	_, err = id.Query(q, f)
	return err
}

func (pic *Picture) Resize(max int) error {
	var buffer bytes.Buffer
	buffer.Write(pic.Media)
	srcImage, _, err := image.Decode(&buffer)
	if err != nil {
		log.Log.Debugf("Decode image for thumbnail error %v", err)
		return err
	}
	m, x, y, err := resizeImage(srcImage, max)
	if err != nil {
		return err
	}
	pic.Media = m
	pic.Width = uint64(x)
	pic.Height = uint64(y)
	pic.ChecksumPicture = store.CreateMd5(pic.Media)
	pic.Sha256checksum = store.CreateSHA(pic.Media)
	return nil
}

func resizeImage(srcImage image.Image, max int) ([]byte, uint32, uint32, error) {
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
	// fmt.Println("Original size: ", height, width, "to", max, "window", maxX, maxY)
	//dstImageFill := imaging.Fill(srcImage, 100, 100, imaging.Center, imaging.Lanczos)
	newImage := resize.Resize(maxX, maxY, srcImage, resize.Lanczos3)
	b = newImage.Bounds()
	width = uint32(b.Max.X)
	height = uint32(b.Max.Y)
	// fmt.Println("New size: ", height, width)
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, newImage, nil)
	if err != nil {
		// fmt.Println("Error generating thumbnail", err)
		log.Log.Debugf("Encode image for thumbnail error %v", err)
		return nil, 0, 0, err
	}
	return buf.Bytes(), width, height, nil
}
