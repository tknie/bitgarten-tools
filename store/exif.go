package store

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
	"github.com/tknie/adabas-go-api/adatypes"
)

const timeParseFormat = "2006:01:02 15:04:05"

type Printer struct {
	pic    *Pictures
	buffer bytes.Buffer
}

func (pic *Pictures) ExifReader() error {
	adatypes.Central.Log.Debugf("Exif reader pic %s", pic.MIMEType)
	if !strings.HasPrefix(pic.MIMEType, "image/") {
		return nil
	}
	buffer := bytes.NewReader(pic.Media)
	x, err := exif.Decode(buffer)
	if err != nil {
		adatypes.Central.Log.Errorf("Exif decode error:", err)
		return err
	}
	p := &Printer{pic: pic}
	err = x.Walk(p)
	if err != nil {
		adatypes.Central.Log.Errorf("Exif reader error (%s): %v\n", pic.Title, err)
		return err
	}
	pic.Exif = p.buffer.String()
	adatypes.Central.Log.Debugf("Exif result: %s", pic.Exif)
	return nil
}

func (p *Printer) Walk(name exif.FieldName, tag *tiff.Tag) error {
	p.buffer.WriteString(fmt.Sprintf("%40s: %s\n", name, tag))
	switch name {
	case "Model":
		p.pic.ExifModel = tag.String()
	case "Make":
		p.pic.ExifMake = tag.String()
	case "DateTime":
		t, err := getTime(tag.String())
		if err != nil {
			return err
		}
		p.pic.ExifTaken = t
	case "DateTimeOriginal":
		t, err := getTime(tag.String())
		if err != nil {
			return err
		}
		p.pic.ExifOrigTime = t
	case "PixelXDimension", "PixelYDimension":
		x, _ := tag.Int(0)
		p.pic.ExifXDimension = int32(x)
	case "Orientation":
		p.pic.ExifOrientation = tag.String()
	}
	return nil
}

func getTime(dateTime string) (time.Time, error) {
	tx := dateTime
	tx = strings.ReplaceAll(tx, "\"", "")
	tx = strings.ReplaceAll(tx, "/", ":")
	if tx == "0000:00:00 00:00:00" {
		return time.Time{}, nil
	}
	t, err := time.Parse(timeParseFormat, tx)
	if err != nil {
		fmt.Println("TERR", err)
		return time.Time{}, err
	}
	return t, nil
}
