package store

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"

	"github.com/nfnt/resize"
	"github.com/tknie/adabas-go-api/adabas"
	"github.com/tknie/adabas-go-api/adatypes"
	"golang.org/x/net/html"
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
	ExifModel       string
	ExifMake        string
	ExifTaken       string
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
	Index              uint64 `adabas:"#isn"`
	Directory          string `adabas:"::DN"`
	Md5                string `adabas:"::M5"`
	ChecksumThumbnail  string `adabas:"::CT"`
	ChecksumPicture    string `adabas:"::CP"`
	ChecksumPictureSHA string
	Title              string `adabas:"::TI"`
	Fill               string `adabas:"::FI"`
	MIMEType           string `adabas:"::TY"`
	Option             string `adabas:"::OP"`
	Width              uint32 `adabas:"::WI"`
	Height             uint32 `adabas:"::HE"`
	Media              []byte `adabas:"::DP"`
	Thumbnail          []byte `adabas:"::DT"`
	Generated          int64  `adabas:"::GE"`
	PictureName        string `adabas:"::PN"`
	Exif               string
	ExifModel          string
	ExifMake           string
	ExifTaken          time.Time
	ExifOrigTime       time.Time
	ExifXDimension     int32
	ExifYDimension     int32
	ExifOrientation    string
	Available          Available
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
	adatypes.Central.Log.Debugf("Number of bytes read: %d/%d -> %v\n", n, len(pic.Data.Media), err)
	if err != nil {
		return err
	}
	pic.Data.ChecksumPicture = createMd5(pic.Data.Media)
	// pic.MetaData.ChecksumPicture = pic.Data.ChecksumPicture
	adatypes.Central.Log.Debugf("PictureBinary checksum %s size=%d len=%d", pic.Data.ChecksumPicture, fi.Size(), len(pic.Data.Media))

	return nil
}

func createMd5(input []byte) string {
	return fmt.Sprintf("%X", md5.Sum(input))
}

func resizePicture(media []byte, max int) ([]byte, uint32, uint32, error) {
	var buffer bytes.Buffer
	buffer.Write(media)
	srcImage, _, err := image.Decode(&buffer)
	if err != nil {
		adatypes.Central.Log.Debugf("Decode image for thumbnail error %v", err)
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
	width = uint32(b.Max.X)
	height = uint32(b.Max.Y)
	//fmt.Println("New size: ", height, width)
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, newImage, nil)
	if err != nil {
		// fmt.Println("Error generating thumbnail", err)
		adatypes.Central.Log.Debugf("Encode image for thumbnail error %v", err)
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
		pic.Data.ChecksumThumbnail = createMd5(pic.Data.Thumbnail)
		adatypes.Central.Log.Debugf("Thumbnail checksum %s", pic.Data.ChecksumThumbnail)
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
		pic.ChecksumThumbnail = createMd5(pic.Thumbnail)
		pic.Md5 = pic.ChecksumThumbnail
		adatypes.Central.Log.Debugf("Thumbnail checksum %s", pic.ChecksumThumbnail)

		err = pic.ExifReader()
		if err != nil && err != io.EOF {
			return err
		}

	}
	return nil
}

// ReadDatabase read picture binary from database
func (pic *PictureBinary) ReadDatabase(hash, repository string) (err error) {
	connection, err := adabas.NewConnection("acj;map;config=[" + repository + "]")
	if err != nil {
		return
	}
	defer connection.Close()

	request, rerr := connection.CreateMapReadRequest(PictureBinary{})
	if rerr != nil {
		fmt.Println("Error create request", rerr)
		err = rerr
		return
	}
	err = request.QueryFields("Data")
	if err != nil {
		return
	}
	result, resErr := request.ReadLogicalWith("Md5=" + hash)
	if resErr != nil {
		fmt.Println("Error reading ISN order", resErr)
		err = resErr
		return
	}
	if len(result.Data) == 0 {
		return fmt.Errorf("No data found")
	}
	resultPic := result.Data[0].(*PictureBinary)
	*pic = *resultPic
	return
}

