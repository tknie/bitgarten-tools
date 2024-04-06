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

package main

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"tux-lobload/sql"
	"tux-lobload/store"

	"github.com/docker/go-units"
	"github.com/tknie/log"
)

var MaxBlobSize = int64(30000000)

var globalindex = uint64(0)
var ShortPath = false
var wgStore sync.WaitGroup

type Data struct {
	Media                 []byte
	Thumbnail             []byte
	ChecksumPicture       string
	ChecksumPictureSha256 string
	ChecksumThumbnail     string
}

type StoreFile struct {
	fileName string
	albumid  int
}

var storeChannel = make(chan *StoreFile, 4)
var stop = make(chan bool)

func queueStoreFileInAlbumID(fileName string, albumid int) {
	wgStore.Add(1)
	storeChannel <- &StoreFile{fileName: fileName, albumid: albumid}
}

func StoreWorker() {
	checker, err := sql.CreateConnection()
	if err != nil {
		log.Log.Fatalf("Database connection not established: %v", err)
	}
	for {
		select {
		case file := <-storeChannel:
			err := storeFileInAlbumID(checker, file, albumid)
			if err != nil {
				if !strings.HasPrefix(err.Error(), "file empty") {
					fmt.Println("Error inserting SQL picture:", err)
				}
			}
			wgStore.Done()
		case <-stop:
			return
		}
	}
}

func storeFileInAlbumID(db *sql.DatabaseInfo, file *StoreFile, storeAlbum int) error {
	log.Log.Debugf("Store file %s", file.fileName)
	ti := sql.IncStored()
	baseName := path.Base(file.fileName)
	//dirName := path.Dir(fileName)
	pic, err := LoadFile(db, file.fileName)
	if err != nil {
		log.Log.Errorf("Store file %s load failed: %v", file.fileName, err)
		return err
	}
	sql.RegisterBlobSize(int64(len(pic.Media)))
	log.Log.Debugf("Available = %d", pic.Available)
	if pic.Available == store.BothAvailable {
		ti.IncDuplicate()
		ti.IncDuplicateLocation()
		log.Log.Debugf("Duplicate found")
		return nil
	}
	log.Log.Debugf("Store file %s", file.fileName)
	ti.IncLoaded()
	globalindex++
	pic.Index = globalindex
	pic.Title = baseName
	pic.StoreAlbum = storeAlbum
	sql.StorePictures(pic)
	ti.IncEndStored()
	return nil
}

// LoadFile load file
func LoadFile(db *sql.DatabaseInfo, fileName string) (*store.Pictures, error) {
	f, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Open file error:", err)
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		sql.IncError("Stat Error "+fileName, err)
		return nil, err
	}
	if fi.Size() == 0 {
		return nil, fmt.Errorf("file empty %s", fileName)
	}
	if fi.Size() > MaxBlobSize {
		sql.IncToBig()
		sql.DeferredBlobSize(fi.Size())
		fileSize := units.HumanSize(float64(fi.Size()))
		maxSize := units.HumanSize(float64(MaxBlobSize))
		err := fmt.Errorf("file %s tooo big %s > %s", fileName, fileSize, maxSize)
		sql.IncError("File too big "+fileName, err)
		return nil, err
	}
	pic := store.NewPictures(fileName)
	if ShortPath {
		pic.Directory = path.Base(pic.Directory)
	}
	fileType := filepath.Ext(fileName)
	pic.Fill = "1"
	switch strings.ToLower(fileType[1:]) {
	case "heic", "heif":
		pic.MIMEType = "image/" + fileType[1:]
	case "jpeg", "jpg", "gif":
		pic.MIMEType = "image/" + fileType[1:]
	case "mp4", "mov", "m4v", "webm":
		pic.MIMEType = "video/" + fileType[1:]
	default:
		fmt.Println("Unknown format found:", fileType[1:])
		return nil, fmt.Errorf("no format to upload of type " + fileType[1:])
	}

	pic.Media = make([]byte, fi.Size())
	var n int
	n, err = f.Read(pic.Media)
	log.Log.Debugf("Number of bytes read: %d/%d -> %v\n", n, len(pic.Media), err)
	if err != nil {
		sql.IncError("Read error "+fileName, err)
		return nil, err
	}
	pic.ChecksumPicture = CreateMd5(pic.Media)
	pic.ChecksumPictureSHA = CreateSHA(pic.Media)

	db.CheckExists(pic)
	if pic.Available == store.BothAvailable {
		return pic, nil
	}

	err = pic.CreateThumbnail()
	if err != nil {
		log.Log.Errorf("Error creating thumbnail %s: %v", fileName, err)
		sql.IncErrorFile(err, pic.Directory+"/"+pic.PictureName)
	}

	log.Log.Debugf("PictureBinary md5=%s sha512=%s size=%d len=%d", pic.ChecksumPicture, pic.ChecksumPictureSHA, fi.Size(), len(pic.Media))

	return pic, nil
}

func CreateMd5(input []byte) string {
	return fmt.Sprintf("%X", md5.Sum(input))
}

func CreateSHA(input []byte) string {
	return fmt.Sprintf("%X", sha256.Sum256(input))
}
