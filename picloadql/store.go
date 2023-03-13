package main

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"tux-lobload/sql"
	"tux-lobload/store"

	"github.com/tknie/adabas-go-api/adatypes"
)

var MaxBlobSize = int64(30000000)

var globalindex = uint64(0)
var ShortPath = false

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
	storeChannel <- &StoreFile{fileName: fileName, albumid: albumid}
}

func StoreWorker() {
	for {
		select {
		case file := <-storeChannel:
			err := storeFileInAlbumID(file.fileName, file.albumid)
			if err != nil {
				fmt.Println("Error inserting SQL picture:", err)
			}
		case <-stop:
			return
		}
	}
}

func storeFileInAlbumID(fileName string, albumid int) error {
	ti := sql.IncStored()
	baseName := path.Base(fileName)
	//dirName := path.Dir(fileName)
	pic, err := LoadFile(fileName)
	if err != nil {
		return err
	}
	sql.RegisterBlobSize(int64(len(pic.Media)))
	ti.IncLoaded()
	globalindex++
	pic.Index = globalindex
	sql.StorePictures(pic)
	pic.Title = baseName
	ti.IncInserted()
	sql.StorePictures(pic)
	ti.IncEndStored()
	return nil
}

func storeFile(con *sql.DatabaseInfo, fileName string, albumid int) error {
	ti := sql.IncStored()
	baseName := path.Base(fileName)
	//dirName := path.Dir(fileName)
	pic, err := LoadFile(fileName)
	if err != nil {
		return err
	}
	sql.RegisterBlobSize(int64(len(pic.Media)))
	ti.IncLoaded()
	globalindex++
	pic.Index = globalindex
	//pic.Directory = path.Base(dirName)
	//pic.PictureName = baseName
	pic.Title = baseName
	globalindex++
	err = con.InsertAlbumPictures(pic, int(globalindex), albumid)
	if err != nil {
		return err
	}
	ti.IncInserted()
	sql.StorePictures(pic)
	ti.IncEndStored()
	return nil
}

// LoadFile load file
func LoadFile(fileName string) (*store.Pictures, error) {
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
	pic := &store.Pictures{Directory: filepath.Dir(fileName), PictureName: filepath.Base(fileName)}
	if ShortPath {
		pic.Directory = path.Base(pic.Directory)
	}
	pic.Media = make([]byte, fi.Size())
	var n int
	n, err = f.Read(pic.Media)
	adatypes.Central.Log.Debugf("Number of bytes read: %d/%d -> %v\n", n, len(pic.Media), err)
	if err != nil {
		sql.IncError(err)
		return nil, err
	}
	pic.ChecksumPicture = createMd5(pic.Media)
	pic.ChecksumPictureSHA = createSHA(pic.Media)

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

	adatypes.Central.Log.Debugf("Check image %s", pic.MIMEType)
	if strings.HasPrefix(pic.MIMEType, "image/") {
		err = pic.CreateThumbnail()
		if err != nil {
			adatypes.Central.Log.Errorf("Error creating thumbnail: %s", fileName)
			sql.IncErrorFile(err, pic.Directory+"/"+pic.PictureName)
			// return nil, err
		} else {
			pic.Md5 = pic.ChecksumThumbnail
		}
		err = pic.ExifReader()
		if err != nil && err != io.EOF {
			adatypes.Central.Log.Errorf("Error Exif reader: %s -> %v", fileName, err)
			sql.IncErrorFile(err, pic.Directory+"/"+pic.PictureName)
		}
	}
	// pic.MetaData.ChecksumPicture = pic.Data.ChecksumPicture
	adatypes.Central.Log.Debugf("PictureBinary md5=%s sha512=%s size=%d len=%d", pic.ChecksumPicture, pic.ChecksumPictureSHA, fi.Size(), len(pic.Media))

	return pic, nil
}

func createMd5(input []byte) string {
	return fmt.Sprintf("%X", md5.Sum(input))
}

func createSHA(input []byte) string {
	return fmt.Sprintf("%X", sha256.Sum256(input))
}
