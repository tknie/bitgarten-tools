package sql

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"tux-lobload/store"

	"github.com/go-sql-driver/mysql"
	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/tknie/adabas-go-api/adatypes"
)

type DataConfig struct {
	User     string
	Password string
	URL      string
	Port     int
}

type DatabaseInfo struct {
	db        *sql.DB
	duraction time.Duration
}

var Md5Map sync.Map
var hostname = ""

var picChannel = make(chan *store.Pictures)
var stop = make(chan bool)
var wg sync.WaitGroup
var sqlSendCounter = uint32(0)
var sqlInsertCounter = uint32(0)
var sqlSkipCounter = uint32(0)
var albEntryIndex = int32(0)

func init() {
	hostname, _ = os.Hostname()
	hostname = strings.Split(hostname, ".")[0]
}

func CreateConnection() (*DatabaseInfo, error) {
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "lion.fritz.box"
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "admin"
	}
	passwd := os.Getenv("POSTGRES_PASS")
	if passwd == "" {
		passwd = "lxXx"
	}
	dc := &DataConfig{User: user, Password: passwd, URL: host, Port: 5432}
	driver, url := dc.PostgresXConnection()
	connStr := url
	// connConfig, _ := pgx.ParseConfig(url)
	// connConfig.Tracer = &x{}
	// connStr := stdlib.RegisterConnConfig(connConfig)
	// dc := &DataConfig{User: "admin", Password: "Testtkn1+", URL: "lion.fritz.box:3306"}
	// driver, url := dc.MySQLConnection()
	fmt.Println("Connecting to ....", url, connStr)
	db, err := sql.Open(driver,
		connStr)
	if err != nil {
		fmt.Println("Error db open:", err)
		return nil, err
	}
	return &DatabaseInfo{db, 0}, nil
}

func (di *DatabaseInfo) Close() {
	di.db.Close()
}

