package store

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
	"github.com/tknie/log"
)

const timeParseFormat = "2006:01:02 15:04:05"

type Printer struct {
	pic    *Pictures
	buffer bytes.Buffer
}

func (pic *Pictures) ExifReader() error {
	log.Log.Debugf("Exif reader pic %s", pic.MIMEType)
	if !strings.HasPrefix(pic.MIMEType, "image/") {
		return nil
	}
	buffer := bytes.NewReader(pic.Media)
	x, err := exif.Decode(buffer)
	if err != nil {
		log.Log.Debugf("Exif decode error: %v", err)
		return err
	}
	p := &Printer{pic: pic}
	err = x.Walk(p)
	if err != nil {
		log.Log.Errorf("Exif reader error (%s): %v", pic.Title, err)
		return err
	}
	pic.GPSlatitude, pic.GPSlongitude, err = x.LatLong()
	if err != nil {
		log.Log.Debugf("Exif GPS error (%s): %v", pic.Title, err)
	} else {
		p.buffer.WriteString(fmt.Sprintf("%s: %f,%f\n", "GPS", pic.GPSlatitude, pic.GPSlongitude))
		pic.GPScoordinates = fmt.Sprintf("%f,%f", pic.GPSlatitude, pic.GPSlongitude)
	}
	pic.Exif = p.buffer.String()
	log.Log.Debugf("Exif result: %s", pic.Exif)
	return nil
}

func removeQuotes(in string) string {
	toModel := strings.Trim(in, "\"")
	toModel = strings.Trim(toModel, "<>")
	toModel = strings.Trim(toModel, " ")
	return toModel
}

func (p *Printer) Walk(name exif.FieldName, tag *tiff.Tag) error {
	p.buffer.WriteString(fmt.Sprintf("%s: %s\n", name, tag))
	switch name {
	case "Model":
		p.pic.ExifModel = removeQuotes(tag.String())
	case "Make":
		p.pic.ExifMake = removeQuotes(tag.String())
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
	case "PixelXDimension":
		x, _ := tag.Int(0)
		p.pic.ExifXDimension = int32(x)
	case "PixelYDimension":
		x, _ := tag.Int(0)
		p.pic.ExifYDimension = int32(x)
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
