package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"tux-lobload/sql"
	"tux-lobload/store"

	"github.com/tknie/adabas-go-api/adabas"
	"github.com/tknie/adabas-go-api/adatypes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var cpMap sync.Map

func init() {
	level := zapcore.ErrorLevel
	ed := os.Getenv("ENABLE_DEBUG")
	switch ed {
	case "1":
		level = zapcore.DebugLevel
		adatypes.Central.SetDebugLevel(true)
	case "2":
		level = zapcore.InfoLevel
	}

	err := initLogLevelWithFile("converter.log", level)
	if err != nil {
		fmt.Println("Error initialize logging")
		os.Exit(255)
	}
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
		"outputPaths": [ "loadpicture.log"],
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

	sugar.Infof("Start logging with level", level)
	adatypes.Central.Log = sugar

	return
}

func convertAlbum() error {
	connection, cerr := adabas.NewConnection("acj;inmap=adatcp://lion.fritz.box:64150,5")
	if cerr != nil {
		return cerr
	}
	defer connection.Close()
	request, err := connection.CreateMapReadRequest(&store.Album{})
	if err != nil {
		return err
	}
	err = request.QueryFields("*")
	if err != nil {
		fmt.Println("Query fields error:", err)
		return err
	}
	response, rerr := request.ReadPhysicalWithCursoring()
	if rerr != nil {
		fmt.Println("Read fields error:", rerr)
		return rerr
	}

	db, err := sql.CreateConnection()
	if err != nil {
		fmt.Println("Connection error:", rerr)
		return err
	}
	n := time.Now()
	count := 0
	var sumAdabasUsed time.Duration
	for response.HasNextRecord() {
		record, err := response.NextData()
		if err != nil {
			fmt.Println("Read error:", err)
			return err
		}
		sumAdabasUsed += time.Since(n)
		//		fmt.Println("Used adabas", time.Since(n))
		// _ = response.DumpData()
		a := record.(*store.Album)
		for _, p := range a.Pictures {
			if m, ok := sql.Md5Map.Load(p.Md5); ok {
				fmt.Printf("Map md5=<%s> use instead <%s>\n", p.Md5, m.(string))
				p.Md5 = m.(string)
			}
		}

		fmt.Println("Found entry and insert ", a.Title, time.UnixMilli(int64(a.Date*1000)))
		err = db.InsertAlbum(a)
		if err != nil {
			fmt.Println("SQL insert error:", err)
			return err
		}
		count++
		if count%100 == 0 {
			fmt.Println("Read adabas records", count)
		}
		n = time.Now()
	}
	fmt.Println("Used adabas", sumAdabasUsed)

	fmt.Printf("Load %d albums...\n", count)
	return nil
}