type entry struct {
	fillType string
	imgName  string
	text     string
}

var entries []entry

// LoadIndex load index info
func (psx *PictureConnection) LoadIndex(insert bool, fileName string, ada *adabas.Adabas) error {
	fmt.Println("Load index", fileName)
	i := strings.LastIndex(fileName, "/")
	directory := fileName[:i]
	albumName := directory[strings.LastIndex(directory, "/")+1:]
	fmt.Println("Got album name ", albumName, " directory=", directory)
	ps := string(os.PathSeparator)
	f, err := os.Open(fileName)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer f.Close()
	doc, derr := html.Parse(f)
	if derr != nil {
		return derr
	}
	var fctHtml func(*html.Node)
	fctHtml = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a":
				for _, a := range n.Attr {
					if a.Key == "class" && strings.Contains(a.Val, "navbar-brand") {
						var buffer bytes.Buffer
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							buffer.WriteString(c.Data)
						}
						adatypes.Central.Log.Debugf("Title -> %s", buffer.String())
						break
					}
				}
			case "div":
				for _, a := range n.Attr {
					if a.Key == "class" && strings.Contains(a.Val, "item") {
						e := entry{}
						adatypes.Central.Log.Debugf("Found item: %s", a.Val)
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							switch c.Data {
							case "video":
								for _, sa := range c.Attr {
									adatypes.Central.Log.Debugf("VideoX -> %s", sa.Val)
									if sa.Key == "class" {
										e.fillType = sa.Val
									}
								}
								for s := c.FirstChild; s != nil; s = s.NextSibling {
									if s.Data == "source" {
										adatypes.Central.Log.Debugf("VideoY -> %s", s.Data)
										for _, sb := range s.Attr {
											if sb.Key == "src" {
												adatypes.Central.Log.Debugf("VideoZ -> %s", sb.Key, sb.Val)
												li := strings.LastIndex(sb.Val, "/")
												e.imgName = sb.Val[li+1:]
											}
										}
									}
								}
							case "div":
								for _, sa := range c.Attr {
									switch sa.Key {
									case "style":
										adatypes.Central.Log.Debugf("Style -> %s", sa.Val)
										e.imgName = sa.Val[strings.Index(sa.Val, "/")+1 : strings.LastIndex(sa.Val, "'")]
									case "class":
										if sa.Val == "carousel-caption" {
											adatypes.Central.Log.Debugf("classX -> %s", sa.Val)
											for s := c.FirstChild; s != nil; s = s.NextSibling {
												for sb := s.FirstChild; sb != nil; sb = sb.NextSibling {
													adatypes.Central.Log.Debugf(" -> %s", sb.Data)
													e.text = sb.Data
												}
											}
										} else {
											adatypes.Central.Log.Debugf("Fill -> %s", sa.Val)
											e.fillType = sa.Val
										}
									}
								}
							}
						}
						err = psx.LoadPicture(insert, directory+ps+"img"+ps+e.imgName, ada)
						if err != nil {
							adatypes.Central.Log.Debugf("Loaded %s with error=%v", directory+ps+"img"+ps+e.imgName, err)
							fmt.Println("Error loading picture:", err)
							os.Exit(1)
						}
						entries = append(entries, e)
						break
					}
				}
			default:
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			fctHtml(c)
		}
	}
	fctHtml(doc)

	// fmt.Println("Entries:", entries)

	return nil
}

func loadMovie(fileName string, ada *adabas.Adabas) error {
	fmt.Println("Load movie", fileName)
	return nil
}

