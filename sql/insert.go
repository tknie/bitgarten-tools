/*
* Copyright © 2023-2024 private, Darmstadt, Germany and/or its licensors
*
* SPDX-License-Identifier: Apache-2.0
*
*   Licensed under the Apache License, Version 2.0 (the "License");
*   you may not use this file except in compliance with the License.
*   You may obtain a copy of the License at
*
*       http://www.apache.org/licenses/LICENSE-2.0
*
*   Unless required by applicable law or agreed to in writing, software
*   distributed under the License is distributed on an "AS IS" BASIS,
*   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*   See the License for the specific language governing permissions and
*   limitations under the License.
*
 */

package sql

import (
	"crypto/md5"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"tux-lobload/store"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

type DataConfig struct {
	User     string
	Password string
	URL      string
	Port     int
	Database string
}

type DatabaseInfo struct {
	// Deprecated: Get rid of native SQL driver
	// db        *sql.DB
	id        common.RegDbID
	Reference *common.Reference
	passwd    string
	duraction time.Duration
	workerNr  int32
}

const DefaultAlbum = "Default Album"

var Md5Map sync.Map

var picChannel = make(chan *store.Pictures)
var stop = make(chan bool)
var wg sync.WaitGroup
var sqlSendCounter = uint32(0)
var sqlInsertCounter = uint32(0)
var sqlSkipCounter = uint32(0)
var albEntryIndex = int32(0)

var ExitOnError = false

var workerCounter = int32(0)

func init() {
}

func CreateConnection() (*DatabaseInfo, error) {
	ref, passwd, err := DatabaseLocation()
	if err != nil {
		return nil, err
	}

	dc := &DataConfig{User: ref.User, Password: passwd, URL: ref.Host,
		Port: ref.Port, Database: ref.Database}
	_, url := dc.PostgresXConnection()
	ref, pwd, err := common.NewReference(url)
	if err != nil {
		return nil, err
	}
	log.Log.Infof("Connecting to ....%s", ref.Host)
	id, err := flynn.Handler(ref, pwd)
	if err != nil {
		fmt.Println("Error db open:", err)
		return nil, err
	}
	return &DatabaseInfo{id, nil, "", 0, 0}, nil
}

func (di *DatabaseInfo) WriteAlbum(album *Albums) error {
	found, err := di.CheckAlbum(album)
	if err != nil {
		return err
	}
	log.Log.Debugf("Check Album found: %v", found)
	fmt.Printf("Search found on source %03d -> %s\n", album.Id, album.Title)

	id, err := di.Open()
	if err != nil {
		return err
	}
	defer id.FreeHandler()
	list := [][]any{{
		album.Type,
		album.Key,
		album.Directory,
		album.Title,
		album.Description,
		album.Option,
		album.ThumbnailHash,
		album.Published,
	}}
	input := &common.Entries{
		Fields: []string{
			"Type",
			"Key",
			"Directory",
			"Title",
			"Description",
			"Option",
			"ThumbnailHash",
			"Published",
		},
		Values: list}
	if found > 0 {
		input.Update = []string{"title='" + strings.Replace(album.Title, "'", "''", -1) + "'"}
		_, n, err := id.Update("Albums", input)
		if err != nil {
			return err
		}
		log.Log.Debugf("Update %d entries", n)
		album.Id = found
	} else {
		_, err = id.Insert("Albums", input)
		if err != nil {
			return err
		}
		found, err = di.CheckAlbum(album)
		if err != nil {
			return err
		}
		album.Id = found
	}
	fmt.Printf("Adapt new album id on destination %03d -> %s\n", album.Id, album.Title)
	for _, p := range album.Pictures {
		p.AlbumId = album.Id
	}

	return nil
}

func (di *DatabaseInfo) WriteAlbumPictures(albumPic *AlbumPictures) error {
	found, err := di.CheckAlbumPictures(albumPic)
	if err != nil {
		return err
	}
	log.Log.Debugf("Check AlbumPicture found: %v", found)
	id, err := di.Open()
	if err != nil {
		return err
	}
	defer id.FreeHandler()
	list := [][]any{{
		albumPic.Index,
		albumPic.AlbumId,
		albumPic.Name,
		albumPic.Description,
		albumPic.ChecksumPicture,
		albumPic.MimeType,
		albumPic.SkipTime,
		albumPic.Height,
		albumPic.Width,
	}}
	input := &common.Entries{
		Fields: []string{
			"Index",
			"AlbumId",
			"Name",
			"Description",
			"ChecksumPicture",
			"MimeType",
			"SkipTime",
			"Height",
			"Width",
		},
		Values: list}
	if found {
		input.Update = []string{fmt.Sprintf("index = %d AND albumid = %d",
			albumPic.Index, albumPic.AlbumId)}
		_, n, err := id.Update("AlbumPictures", input)
		if err != nil {
			return err
		}
		log.Log.Debugf("Update %d entries", n)
	} else {
		_, err = id.Insert("AlbumPictures", input)
		log.Log.Debugf("Update AlbumPictures entry")
	}
	if err != nil {
		return err
	}

	return nil
}

func (di *DatabaseInfo) WritePicture(pic *Picture) error {
	id, err := di.Open()
	if err != nil {
		return err
	}
	defer id.FreeHandler()
	return di.WritePictureTransaction(id, pic)
}

func (di *DatabaseInfo) WritePictureTransaction(id common.RegDbID, pic *Picture) error {

	list := [][]any{{
		pic.ChecksumPicture,
		pic.Sha256checksum,
		pic.Thumbnail,
		pic.Media,
		pic.Title,
		pic.Fill,
		pic.Mimetype,
		pic.Height,
		pic.Width,
		pic.Exifmodel,
		pic.Exifmake,
		pic.Exiftaken,
		pic.Exiforigtime,
		pic.Exifxdimension,
		pic.Exifydimension,
		pic.Exiforientation,
		pic.Created,
		pic.Updated_at,
	}}
	input := &common.Entries{
		Fields: []string{
			"ChecksumPicture",
			"Sha256checksum",
			"Thumbnail",
			"Media",
			"Title",
			"Fill",
			"Mimetype",
			"Height",
			"Width",
			"Exifmodel",
			"Exifmake",
			"Exiftaken",
			"Exiforigtime",
			"Exifxdimension",
			"Exifydimension",
			"Exiforientation",
			"Created",
			"Updated_at"},
		Values: list}
	_, err := id.Insert("Pictures", input)
	if err != nil {
		return err
	}

	return nil
}

func (di *DatabaseInfo) Close() {
	if di != nil && di.id != 0 {
		di.id.FreeHandler()
	}
}

func (di *DatabaseInfo) InsertNewAlbum(directory string) (int, error) {

	di.id.BeginTransaction()

	entries := &common.Entries{
		Fields:    []string{"Title", "Key", "Directory", "ThumbnailHash", "published"},
		Returning: []string{"ID"},
		Values:    [][]any{{DefaultAlbum, "Key", directory, "", time.Now()}},
	}
	r, err := di.id.Insert("Albums", entries)
	newID := 0
	if err != nil {
		if !checkErrorContinue(err) {
			fmt.Printf("Error inserting Albums(unknown): %v\n", err)
			di.id.Rollback()
			return 0, err
		}
		query := &common.Query{Fields: []string{"id"}, TableName: "Albums",
			Search: "title = '" + DefaultAlbum + "'"}
		_, err := di.id.Query(query, func(search *common.Query, result *common.Result) error {
			fmt.Println(result.Rows)
			newID = int(result.Rows[0].(int32))
			return nil
		})
		if err != nil {
			fmt.Printf("Error quering Albums(unknown): %v\n", err)
			return -1, err
		}
		return -1, err
	}
	fmt.Println(r[0][0])
	newID, _ = strconv.Atoi(r[0][0].(string))
	fmt.Println("Album created", newID)
	log.Log.Debugf("Album created: %d", newID)

	err = di.id.Commit()
	if err != nil {
		fmt.Println("Commit tx error:", err)
		log.Log.Fatal(err)
	}
	if newID%20 == 0 {
		fmt.Print("*")
	}
	if newID%100 == 0 {
		fmt.Print("\n")
	}
	return newID, nil
}

func (di *DatabaseInfo) InsertAlbum(album *store.Album) error {
	id, err := DatabaseHandler()
	if err != nil {
		return err
	}

	err = id.BeginTransaction()
	if err != nil {
		return err
	}

	// TODO move to flynn
	// Create a new context, and begin a transaction
	// ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancelfunc()
	// conn, _ := di.db.Conn(ctx)
	// conn.Raw(func(driverConn any) error {
	// 	c := driverConn.(*stdlib.Conn).Conn()
	// 	pgxdecimal.Register(c.TypeMap())
	// 	return nil
	// })

	newID := 0
	k := strings.Trim(album.Key, " ")
	if k == "" {
		d := strings.Trim(album.Directory, " ")
		if d == "" {
			album.Directory = strings.ReplaceAll(album.Title, " ", "")
		}
		album.Key = createStringMd5(album.Title)
	}
	if m, ok := Md5Map.Load(album.Thumbnailhash); ok {
		album.Thumbnailhash = m.(string)
	}
	insert := &common.Entries{Fields: []string{"Title", "key", "Directory", "ThumbnailHash", "published"},
		DataStruct: album,
		Values:     [][]any{{album}},
		Returning:  []string{"ID"}}
	data, err := di.id.Insert("Albums", insert)
	if err != nil {
		err2 := di.id.Rollback()
		fmt.Println("Rollback ...", err2)
		if !checkErrorContinue(err) {
			fmt.Printf("Error inserting Albums(unknown): %v\n", err)
			return err
		}
		return nil
	}
	newID = data[0][0].(int)
	for i, p := range album.Pictures {
		log.Log.Debugf("Insert picture Md5=%s", p.Md5)
		if p.Md5 == "" {
			log.Log.Fatalf("Pic MD5 empty, %s", p.Name)
		}
		if m, ok := Md5Map.Load(p.Md5); ok {
			p.Md5 = strings.Trim(m.(string), " ")
			fmt.Printf("MD5 AP <%s>", p.Md5)
		}
		p.Index = i + 1
		p.AlbumID = newID
		insertPic := &common.Entries{Fields: []string{"Index", "AlbumId", "Name",
			"Description", "ChecksumPicture",
			"Fill", "Height", "Width", "SkipTime", "MIMEType"},
			DataStruct: p,
			Values:     [][]any{{p}},
			Returning:  []string{"ID"}}
		_, err := di.id.Insert("AlbumPictures", insertPic)
		// 	_, err := tx.Exec("insert into AlbumPictures ( Index, AlbumId, Name, Description, ChecksumPicture,  Fill, Height, Width, SkipTime, MIMEType)"+
		// " VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
		// strconv.Itoa(i+1), strconv.Itoa(int(newID)), p.Name, p.Description,
		// p.Md5, p.Fill, strconv.Itoa(int(p.Height)), strconv.Itoa(int(p.Width)),
		// strconv.Itoa(int(p.Interval)), p.MIMEType)
		if err != nil {
			err2 := di.id.Rollback()
			fmt.Println("Rollback ...", err2)
			if !checkErrorContinue(err) {
				fmt.Printf("Error inserting in AlbumPictures: %v\n", err)
				return err
			}
			return nil
		}
	}
	err = di.id.Commit()
	if err != nil {
		fmt.Println("Commit tx error:", err)
		log.Log.Fatal(err)
	}
	if newID%20 == 0 {
		fmt.Print("*")
	}
	if newID%100 == 0 {
		fmt.Print("\n")
	}
	return nil
}

func checkErrorContinue(err error) bool {
	switch e := err.(type) {
	case *mysql.MySQLError:
		if e.Number == 1062 {
			atomic.AddUint32(&sqlSkipCounter, 1)
			return true
		}
		fmt.Printf("Error inserting record(no duplicate): %v\n", err)
	case *pgconn.PgError:
		if e.Code == "23505" {
			atomic.AddUint32(&sqlSkipCounter, 1)
			return true
		}
		fmt.Printf("Error inserting record(no duplicate): %v\n", err)
	default:
		fmt.Printf("Error inserting record(default): %v\n", err)
	}
	return false
}

func createStringMd5(input string) string {
	h := md5.New()
	io.WriteString(h, input)
	return fmt.Sprintf("%X", h.Sum(nil))
}

func StorePictures(pic *store.Pictures) {
	wg.Add(1)
	picChannel <- pic
	atomic.AddUint32(&sqlSendCounter, 1)
}

func WaitStored() {
	wg.Wait()
	fmt.Printf("Store requested = %d inserted = %d skipped = %d\n",
		sqlSendCounter, sqlInsertCounter, sqlSkipCounter)
}

func InsertWorker() {
	di, err := CreateConnection()
	if err != nil {
		fmt.Println("Connection error:", err)
		return
	}
	counter := uint64(0)
	workerNr := atomic.AddInt32(&workerCounter, 1)
	di.workerNr = workerNr
	defer di.Close()
	defer log.Log.Debugf("Leaving worker...")
	for {
		select {
		case pic := <-picChannel:
			log.Log.Debugf("Inserting pic in worker %d", workerNr)
			err = di.InsertPictures(pic)
			if err != nil {
				fmt.Printf("worker (%d) error inserting picture: %v\n", workerNr, err)
			}
			log.Log.Debugf("Inserting pic worker %d done", workerNr)
			wg.Done()
			counter++
		case <-stop:
			fmt.Printf("Stored data in %v count=%d\n", di.duraction, counter)
			log.Log.Debugf("Stored data in %v count=%d", di.duraction, counter)
			return
		}
	}
}

func (di *DatabaseInfo) InsertAlbumPictures(pic *store.Pictures, index, albumid int) error {
	err := di.id.BeginTransaction()
	if err != nil {
		IncError("Begin Tx "+pic.PictureName, err)
		fmt.Println("Error init Transaction storing file:", pic.PictureName, "->", err)
		return err
	}
	log.Log.Debugf("Insert album picture info Md5=%s", pic.Md5)
	insert := &common.Entries{
		Fields: []string{"index", "albumid", "name", "description",
			"checksumpicture", "mimetype", "fill", "skiptime", "height", "width"},
		Values: [][]any{{index, albumid,
			pic.Title, pic.Title + " description", pic.ChecksumPicture, pic.MIMEType,
			pic.Fill, "5000", strconv.Itoa(int(pic.Height)), strconv.Itoa(int(pic.Width))}},
	}
	log.Log.Debugf("Pic value: %#v", insert.Values[0])
	_, err = di.id.Insert("AlbumPictures", insert)
	if err != nil {
		fmt.Println("Error inserting picture album info: ", index, err)
		//return err
		log.Log.Fatalf("Error storing: %#v", insert.Values)
	}
	err = di.id.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (di *DatabaseInfo) InsertPictures(pic *store.Pictures) error {
	log.Log.Debugf("Insert picture in AlbumPictures (worker %d)", di.workerNr)
	if pic.ChecksumPictureSHA == "" {
		pic.ChecksumPictureSHA = store.CreateSHA(pic.Media)
	}
	ti := ps.IncStarted()
	err := di.id.BeginTransaction()
	if err != nil {
		IncError("BeginTx "+pic.PictureName, err)
		fmt.Println("Error init Transaction storing file:", pic.PictureName, "->", err)
		log.Log.Errorf("Error init Transaction storing file:", pic.PictureName, "->", err)
		return err
	}

	if pic.StoreAlbum > 0 {
		index := atomic.AddInt32(&albEntryIndex, 1)
		pic.Index = uint64(index)
		err = di.InsertAlbumPictures(pic, int(index), pic.StoreAlbum)
		if err != nil {
			fmt.Println("Error inserting album pictures")
			log.Log.Errorf("Error inserting album: %v", err)
			return err
		}
	}
	// fmt.Printf("Store file MD5=%s SHA=%s -> %s\n", pic.ChecksumPicture,
	// 	pic.ChecksumPictureSHA, pic.PictureName)
	log.Log.Errorf("Store file MD5=%s SHA=%s -> %s (worker %d)\n", pic.ChecksumPicture,
		pic.ChecksumPictureSHA, pic.PictureName, di.workerNr)
	if pic.Available == store.NoAvailable {
		err = insertPictureData(ti, pic)
		if err != nil {
			log.Log.Errorf("Reopen transaction")
			_ = di.id.BeginTransaction()
		}

	}
	log.Log.Debugf("Check picture location available: %d", pic.Available)
	if pic.Available == store.NoAvailable || pic.Available == store.PicAvailable {
		log.Log.Debugf("Insert picture location CP=%s worker=%d", pic.ChecksumPicture, di.workerNr)
		insert := &common.Entries{Fields: []string{"PictureName", "ChecksumPicture", "PictureHost", "PictureDirectory"},
			Values: [][]any{{pic.PictureName, pic.ChecksumPicture, store.Hostname, pic.Directory}},
		}
		_, err = di.id.Insert("PictureLocations", insert)
		if err != nil {
			log.Log.Errorf("Insert location error: %v", err)
			di.id.Rollback()
			if !checkErrorContinue(err) {
				fmt.Println("Error inserting PictureLocations",
					pic.ChecksumPicture, pic.PictureName, err)
				fmt.Println("Error rolling back Md5=", pic.Md5, pic.PictureName, "CP=", pic.ChecksumPicture)
				return err
			}
			log.Log.Debugf("Error inserting locations: %v", err)
			ti.IncDuplicateLocation()
			return nil
		}

		log.Log.Debugf("Commiting picture location (worker %d)", di.workerNr)
		err = di.id.Commit()
		if err != nil {
			IncError("Commit error "+pic.PictureName, fmt.Errorf("error commiting: %v", err))
			return err
		}
		ti.IncCommit()
		log.Log.Debugf("Commited pic", "Commited pic: md5=%s %s CP=%s worker=%d", pic.Md5, pic.PictureName,
			pic.ChecksumPicture, di.workerNr)
		atomic.AddUint32(&sqlInsertCounter, 1)
	} else {
		err = di.id.Commit()
		if err != nil {
			IncError("Commit error "+pic.PictureName, fmt.Errorf("error commiting: %v", err))
			return err
		}
		ti.IncCommit()
	}
	log.Log.Debugf("Success inserting picture (worker %d)", di.workerNr)
	return nil
}

func insertPictureData(ti *timeInfo, pic *store.Pictures) error {
	fill := pic.Fill
	if len(fill) > 1 {
		fmt.Println("Fill >1: " + fill)
		fill = fill[0:1]
	}
	orientation := pic.ExifOrientation
	if len(orientation) > 1 {
		fmt.Println("Orienation >1: " + orientation)
		orientation = orientation[0:1]
	}
	id, err := DatabaseHandler()
	if err != nil {
		return err
	}
	log.Log.Debugf("Insert picture data Md5=%s CP=%s", pic.Md5, pic.ChecksumPicture)
	inserts := &common.Entries{
		Fields: []string{"ChecksumPicture", "Sha256Checksum", "Title", "Fill",
			"Height", "Width", "Media", "Thumbnail", "mimetype", "exifmodel", "exifmake",
			"exiftaken", "exiforigtime", "exifxdimension", "exifydimension",
			"exiforientation", "created", "exif", "GPScoordinates", "GPSlatitude", "GPSlongitude"},
		Values: [][]any{{pic.ChecksumPicture, pic.ChecksumPictureSHA, pic.Title, fill, pic.Height,
			pic.Width, pic.Media, pic.Thumbnail, pic.MIMEType,
			pic.ExifModel, pic.ExifMake, pic.ExifTaken.Format(timeFormat),
			pic.ExifOrigTime.Format(timeFormat), pic.ExifXDimension, pic.ExifYDimension,
			orientation, pic.Generated, pic.Exif, pic.GPScoordinates, pic.GPSlatitude, pic.GPSlongitude}},
	}
	_, err = id.Insert("Pictures", inserts)
	if err != nil {
		id.Rollback()
		if !checkErrorContinue(err) {
			fmt.Println("Error rolling back pic data md5=", pic.Md5, pic.PictureName, "CP=", pic.ChecksumPicture)
			//fmt.Println("Error inserting Pictures", err, len(pic.Media))
			log.Log.Debugf("Error rolling back pic data md5=%s name=%s CP=%s", pic.Md5, pic.PictureName, pic.ChecksumPicture)
			IncError("ExecContext "+pic.PictureName, err)
			if ExitOnError {
				log.Log.Fatalf("Error happening")
			}
			return err
		}
		log.Log.Errorf("Error inser picture: %v", err)
		ti.IncDuplicate()
		return err
	}
	// tx.Commit()
	log.Log.Debugf("Done insert picture Md5=%s CP=%s", pic.Md5, pic.ChecksumPicture)
	ti.IncInsert()
	return nil
}

func (dc *DataConfig) MySQLConnection() (string, string) {
	return "mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s", dc.User, dc.Password, dc.URL, dc.Database)
}

func (dc *DataConfig) PostgresConnection() (string, string) {
	return "postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dc.URL, dc.Port, dc.User, dc.Password, dc.Database)
}

func (dc *DataConfig) PostgresXConnection() (string, string) {
	// urlExample := "postgres://username:password@localhost:5432/database_name"
	return "pgx", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?application_name=picloadql",
		dc.User, dc.Password, dc.URL, dc.Port, dc.Database)
}