func convertPictures(nr int) error {
	connection, cerr := adabas.NewConnection("acj;inmap=adatcp://lion.fritz.box:64150,6")
	if cerr != nil {
		return cerr
	}
	defer connection.Close()
	fmt.Printf("Created Adabas connection...\n")
	request, err := connection.CreateMapReadRequest(&store.Pictures{})
	if err != nil {
		return err
	}
	err = request.QueryFields("*")
	if err != nil {
		fmt.Println("Query fields error:", err)
		return err
	}
	request.Multifetch = 2
	response, rerr := request.ReadPhysicalWithCursoring()
	if rerr != nil {
		fmt.Println("Read fields error:", rerr)
		return rerr
	}
	descrRequest, err := connection.CreateMapReadRequest(&store.Pictures{})
	if err != nil {
		return err
	}
	err = descrRequest.QueryFields("M5,CP")
	if err != nil {
		return err
	}
	// db, err := sql.CreateConnection()
	// if err != nil {
	// 	fmt.Println("Connection error:", rerr)
	// 	return err
	// }
	// defer db.Close()
	for i := 0; i < nr; i++ {
		go sql.InsertWorker()
	}
	count := 0
	var sumAdabasUsed time.Duration
	for response.HasNextRecord() {
		n := time.Now()
		record, err := response.NextData()
		if err != nil {
			fmt.Println("Read error:", err)
			return err
		}
		sumAdabasUsed += time.Since(n)
		// _ = response.DumpData()
		pic := record.(*store.Pictures)
		descRes, err := descrRequest.ReadLogicalWith("CP=" + pic.ChecksumPicture)
		if err != nil {
			log.Fatal("Err log read")
			return err
		}
		if len(descRes.Data) > 1 {
			// log.Fatalf("Error length != 0 (%d/%d) count=%d", len(descRes.Values), len(descRes.Data), count)
			fmt.Printf("Descriptors > 1 for CP=%s Md5=%s\n", pic.ChecksumPicture, pic.Md5)
			if pic.Md5 == "" {
				fmt.Printf("Map md5 (%s) empty for %s\n", pic.Md5, pic.Title)
				for _, d := range descRes.Data {
					picData := d.(*store.Pictures)
					if picData.Md5 != "" {
						pic.Md5 = picData.Md5
						pic.Title = picData.Title
					}
				}
				if pic.Md5 == "" {
					fmt.Println("Skip picture md5 empty CP=", pic.ChecksumPicture)
					continue
				}
			}
			if m, ok := cpMap.LoadOrStore(pic.ChecksumPicture, pic.Md5); ok {
				fmt.Println("Skip picture md5=", pic.Md5, "for", m.(string), "CP=", pic.ChecksumPicture)
				continue
			} else {
				for _, d := range descRes.Data {
					picData := d.(*store.Pictures)
					sql.Md5Map.Store(picData.Md5, pic.Md5)
					fmt.Println("Map md5", picData.Md5, "use instead", pic.Md5)
				}
			}
			pic.ExifReader()

		}
		fmt.Printf("Before <%s> -> <%s>\n", pic.Md5, pic.ChecksumPicture)
		pic.Md5 = strings.Trim(pic.Md5, " ")
		pic.ChecksumPicture = strings.Trim(pic.ChecksumPicture, " ")
		fmt.Printf("After  <%s> -> <%s>\n", pic.Md5, pic.ChecksumPicture)
		sql.StorePictures(pic)
		// fmt.Println("Found entry and insert ", pic.Title, pic.ChecksumPicture)
		// err = sql.InsertPictures(db, pic)
		// if err != nil {
		// 	fmt.Println("SQL insert error:", err)
		// 	return err
		// }
		// n = time.Now()
		count++
		if count%100 == 0 {
			fmt.Println("Read adabas records", count)
		}
	}
	fmt.Println("Used adabas", sumAdabasUsed)
	fmt.Printf("Load %d pictures, waiting ...\n", count)
	sql.WaitStored()
	fmt.Printf("Stop all worker...\n")
	for i := 0; i < nr; i++ {
		sql.StopWorker()
	}
	return nil
}

func main() {
	verify := false
	skip := false
	picOnly := false

	workerNr := 1
	flag.IntVar(&workerNr, "w", 4, "Nr of workers for sql insert")
	flag.BoolVar(&verify, "v", false, "Verify data")
	flag.BoolVar(&picOnly, "p", false, "Load picture data only")
	flag.BoolVar(&skip, "s", false, "Skip picture load")
	flag.Parse()

	fmt.Printf("Number of worker   : %d\n", workerNr)
	fmt.Printf("Skip picture load  : %v\n", skip)
	fmt.Printf("Load picture only  : %v\n", picOnly)
	fmt.Printf("Verify picture data: %v\n", verify)

	err := sql.Display()
	if err != nil {
		fmt.Println("SQL display error", err)
		return
	}
	if !skip || picOnly {
		fmt.Println("Convert pictures ...")
		err = convertPictures(workerNr)
		if err != nil {
			fmt.Println("SQL convert pictures error", err)
			return
		}
	}
	fmt.Println("Convert albums ...")
	if !picOnly {
		err = convertAlbum()
		if err != nil {
			fmt.Println("SQL convert album error", err)
			return
		}
	}
}
