package store

import (
	"fmt"
	"os"
	"regexp"
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

// LoadPicture load picture data into database
func (ps *PictureConnection) LoadPicture(insert bool, fileName string, ada *adabas.Adabas) error {
	fs := strings.Split(fileName, string(os.PathSeparator))
	pictureName := fileName
	if !ps.ShortenName {
		if fs[len(fs)-2] == "img" {
			pictureName = fs[len(fs)-3] + "/" + fs[len(fs)-1]
		} else {
			pictureName = fs[len(fs)-2] + "/" + fs[len(fs)-1]
		}
		fmt.Printf("Shorten name from %s to %s\n", fileName, pictureName)
	}
	pictureKey := createMd5([]byte(pictureName))
	var err error
	var ok bool
	ok, err = ps.available(pictureKey)
	if err != nil {
		adatypes.Central.Log.Debugf("Availability check error %v", err)
		return err
	}
	empty := checkEmpty(fileName)
	if empty {
		adatypes.Central.Log.Debugf(pictureName, "-> picture file empty")
		ps.Empty++
		if ok {
			fmt.Printf("Remove empty file from database: %s(%s)\n", fileName, pictureKey)
			ps.DeleteMd5(ada, pictureKey)
		}
		return nil
	}
	ps.Checked++
	if ok && insert {
		adatypes.Central.Log.Debugf(pictureName, "-> picture name already loaded")
		ps.Found++
		return nil
	}
	info := "Loading"
	if !insert {
		info = "Updating"
	}
	fmt.Printf("%s picture ... %s\n", info, fileName)
	// fmt.Println("-> load picture name ...", pictureName, "Md5=", pictureKey)
	var re = regexp.MustCompile(`(?m)([^/]*)/.*`)
	d := re.FindStringSubmatch(pictureName)[1]
	// fmt.Println("Directory: ", d)
	p := PictureBinary{FileName: fileName,
		MetaData: &PictureMetadata{PictureName: pictureName, Directory: d,
			PictureHost: Hostname, Md5: pictureKey}, MaxBlobSize: ps.MaxBlobSize}
	err = p.LoadFile()
	if err != nil {
		adatypes.Central.Log.Debugf("Load file error %v", err)
		return err
	}

	suffix := fileName[strings.LastIndex(fileName, ".")+1:]
	suffix = strings.ToLower(suffix)
	switch suffix {
	case "jpg", "jpeg", "gif":
		p.MetaData.MIMEType = "image/" + suffix
		p.ExtractExif()
		terr := p.CreateThumbnail()
		if terr != nil {
			adatypes.Central.Log.Debugf("Create thumbnail error %v", terr)
			return terr
		}
		if p.MetaData.Height > p.MetaData.Width {
			p.MetaData.Fill = "1"
		} else {
			p.MetaData.Fill = "2"
		}
	case "m4v", "mov", "mp4":
		p.MetaData.MIMEType = "video/mp4"
		p.MetaData.Fill = "0"
	case "webm":
		p.MetaData.MIMEType = "video/webm"
		p.MetaData.Fill = "0"
	default:
		panic("Unknown suffix " + suffix)
	}
	adatypes.Central.Log.Debugf("Done set value to Picture, searching ...")

	if insert {
		//fmt.Println("Store record metadata ....", p.MetaData.Md5)
		err = ps.store.StoreData(p.MetaData)
	} else {
		// fmt.Println("Update record ....", p.MetaData.Md5, "with ISN", p.MetaData.Index)
		err = ps.store.UpdateData(p.MetaData)
	}
	// fmt.Println("Stored metadata into ISN=", p.MetaData.Index)
	if err != nil {
		fmt.Printf("Error storing record metadata: %v %#v", err, p.MetaData)
		return err
	}
	p.Data.Md5 = p.MetaData.Md5
	p.Data.Index = p.MetaData.Index
	if !ps.ChecksumRun {
		//ok, err = ps.checkPicture(pictureKey)
		if ok || !insert {
			// fmt.Println("Store data storage")
			fmt.Println("Update record data ....", p.Data.Md5, " of size ", len(p.Data.Media))
			err = ps.storeData.UpdateData(p.Data, true)
			if err != nil {
				fmt.Println("Error storing record data:", err)
				return err
			}
			err = ps.conn.EndTransaction()
			if err != nil {
				panic("Data write: end of transaction error: " + err.Error())
			}
		}
	}
	// fmt.Println("Update record thumbnail ....", p.Data.Md5)
	err = ps.storeThumb.UpdateData(p.Data)
	if err != nil {
		fmt.Printf("Store request error %v\n", err)
		return err
	}
	adatypes.Central.Log.Debugf("Updated record into ISN=%d MD5=%s", p.MetaData.Index, p.Data.Md5)
	err = ps.store.EndTransaction()
	if err != nil {
		panic("End of transaction error: " + err.Error())
	}
	ps.Loaded++
	return nil
}

func checkEmpty(fileName string) bool {
	st, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		// file is not exists similar to empty
		return true
	}
	if st.Size() == 0 {
		return true
	}
	return false
}

func (ps *PictureConnection) available(key string) (bool, error) {
	//fmt.Println("Check Md5=" + key)
	result, err := ps.readCheck.HistogramWith("Md5=" + key)
	if err != nil {
		fmt.Printf("Error checking Md5=%s: %v\n", key, err)
		panic("Read error " + err.Error())
		//		return false, err
	}
	// result.DumpValues()
	if len(result.Values) > 0 || len(result.Data) > 0 {
		adatypes.Central.Log.Debugf("Md5=%s is available\n", key)
		return true, nil
	}
	adatypes.Central.Log.Debugf("Md5=%s is not loaded\n", key)
	return false, nil
}

func (ps *PictureConnection) checkPicture(key string) (bool, error) {
	//fmt.Println("Check Md5=" + key)
	result, err := ps.histCheck.HistogramWith("ChecksumPicture=" + key)
	if err != nil {
		fmt.Printf("Error checking ChecksumPicture=%s: %v\n", key, err)
		panic("Read error " + err.Error())
		//		return false, err
	}
	// result.DumpValues()
	if len(result.Values) > 0 || len(result.Data) > 0 {
		adatypes.Central.Log.Debugf("ChecksumPicture=%s is available\n", key)
		return true, nil
	}
	adatypes.Central.Log.Debugf("ChecksumPicture=%s is not loaded\n", key)
	return false, nil
}

// Close connection
func (ps *PictureConnection) Close() {
	if ps != nil && ps.conn != nil {
		ps.conn.Close()
	}
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
	md := createMd5(v.Bytes())
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