func (di *DatabaseInfo) InsertNewAlbum(directory string) (int, error) {

	// Create a new context, and begin a transaction
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	conn, err := di.db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	conn.Raw(func(driverConn any) error {
		c := driverConn.(*stdlib.Conn).Conn()
		pgxdecimal.Register(c.TypeMap())
		return nil
	})
	// pgxdecimal.Register(c.TypeMap())
	// ctx := context.Background()
	tx, err := di.db.BeginTx(ctx, nil)
	if err != nil {
		fmt.Printf("Error beginning Albums transaction: %v\n", err)
		return -1, err
	}
	// fmt.Println("Insert album", time.Unix(int64(album.Date), 0))
	// stmt, err := tx.Prepare("insert into Albums ( ID, Title, Key, Directory, Thumbnail ) VALUES($1,$2,$3,$4,$5)")
	// if err != nil {
	// 	fmt.Printf("Error preparing SQL Albums: %v\n", err)
	// 	return err
	// }
	newID := 0
	err = tx.QueryRow("insert into Albums ( Title, Key, Directory, ThumbnailHash, published ) VALUES($1,$2,$3,$4,$5) RETURNING ID",
		"New Album", "Key", directory, "", time.Now()).Scan(&newID)
	if err != nil {
		if !checkErrorContinue(err) {
			fmt.Printf("Error inserting Albums(unknown): %v\n", err)
			tx.Rollback()
			return 0, err
		}
		r, err := di.db.Query("select id FROM Albums where title = 'New Album'")
		if err != nil {
			fmt.Printf("Error quering Albums(unknown): %v\n", err)
			return -1, err
		}
		if r.Next() {
			r.Scan(&newID)
			return newID, nil
		}
		return -1, err
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println("Commit tx error:", err)
		log.Fatal(err)
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

	// Create a new context, and begin a transaction
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	conn, _ := di.db.Conn(ctx)
	conn.Raw(func(driverConn any) error {
		c := driverConn.(*stdlib.Conn).Conn()
		pgxdecimal.Register(c.TypeMap())
		return nil
	})
	// pgxdecimal.Register(c.TypeMap())
	// ctx := context.Background()
	tx, err := di.db.BeginTx(ctx, nil)
	if err != nil {
		fmt.Printf("Error beginning Albums transaction: %v\n", err)
		log.Fatal(err)
	}

	// fmt.Println("Insert album", time.Unix(int64(album.Date), 0))
	// stmt, err := tx.Prepare("insert into Albums ( ID, Title, key, Directory, Thumbnail ) VALUES($1,$2,$3,$4,$5)")
	// if err != nil {
	// 	fmt.Printf("Error preparing SQL Albums: %v\n", err)
	// 	return err
	// }
	newID := 0
	k := strings.Trim(album.Key, " ")
	if k == "" {
		d := strings.Trim(album.Directory, " ")
		if d == "" {
			album.Directory = strings.ReplaceAll(album.Title, " ", "")
		}
		album.Key = createStringMd5(album.Title)
	}
	if m, ok := Md5Map.Load(album.Thumbnail); ok {
		album.Thumbnail = m.(string)
	}
	err = tx.QueryRow("insert into Albums ( Title, key, Directory, ThumbnailHash, published ) VALUES($1,$2,$3,$4,$5) RETURNING ID",
		album.Title, album.Key, album.Directory, album.Thumbnail, time.UnixMilli(int64(album.Date*1000))).Scan(&newID)
	if err != nil {
		err2 := tx.Rollback()
		fmt.Println("Rollback ...", err2)
		if !checkErrorContinue(err) {
			fmt.Printf("Error inserting Albums(unknown): %v\n", err)
			return err
		}
		return nil
	}
	for i, p := range album.Pictures {
		adatypes.Central.Log.Debugf("Insert picture Md5=%s", p.Md5)
		if p.Md5 == "" {
			log.Fatalf("Pic MD5 empty, %s", p.Name)
		}
		if m, ok := Md5Map.Load(p.Md5); ok {
			p.Md5 = strings.Trim(m.(string), " ")
			fmt.Printf("MD5 AP <%s>", p.Md5)
		}
		_, err := tx.Exec("insert into AlbumPictures ( Index, AlbumId, Name, Description, ChecksumPicture,  Fill, Height, Width, SkipTime, MIMEType)"+
			" VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
			strconv.Itoa(i+1), strconv.Itoa(int(newID)), p.Name, p.Description,
			p.Md5, p.Fill, strconv.Itoa(int(p.Height)), strconv.Itoa(int(p.Width)),
			strconv.Itoa(int(p.Interval)), p.MIMEType)
		if err != nil {
			err2 := tx.Rollback()
			fmt.Println("Rollback ...", err2)
			if !checkErrorContinue(err) {
				fmt.Printf("Error inserting in AlbumPictures: %v\n", err)
				return err
			}
			return nil
		}
	}
	err = tx.Commit()
	if err != nil {
		fmt.Println("Commit tx error:", err)
		log.Fatal(err)
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
	defer di.Close()
	for {
		select {
		case pic := <-picChannel:
			err = di.InsertPictures(pic)
			if err != nil {
				fmt.Println("Error inserting SQL picture:", err)
			}
			wg.Done()
		case <-stop:
			fmt.Printf("Stored data in %v\n", di.duraction)
			return
		}
	}
}

func (di *DatabaseInfo) InsertAlbumPictures(pic *store.Pictures, index, albumid int) error {
	ctx := context.Background()
	tx, err := di.db.BeginTx(ctx, nil)
	if err != nil {
		IncError(err)
		fmt.Println("Error init Transaction storing file:", pic.PictureName, "->", err)
		return err
	}
	adatypes.Central.Log.Debugf("Insert album picture info Md5=%s", pic.Md5)
	_, err = tx.ExecContext(ctx, "insert into AlbumPictures (index,albumid,name,description,checksumpicture,mimetype,fill,skiptime,height,width)"+
		" VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
		strconv.Itoa(index), albumid, pic.Title, pic.Title+" description", pic.ChecksumPicture, pic.MIMEType, pic.Fill, "5000", strconv.Itoa(int(pic.Height)), strconv.Itoa(int(pic.Width)))
	if err != nil {
		fmt.Println("Error inserting picture album info: ", index, err)
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (di *DatabaseInfo) InsertPictures(pic *store.Pictures) error {
	if pic.ChecksumPictureSHA == "" {
		pic.ChecksumPictureSHA = createSHA(pic.Media)
	}
	ti := IncStarted()
	ctx := context.Background()
	tx, err := di.db.BeginTx(ctx, nil)
	if err != nil {
		IncError(err)
		fmt.Println("Error init Transaction storing file:", pic.PictureName, "->", err)
		return err
	}

	if pic.StoreAlbum > 0 {
		index := atomic.AddInt32(&albEntryIndex, 1)
		err = di.InsertAlbumPictures(pic, int(index), pic.StoreAlbum)
		if err != nil {
			fmt.Println("Error inserting album pictures")
			return err
		}
	}
	if pic.Available == store.NoAvailable {
		adatypes.Central.Log.Debugf("Insert picture Md5=%s CP=%s", pic.Md5, pic.ChecksumPicture)
		_, err = tx.ExecContext(ctx, "insert into Pictures (ChecksumPicture, Sha256Checksum, Title, Fill, Height, Width, Media, Thumbnail,mimetype,exifmodel,exifmake,exiftaken,exiforigtime,exifxdimension,exifydimension,exiforientation,created)"+
			" VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)",
			pic.ChecksumPicture, pic.ChecksumPictureSHA, pic.Title, pic.Fill, pic.Height,
			pic.Width, pic.Media, pic.Thumbnail, pic.MIMEType,
			pic.ExifModel, pic.ExifMake, pic.ExifTaken.Format(timeFormat), pic.ExifOrigTime.Format(timeFormat), pic.ExifXDimension, pic.ExifYDimension, pic.ExifOrientation, pic.Generated)
		if err != nil {
			tx.Rollback()
			if !checkErrorContinue(err) {
				IncError(err)
				fmt.Println("Error rolling back pic data md5=", pic.Md5, pic.PictureName, "CP=", pic.ChecksumPicture)
				//fmt.Println("Error inserting Pictures", err, len(pic.Media))
				return err
			}
			ti.IncDuplicate()
			return nil
		}
		ti.IncInsert()
	}
	if pic.Available == store.NoAvailable || pic.Available == store.PicAvailable {
		adatypes.Central.Log.Debugf("Insert picture location CP=%s", pic.ChecksumPicture)
		ins := "insert into PictureLocations (PictureName, ChecksumPicture, PictureHost, PictureDirectory)" +
			" VALUES($1,$2,$3,$4)"
		_, err = tx.ExecContext(ctx, ins,
			pic.PictureName, pic.ChecksumPicture, hostname, pic.Directory)
		if err != nil {
			tx.Rollback()
			if !checkErrorContinue(err) {
				fmt.Println("Error inserting PictureLocations",
					pic.ChecksumPicture, pic.PictureName, err)
				fmt.Println("Error rolling back Md5=", pic.Md5, pic.PictureName, "CP=", pic.ChecksumPicture)
				return err
			}
			ti.IncDuplicateLocation()
			return nil
		}

		err = tx.Commit()
		if err != nil {
			IncError(fmt.Errorf("error commiting: %v", err))
			return err
		}
		ti.IncCommit()
		adatypes.Central.Log.Debugf("Commited pic: md5=%s %s CP=%s", pic.Md5, pic.PictureName, pic.ChecksumPicture)
		atomic.AddUint32(&sqlInsertCounter, 1)
	}
	return nil
}

func (dc *DataConfig) MySQLConnection() (string, string) {
	return "mysql", fmt.Sprintf("%s:%s@tcp(%s)/bitgarten", dc.User, dc.Password, dc.URL)
}

func (dc *DataConfig) PostgresConnection() (string, string) {
	return "postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dc.URL, dc.Port, dc.User, dc.Password, "bitgarten")
}

func (dc *DataConfig) PostgresXConnection() (string, string) {
	// urlExample := "postgres://username:password@localhost:5432/database_name"
	return "pgx", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?application_name=picloadql", dc.User, dc.Password, dc.URL, dc.Port, "bitgarten")
}

func Display() error {
	dc := &DataConfig{User: "admin", Password: os.Getenv("POSTGRES_PASS"), URL: os.Getenv("POSTGRES_HOST"), Port: 5432}
	driver, url := dc.PostgresXConnection()
	// dc := &DataConfig{User: "admin", Password: "Testtkn1+", URL: "lion.fritz.box:3306"}
	// driver, url := dc.MySQLConnection()
	fmt.Println("Connecting to ....", url)
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
	rowColumns, err := rows.Columns()
	if err != nil {
		fmt.Println("Error row read:", err)
		return err
	}
	colsType, err := rows.ColumnTypes()
	if err != nil {
		fmt.Println("Error cols read:", err)
		return err
	}
	for _, r := range rowColumns {
		fmt.Println(r)
	}
	colsValue := make([]interface{}, 0)
	for nr, col := range colsType {
		len, ok := col.Length()
		fmt.Println(nr, "name=", col.Name(), "len=", len, "ok=", ok)
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
	fmt.Println("Entries: ", len(colsValue))
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

func createSHA(input []byte) string {
	return fmt.Sprintf("%X", sha256.Sum256(input))
}
