package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"

	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type PictureHash struct {
	Checksumpicture string
	PerceptionHash  string
}

type PictureHashCount struct {
	Count          int
	PerceptionHash string
}

var url = os.Getenv("POSTGRES_URL")
var commit = false

const defaultLimit = 20

var limit = defaultLimit

const readHashs = `
SELECT count(hash) AS count,
perceptionhash
   FROM picturehash p
  WHERE (EXISTS ( SELECT 1
           FROM pictures pp
          WHERE pp.checksumpicture::text = p.checksumpicture::text AND pp.markdelete = false))
  GROUP BY perceptionhash
 HAVING count(perceptionhash) > {{.Count}}
  ORDER BY (count(perceptionhash)) DESC
  LIMIT {{.Limit}}
`

const readPictureByHashs = `
select checksumpicture, title, height, width, Exifxdimension, Exifydimension,
( SELECT string_agg(DISTINCT (''''::text || pt.tagname::text) || ''''::text, ','::text) AS string_agg
           FROM picturetags pt
          WHERE pt.checksumpicture::text = p.checksumpicture::text) AS tags
from pictures p where markdelete = false and EXISTS ( SELECT 1
	FROM picturehash pp
   WHERE pp.perceptionhash = {{.}} AND pp.checksumpicture::text = p.checksumpicture::text );
`

func init() {
	level := zapcore.ErrorLevel
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("hashclean.log", level)
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
	var hashType string

	flag.IntVar(&limit, "l", defaultLimit, "Maximum number of records loaded")
	flag.StringVar(&hashType, "h", "", "Hash type to use, valid are (averageHash,perceptHash,diffHash,waveletHash), default perceptHash")
	flag.BoolVar(&commit, "c", false, "Enable commit to database")
	flag.Parse()

	fmt.Println("Query database entries for one week not hashed commit=", commit)
	hashList, err := queryHash()
	if err != nil {
		fmt.Println("Error query max hash:", err)
		return
	}
	for i, h := range hashList {
		if h == "0" {
			fmt.Println("Breaking found empty hash")
			break
		}
		fmt.Printf("Working on %d.Hash %s\n", i+1, h)
		err = queryPictureByHash(h)
		if err != nil {
			fmt.Println("Error query max hash:", err)
			return
		}
	}
	fmt.Println("Final end")
}

func queryHash() ([]string, error) {
	id, err := flynn.Handle(url)
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return nil, err
	}
	sql, err := templateSql(readHashs, struct {
		Limit int
		Count int
	}{limit, 2})
	if err != nil {
		return nil, err
	}
	query := &common.Query{
		TableName: "picturehash",
		//Fields:     []string{"count(perceptionhash) as count", "perceptionhash"},
		DataStruct: &PictureHashCount{},
		Limit:      uint32(limit),
		Search:     sql,
	}
	counter := uint64(0)
	hash := uint64(0)
	hashList := make([]string, 0)
	err = id.BatchSelectFct(query, func(search *common.Query, result *common.Result) error {
		ph := result.Rows
		v := ph[1].(*string)
		log.Log.Debugf("Hash found: %v - %v", ph[0], v)
		hashList = append(hashList, *v)
		counter++
		return nil
	})
	if err != nil {
		fmt.Println("Error query ...:", err)
		return nil, err
	}
	log.Log.Debugf("Query hash end: %v -> %d, hash=%d", err, counter, hash)
	return hashList, nil
}

type PictureByHash struct {
	Checksumpicture string
	Title           string
	Height          int
	Width           int
	Exifxdimension  int
	Exifydimension  int
	Tags            string
	delete          bool `flynn:":ignore"`
}

func templateSql(t string, p any) (string, error) {
	t1 := template.New("t1")
	t1 = template.Must(t1.Parse(t))
	var sql bytes.Buffer
	err := t1.Execute(&sql, p)
	if err != nil {
		return "", err
	}
	return sql.String(), nil
}

