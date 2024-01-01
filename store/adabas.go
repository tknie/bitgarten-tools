package store

import (
	"fmt"
	"os"
	"strings"

	"github.com/tknie/adabas-go-api/adabas"
	"github.com/tknie/adabas-go-api/adatypes"
)

// PictureConnection picture connection handle
type PictureConnection struct {
	conn        *adabas.Connection
	store       *adabas.StoreRequest
	storeData   *adabas.StoreRequest
	storeThumb  *adabas.StoreRequest
	readCheck   *adabas.ReadRequest
	histCheck   *adabas.ReadRequest
	ShortenName bool
	ChecksumRun bool
	Found       uint64
	Empty       uint64
	Loaded      uint64
	Checked     uint64
	ToBig       uint64
	Errors      map[string]uint64
	Filter      []string
	NrErrors    uint64
	NrDeleted   uint64
	Ignored     uint64
	MaxBlobSize int64
}

// Hostname of this host
var Hostname = "Unknown"

func init() {
	host, err := os.Hostname()
	if err == nil {
		Hostname = host
	}
}

// InitStorePictureBinary init store picture connection
func InitStorePictureBinary(shortenName bool) (ps *PictureConnection, err error) {
	ps = &PictureConnection{ShortenName: shortenName, ChecksumRun: false,
		Errors: make(map[string]uint64)}
	ps.conn, err = adabas.NewConnection("acj;map")
	if err != nil {
		return nil, err
	}
	ps.store, err = ps.conn.CreateMapStoreRequest((*PictureMetadata)(nil))
	if err != nil {
		ps.conn.Close()
		return nil, err
	}
	err = ps.store.StoreFields("*")
	if err != nil {
		return nil, err
	}
	ps.storeData, err = ps.conn.CreateMapStoreRequest((*PictureData)(nil))
	if err != nil {
		ps.conn.Close()
		return nil, err
	}
	err = ps.storeData.StoreFields("Md5,Media")
	if err != nil {
		return nil, err
	}
	ps.storeThumb, err = ps.conn.CreateMapStoreRequest((*PictureData)(nil))
	if err != nil {
		ps.conn.Close()
		return nil, err
	}
	err = ps.storeThumb.StoreFields("Md5,ChecksumPicture,ChecksumThumbnail,Thumbnail")
	if err != nil {
		return nil, err
	}
	ps.readCheck, err = ps.conn.CreateMapReadRequest("PictureMetadata")
	if err != nil {
		ps.conn.Close()
		return nil, err
	}
	err = ps.readCheck.QueryFields("Md5")
	if err != nil {
		ps.conn.Close()
		return nil, err
	}
	ps.histCheck, err = ps.conn.CreateMapReadRequest("Picture")
	if err != nil {
		ps.conn.Close()
		return nil, err
	}
	return
}

type verify struct {
	mapName string
	ref     string
}

func verifyPictureRecord(record *adabas.Record, x interface{}) error {
	f, ferr := record.SearchValue("PictureName")
	if ferr != nil {
		return ferr
	}
	d, derr := record.SearchValue("Directory")
	if derr != nil {
		return derr
	}
	fileName := f.String()
	directory := strings.Trim(d.String(), " ")
	v, xerr := record.SearchValue("Media")
	if xerr != nil {
		return xerr
	}
	vLen := len(v.Bytes())
	md := CreateMd5(v.Bytes())
	v, xerr = record.SearchValue("ChecksumPicture")
	if xerr != nil {
		return xerr
	}
	smd := strings.Trim(v.String(), " ")
	if md != smd || directory == "" {
		fmt.Printf("ISN=%d. name=%s directory=%s len=%d\n", record.Isn, fileName, directory, vLen)
		if md != smd {
			fmt.Printf("MD5 data=<%s> expected=<%s>\n", md, smd)
			fmt.Println("Record checksum error", record.Isn)
		}
		if directory == "" {
			fmt.Println("Record directory empty", record.Isn)
			DeletePicture(x.(*verify), record.Isn)
		}
		return nil //fmt.Errorf("record checksum error")
	}
	return nil
}

// VerifyPicture verify pictures
func VerifyPicture(mapName, ref string) error {
	connection, cerr := adabas.NewConnection("acj;map;config=[" + ref + "]")
	if cerr != nil {
		return cerr
	}
	defer connection.Close()
	request, rerr := connection.CreateMapReadRequest(mapName)
	if rerr != nil {
		fmt.Println("Error create request", rerr)
		return rerr
	}
	err := request.QueryFields("Media,ChecksumPicture,Directory,PictureName")
	if err != nil {
		return err
	}
	request.Limit = 0
	request.Multifetch = 1
	result, rErr := request.ReadPhysicalSequenceStream(verifyPictureRecord, &verify{mapName, ref})
	if rErr != nil {
		return rErr
	}
	fmt.Println(result)
	return nil
}

// VerifyPicture verify pictures
func DeletePicture(v *verify, isn adatypes.Isn) error {
	connection, cerr := adabas.NewConnection("acj;map;config=[" + v.ref + "]")
	if cerr != nil {
		return cerr
	}
	defer connection.Close()
	delete, derr := connection.CreateMapDeleteRequest(v.mapName)
	if derr != nil {
		fmt.Println("Error create delete request", derr)
		return derr
	}
	derr = delete.Delete(isn)
	if derr != nil {
		return derr
	}

	return delete.EndTransaction()
}