func Display() (err error) {
	portS := os.Getenv("POSTGRES_PORT")
	port := 5432
	if portS != "" {
		port, err = strconv.Atoi(portS)
		if err != nil {
			fmt.Println("Error converting port", portS)
			return err
		}
	}
	dc := &DataConfig{User: "admin", Password: os.Getenv("POSTGRES_PASS"), URL: os.Getenv("POSTGRES_HOST"), Port: port}
	driver, url := dc.PostgresXConnection()
	db, err := sql.Open(driver,
		url)
	if err != nil {
		fmt.Println("Error db open:", err)
		return err
	}
	rows, err := db.Query("select * from Albums;")
	if err != nil {
		fmt.Println("Error db open:", err)
		return err
	}
	defer rows.Close()
	colsType, err := rows.ColumnTypes()
	if err != nil {
		fmt.Println("Error cols read:", err)
		return err
	}
	colsValue := make([]interface{}, 0)
	for _, col := range colsType {
		switch col.DatabaseTypeName() {
		case "VARCHAR2":
			s := ""
			colsValue = append(colsValue, &s)
		case "NUMBER":
			s := int64(0)
			colsValue = append(colsValue, &s)
		case "LONG":
			s := ""
			colsValue = append(colsValue, &s)
		case "DATE":
			n := time.Now()
			colsValue = append(colsValue, &n)
		default:
			s := sql.NullString{}
			colsValue = append(colsValue, &s)
		}
	}
	fmt.Println("Field entries converted: ", len(colsValue))
	for rows.Next() {
		err := rows.Scan(colsValue...)
		if err != nil {
			return err
		}
	}
	return nil
}

func StopWorker() {
	stop <- true
}
