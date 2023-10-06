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
			wgStore.Done()
			if err != nil {
				fmt.Println("Error inserting SQL picture:", err)
			}
		case <-stop:
			return
		}
	}
}

func storeFileInAlbumID(db *sql.DatabaseInfo, file *StoreFile, storeAlbum int) error {
	ti := sql.IncStored()
	baseName := path.Base(file.fileName)
	//dirName := path.Dir(fileName)
	pic, err := LoadFile(db, file.fileName)
	if err != nil {
		return err
	}
	sql.RegisterBlobSize(int64(len(pic.Media)))
	if pic.Available == store.BothAvailable {
		ti.IncDuplicate()
		ti.IncDuplicateLocation()
		return nil
	}
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
		sql.IncError(err)
		return nil, err
	}
	if fi.Size() == 0 {
		return nil, fmt.Errorf("file empty %s", fileName)
	}
	if fi.Size() > MaxBlobSize {
		sql.IncToBig()
		sql.DeferredBlobSize(fi.Size())
		err := fmt.Errorf("file %s tooo big %d > %d", fileName, fi.Size(), MaxBlobSize)
		sql.IncError(err)
		return nil, err
	}
	pic := store.NewPictures(fileName)
	if ShortPath {
		pic.Directory = path.Base(pic.Directory)
	}
	fileType := filepath.Ext(fileName)
	pic.Fill = "1"
	switch strings.ToLower(fileType[1:]) {
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
		sql.IncError(err)
		return nil, err
	}
	pic.ChecksumPicture = createMd5(pic.Media)
	pic.ChecksumPictureSHA = createSHA(pic.Media)

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

func createMd5(input []byte) string {
	return fmt.Sprintf("%X", md5.Sum(input))
}

func createSHA(input []byte) string {
	return fmt.Sprintf("%X", sha256.Sum256(input))
}
