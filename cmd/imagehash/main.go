package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"slices"
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
var defaultHash = 1
var hashes = []string{"averageHash", "perceptHash", "diffHash", "waveletHash"}
var hashType = hashes[defaultHash]

type hashData struct {
	Checksumpicture string
	Hash            uint64
	Averagehash     uint64
	PerceptionHash  uint64
	DifferenceHash  uint64
	Kind            byte
}

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
	flag.StringVar(&hashType, "h", hashes[defaultHash], "Hash type to use, valid are (averageHash,perceptHash,diffHash,waveletHash), default perceptHash")
	flag.Parse()

	fmt.Println("Start Bitgarten hash generator to find doublikates of pictures")

	if preFilter != "" {
		preFilter = fmt.Sprintf(" AND title LIKE '%s%%'", preFilter)
	}

	if !slices.Contains(hashes, hashType) {
		fmt.Println("Incorrect hash parameter given:", hashType, "not in", hashes)
		return
	}

	id, err := flynn.Handle(url)
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return
	}
	fmt.Println("Query database entries for one week not hashed")
	query := &common.Query{
		TableName:  "pictures",
		Fields:     []string{"ChecksumPicture", "title", "mimetype", "media"},
		DataStruct: &store.Pictures{},
		Limit:      uint32(limit),
		Search: "markdelete = false AND mimetype LIKE 'image/%'" + preFilter +
			" and not exists(select 1 from picturehash ph where ph.checksumpicture = tn.checksumpicture and ph.updated_at < current_date + interval '1 week')",
	}
	counter := uint64(0)
	processed := uint64(0)
	_, err = id.Query(query, func(search *common.Query, result *common.Result) error {
		counter++
		p := result.Data.(*store.Pictures)
		buffer := bytes.NewBuffer(p.Media)
		var hd *hashData
		switch strings.ToLower(p.MIMEType) {
		case "image/heic":
			hd, err = hashHeic(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		case "image/jpeg", "image/jpg":
			hd, err = hashJpeg(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		case "image/png":
			hd, err = hashPng(buffer)
			if err != nil {
				fmt.Printf("Error generating hash for %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
				log.Log.Errorf("Error generating hash for %s/%s: %v", p.Title, p.ChecksumPicture, err)
				return nil
			}
		case "image/gif":
			hd, err = hashGif(buffer)
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
		hd.Checksumpicture = p.ChecksumPicture
		hd.Hash = hd.PerceptionHash
		fmt.Printf("%s -> %s\n", p.Title, hd.Checksumpicture)
		err = insertHash(p, hd)
		if err == nil {
			processed++
		}
		return nil
	})
	if err != nil {
		fmt.Println("Query error:", err)
	}
	fmt.Printf("Found %d pictures where %d pictures are hashed", counter, processed)
	fmt.Println()
}

func insertHash(p *store.Pictures, ph *hashData) error {
	wid, err := flynn.Handle(url)
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return nil
	}
	insert := &common.Entries{
		Fields:     []string{"checksumpicture", "hash", "averagehash", "perceptionHash", "differenceHash", "kind"},
		DataStruct: ph,
		// Values:     [][]any{{p.ChecksumPicture, h.GetHash(), byte(h.GetKind())}},
		Values: [][]any{{ph}},
	}
	err = wid.Insert("picturehash", insert)
	if err != nil {
		fmt.Printf("Error inserting %s/%s: %v\n", p.Title, p.ChecksumPicture, err)
		return err
	}
	return nil
}

func hashHeic(f io.Reader) (*hashData, error) {
	i, err := goheif.Decode(f)
	if err != nil {
		return nil, err
	}
	return generateHash(i)
}

func generateHash(i image.Image) (*hashData, error) {
	ph := &hashData{}
	h, err := hash(i, hashes[0])
	if err != nil {
		return nil, err
	}
	ph.Averagehash = h.GetHash()
	h, err = hash(i, hashes[1])
	if err != nil {
		return nil, err
	}
	ph.PerceptionHash = h.GetHash()
	h, err = hash(i, hashes[2])
	if err != nil {
		return nil, err
	}
	ph.DifferenceHash = h.GetHash()
	return ph, nil
}

func hash(i image.Image, hType string) (*goimagehash.ImageHash, error) {
	switch hType {
	case hashes[0]:
		return goimagehash.AverageHash(i)
	case hashes[1]:
		return goimagehash.PerceptionHash(i)
	case hashes[2]:
		return goimagehash.DifferenceHash(i)
	case hashes[3]:
		fmt.Println("Wavelet not yet support by system")
	}
	return nil, fmt.Errorf("unknown hashType")
}

func hashJpeg(f io.Reader) (*hashData, error) {
	i, err := jpeg.Decode(f)
	if err != nil {
		return nil, err
	}
	return generateHash(i)
}

func hashPng(f io.Reader) (*hashData, error) {
	i, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return generateHash(i)
}

func hashGif(f io.Reader) (*hashData, error) {
	i, err := gif.Decode(f)
	if err != nil {
		return nil, err
	}
	return generateHash(i)
}
