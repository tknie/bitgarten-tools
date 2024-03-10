package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"
	"tux-lobload/store"

	"github.com/corona10/goimagehash"
	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/goheif"
	"github.com/tknie/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var url = os.Getenv("POSTGRES_URL")

func init() {
	level := zapcore.ErrorLevel
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("imagehash.log", level)
	if err != nil {
		fmt.Println("Error initialize logging")
		os.Exit(255)
	}
	os.Setenv("PGAPPNAME", "Bitgarten hash")
}

func initLogLevelWithFile(fileName string, level zapcore.Level) (err error) {
	p := os.Getenv("LOGPATH")
	if p == "" {
		p = "."
	}
	name := p + string(os.PathSeparator) + fileName

	rawJSON := []byte(`{
		"level": "error",
		"encoding": "console",
		"outputPaths": [ "default.log"],
		"errorOutputPaths": ["stderr"],
		"encoderConfig": {
		  "messageKey": "message",
		  "levelKey": "level",
		  "levelEncoder": "lowercase"
		}
	  }`)

	var cfg zap.Config
	if err := json.Unmarshal(rawJSON, &cfg); err != nil {
		fmt.Println("Error initialize logging (json)")
		os.Exit(255)
	}
	cfg.Level.SetLevel(level)
	cfg.OutputPaths = []string{name}
	logger, err := cfg.Build()
	if err != nil {
		fmt.Println("Error initialize logging (build)")
		os.Exit(255)
	}
	defer logger.Sync()

	sugar := logger.Sugar()

	log.Log = sugar
	log.Log.Infof("Start logging with level %s", level)
	log.SetDebugLevel(level == zapcore.DebugLevel)

	return
}

func main() {
	limit := 10
	preFilter := ""

	flag.IntVar(&limit, "l", 50, "Maximum number of records loaded")
	flag.StringVar(&preFilter, "f", "", "Prefix of title used in search")
	flag.Parse()

	if preFilter != "" {
		preFilter = fmt.Sprintf(" AND title LIKE '%s%%'", preFilter)
	}

	id, err := flynn.Handle(url)
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return
	}
	query := &common.Query{
		TableName:  "pictures",
		Fields:     []string{"ChecksumPicture", "title", "mimetype", "media"},
		DataStruct: &store.Pictures{},
		Limit:      uint32(limit),
		Search: "markdelete = false AND mimetype LIKE 'image/%'" + preFilter +
			" and not exists(select 1 from picturehash ph where ph.checksumpicture = tn.checksumpicture and ph.updated_at < current_date + interval '1 week')",
	}
	_, err = id.Query(query, func(search *common.Query, result *common.Result) error {
		p := result.Data.(*store.Pictures)
		buffer := bytes.NewBuffer(p.Media)
		var h *goimagehash.ImageHash
		switch strings.ToLower(p.MIMEType) {
		case "image/heic":
			h, err = hashHeic(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		case "image/jpeg", "image/jpg":
			h, err = hashJpeg(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		case "image/png":
			h, err = hashPng(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		case "image/gif":
			h, err = hashGif(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		default:
			fmt.Printf("Error unknown image format for %s/%s: %s\n", p.Title, p.ChecksumPicture, p.MIMEType)
			log.Log.Errorf("Error unknown image format for %s/%s: %s\n", p.Title, p.ChecksumPicture, p.MIMEType)
			return nil
		}
		fmt.Printf("%s -> %+v\n", p.Title, h)
		insertHash(p, h)
		return nil
	})
	if err != nil {
		fmt.Println("Query error:", err)
	}
	fmt.Println()
}

func insertHash(p *store.Pictures, h *goimagehash.ImageHash) error {
	wid, err := flynn.Handle(url)
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return nil
	}
	insert := &common.Entries{
		Fields: []string{"checksumpicture", "hash", "kind"},
		Values: [][]any{{p.ChecksumPicture, h.GetHash(), byte(h.GetKind())}},
	}
	err = wid.Insert("picturehash", insert)
	if err != nil {
		fmt.Println("Error inserting :", err)
		return err
	}
	return nil
}

func hashHeic(f io.Reader) (*goimagehash.ImageHash, error) {
	i, err := goheif.Decode(f)
	if err != nil {
		return nil, err
	}
	return goimagehash.PerceptionHash(i)
}

func hashJpeg(f io.Reader) (*goimagehash.ImageHash, error) {
	i, err := jpeg.Decode(f)
	if err != nil {
		return nil, err
	}
	return goimagehash.PerceptionHash(i)
}

func hashPng(f io.Reader) (*goimagehash.ImageHash, error) {
	i, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return goimagehash.PerceptionHash(i)
}

func hashGif(f io.Reader) (*goimagehash.ImageHash, error) {
	i, err := gif.Decode(f)
	if err != nil {
		return nil, err
	}
	return goimagehash.PerceptionHash(i)
}