// StorePicture store picture data
func (pic *PictureBinary) StorePicture() error {
	s := &Store{}
	s.Store = append(s.Store, pic)
	err := pic.LoadFile()
	if err != nil {
		panic("Error loading file " + err.Error())
	}
	if strings.HasPrefix(pic.MetaData.MIMEType, "image") {
		pic.CreateThumbnail()
	}
	jsonPicture, jerr := json.Marshal(s)
	if jerr != nil {
		panic("Error json marshalling file " + jerr.Error())
	}

	sr, err := SendJSON(PictureName, jsonPicture)
	if err != nil {
		return err
	}
	if sr == nil {
		return fmt.Errorf("Error store nil")
	}
	// i, _ := strconv.Atoi(sr.Stored[0])
	// p.Isn = uint32(i)
	// pic.MetaData.Isn = uint32(sr.Stored[0])
	fmt.Println("Created record on ISN=", pic.MetaData.Index)
	pic.sendBinary(PictureName, true)
	if strings.HasPrefix(pic.MetaData.MIMEType, "image") {
		pic.sendBinary(PictureName, false)
	}
	return nil
}

func (pic *PictureBinary) sendBinary(mapName string, isPicture bool) *StoreResponse {
	data := pic.Data.Media
	field := "Media"
	if !isPicture {
		data = pic.Data.Thumbnail
		field = "Thumbnail"
	}
	mapURL := strings.Replace(URL, "rest/", "binary/", -1) +
		"/" + mapName + "/" + strconv.Itoa(int(pic.MetaData.Index)) + "/" + field
	adatypes.Central.Log.Debugf("Binary URL:>", mapURL, "on ISN=", pic.MetaData.Index)

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	//bodyWriter.WriteField(k, v.(string))
	fileWriter, err := bodyWriter.CreateFormFile("uploadLob", pic.FileName)
	if err != nil {
		fmt.Println(err)
		//fmt.Println("Create form file error: ", error)
		return nil
	}
	fileWriter.Write(data)
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()
	fmt.Println("Put binary")
	req, err := http.NewRequest("PUT", mapURL, bodyBuf)
	c := strings.Split(Credentials, ":")
	req.SetBasicAuth(c[0], c[1])
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Client do error:", err)
		fmt.Println(resp, err)
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Headers:", resp.Header)
		fmt.Println("response Body:", string(body))
		fmt.Println("Malformed binary request")
		return nil
	}
	s := &StoreResponse{}
	json.Unmarshal(body, s)
	return s
}

// DeleteMd5 delete picture key
func (psx *PictureConnection) DeleteMd5(a *adabas.Adabas, key string) error {
	result, err := psx.readCheck.ReadLogicalWith("Md5=" + key)
	if err != nil {
		fmt.Printf("Error checking Md5=%s: %v\n", key, err)
		panic("Read error " + err.Error())
		//		return false, err
	}

	deleteRequest, err := adabas.NewMapNameDeleteRequest(a, psx.store.MapName)
	defer deleteRequest.BackoutTransaction()
	if err != nil {
		return err
	}
	for _, r := range result.Values {
		deleteRequest.Delete(r.Isn)
	}
	return nil
}

// DeleteIsn delete image Isn
func (psx *PictureConnection) DeleteIsn(a *adabas.Adabas, isn adatypes.Isn) error {
	fmt.Printf("Delete image with ISN=%d\n", isn)
	deleteRequest, err := adabas.NewMapNameDeleteRequest(a, psx.store.MapName)
	defer deleteRequest.BackoutTransaction()
	if err != nil {
		return err
	}
	err = deleteRequest.Delete(isn)
	if err != nil {
		return err
	}
	err = deleteRequest.EndTransaction()
	return err
}

// DeletePath delete image given with path
func (psx *PictureConnection) DeletePath(a *adabas.Adabas, path string) error {
	if path == "" {
		return nil
	}
	fmt.Printf("Delete image with path=%s\n", path)
	readRequest, err := adabas.NewReadRequest(a, psx.store.MapName)
	if err != nil {
		return err
	}
	readRequest.QueryFields("")
	result, resErr := readRequest.ReadLogicalWith("PictureName=" + path)
	if resErr != nil {
		return resErr
	}
	if result.NrRecords() != 1 {
		fmt.Printf("Found more then one or no record: %d\n", result.NrRecords())
		return fmt.Errorf("Found more then one record")
	}
	for _, record := range result.Values {
		psx.DeleteIsn(a, record.Isn)
	}
	return nil
}