func queryPictureByHash(hash string) error {
	id, err := flynn.Handle(url)
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return err
	}
	sqlCmd, err := templateSql(readPictureByHashs, hash)
	if err != nil {
		return err
	}

	picturesByHash := make([]*PictureByHash, 0)

	query := &common.Query{
		TableName:  "pictures",
		Fields:     []string{"*"},
		DataStruct: &PictureByHash{},
		Search:     sqlCmd,
	}
	counter := uint64(0)
	err = id.BatchSelectFct(query, func(search *common.Query, result *common.Result) error {
		ph := result.Data.(*PictureByHash)
		log.Log.Debugf("Picture found: %#v", ph)
		newPH := &PictureByHash{}
		*newPH = *ph
		picturesByHash = append(picturesByHash, newPH)
		counter++
		return nil
	})
	if err != nil {
		fmt.Println("Error query ...:", err)
		return err
	}

	sort.SliceStable(picturesByHash, func(x, y int) bool {
		return picturesByHash[x].Width > picturesByHash[y].Width
	})
	fmt.Printf("Found %d picture hash entries\n", len(picturesByHash))
	var firstFound *PictureByHash
	tagMap := make(map[string]bool)
	for _, pbh := range picturesByHash {
		if !strings.Contains(pbh.Tags, "'bitgarten'") {
			if firstFound == nil {
				firstFound = pbh
			} else {
				pbh.delete = true
			}
			if pbh.Tags != "" {
				tags := strings.Split(pbh.Tags, ",")
				for _, t := range tags {
					tagMap[t] = true
				}
			}
		} else {
			if pbh.Tags != "" {
				tags := strings.Split(pbh.Tags, ",")
				for _, t := range tags {
					if t != "'bitgarten'" {
						tagMap[t] = true
					}
				}
			}
		}
		log.Log.Debugf("Find picture hash %#v", pbh)
	}
	if firstFound == nil {
		fmt.Printf("No first found out of %d\n", len(picturesByHash))
	} else {
		err = cleanUpPictures(tagMap, firstFound, picturesByHash)
		if err != nil {
			fmt.Println("Error cleanup pictures:", err)
			return err
		}
	}
	log.Log.Debugf("Picture by hash end: %v -> %d", err, counter)
	return nil
}

func cleanUpPictures(tagMap map[string]bool, firstFound *PictureByHash, picturesByHash []*PictureByHash) error {
	id, err := flynn.Handle(url)
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return err
	}
	defer id.FreeHandler()

	err = id.BeginTransaction()
	if err != nil {
		return err
	}

	newTags := KeysString(tagMap)
	log.Log.Debugf("First: %#v -> %#v", firstFound, newTags)
	if newTags != firstFound.Tags {
		oldTagMap := KeysMap(firstFound.Tags)
		newTagMap := KeysMap(newTags)
		changed := true
		for k := range newTagMap {
			if !oldTagMap[k] {
				oldTagMap[k] = true
				changed = true
			}
		}
		if changed {
			fmt.Println("Need tag update to -> ", firstFound.Checksumpicture, newTags)
		}
	}

	for _, pbh := range picturesByHash {
		if pbh.delete {
			if pbh.Tags != "" {
				fmt.Println("Need to delete all tags for -> ", pbh.Checksumpicture, "tags=", pbh.Tags)
				log.Log.Debugf("Need to delete all tags for -> %s", pbh.Checksumpicture)
				dr, err := id.Delete("picturetags", &common.Entries{Criteria: "checksumpicture='" + pbh.Checksumpicture + "'"})
				if err != nil {
					return err
				}
				if dr == 0 {
					fmt.Printf("error deleting picture tags for %s: no entry deleted\n", pbh.Checksumpicture)
				}
				log.Log.Debugf("%d entries deleted", dr)

			}
			fmt.Println("Need to mark delete -> ", pbh.Checksumpicture)
			input := &common.Entries{
				Fields: []string{"markdelete"},
				Update: []string{"checksumpicture='" + pbh.Checksumpicture + "'"},
				Values: [][]any{{true}},
			}
			ra, err := id.Update("pictures", input)
			if err != nil {
				return nil
			}
			if ra != 1 {
				return fmt.Errorf("incorrect update mark delete of %s: %d", pbh.Checksumpicture, ra)
			}
			log.Log.Debugf("%d entries updated", ra)
		}
	}

	if commit {
		err = id.Commit()
		if err != nil {
			return err
		}
	} else {
		err = id.Rollback()
		if err != nil {
			return err
		}

	}

	return nil
}

func KeysMap(tags string) map[string]bool {
	keysMap := make(map[string]bool)
	for _, k := range strings.Split(tags, ",") {
		keysMap[k] = true
	}
	return keysMap
}

func KeysString(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ",")
}
