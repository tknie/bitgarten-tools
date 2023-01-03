package sql

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"io"
	"log"
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

var picChannel = make(chan *store.Pictures)
var stop = make(chan bool)
var wg sync.WaitGroup
var sqlSendCounter = uint32(0)
var sqlInsertCounter = uint32(0)
var sqlSkipCounter = uint32(0)

func CreateConnection() (*DatabaseInfo, error) {
	dc := &DataConfig{User: "admin", Password: "Testtkn1+", URL: "lion.fritz.box", Port: 5432}
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

func (di *DatabaseInfo) used(s string, t time.Time) {
	// fmt.Println("Used for "+s+":", time.Since(t))
	di.duraction += time.Since(t)
}

func (di *DatabaseInfo) InsertAlbum(album *store.Album) error {
	defer di.used("insert sql", time.Now())

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
	// stmt, err := tx.Prepare("insert into Albums ( ID, Title, AlbumKey, Directory, Thumbnail ) VALUES($1,$2,$3,$4,$5)")
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
	err = tx.QueryRow("insert into Albums ( Title, AlbumKey, Directory, Thumbnail, published ) VALUES($1,$2,$3,$4,$5) RETURNING ID",
		album.Title, album.Key, album.Directory, album.Thumbnail, time.UnixMilli(int64(album.Date*1000))).Scan(&newID)
	if err != nil {
		if !checkErrorContinue(err) {
			fmt.Printf("Error inserting Albums(unknown): %v\n", err)
			tx.Rollback()
			return err
		}
	}
	for i, p := range album.Pictures {
		fmt.Println("Insert album picture Md5=", p.Md5)
		if p.Md5 == "" {
			log.Fatalf("Pic MD5 empty, %s", p.Name)
		}
		if m, ok := Md5Map.Load(p.Md5); ok {
			p.Md5 = m.(string)
		}
		_, err := tx.Exec("insert into AlbumPictures ( Index, AlbumId, Name, Description, Md5, Fill, Height, Width, SkipTime, MIMEType)"+
			" VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
			strconv.Itoa(i+1), strconv.Itoa(int(newID)), p.Name, p.Description, p.Md5, p.Fill, strconv.Itoa(int(p.Height)), strconv.Itoa(int(p.Width)), strconv.Itoa(int(p.Interval)), p.MIMEType)
		if err != nil {
			if !checkErrorContinue(err) {
				tx.Rollback()
				fmt.Printf("Error inserting in AlbumPictures: %v\n", err)
				return err
			}
		}
	}
	err = tx.Commit()
	if err != nil {
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
	picChannel <- pic
	wg.Add(1)
	n := atomic.AddUint32(&sqlSendCounter, 1)
	if n%100 == 0 {
		fmt.Println("Store request", n)
	}
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
	defer di.db.Close()
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

func (di *DatabaseInfo) InsertPictures(pic *store.Pictures) error {
	defer di.used("insert sql", time.Now())
	// Create a new context, and begin a transaction
	//ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancelfunc()
	ctx := context.Background()
	tx, err := di.db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Insert picture Md5=", pic.Md5, "CP=", pic.ChecksumPicture)
	_, err = tx.ExecContext(ctx, "insert into Pictures (ChecksumPicture, Md5, Title, Directory, Fill, Height, Width, Media, Thumbnail,mimetype)"+
		" VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
		pic.ChecksumPicture, pic.Md5, pic.Title, pic.Directory, pic.Fill, pic.Height,
		pic.Width, pic.Media, pic.Thumbnail, pic.MIMEType)
	if err != nil {
		fmt.Println("Error rolling back pic data md5=", pic.Md5, pic.PictureName, "CP=", pic.ChecksumPicture)
		if !checkErrorContinue(err) {
			tx.Rollback()
			//fmt.Println("Error inserting Pictures", err, len(pic.Media))
			return err
		}
	}
	fmt.Println("Insert picture location CP=", pic.ChecksumPicture)
	ins := "insert into PictureLocations (PictureName, ChecksumPicture, PictureHost, PictureDirectory)" +
		" VALUES($1,$2,$3,$4)"
	_, err = tx.ExecContext(ctx, ins,
		pic.PictureName, pic.ChecksumPicture, "", "")
	if err != nil {
		if !checkErrorContinue(err) {
			fmt.Println("Error rolling back Md5=", pic.Md5, pic.PictureName, "CP=", pic.ChecksumPicture)
			tx.Rollback()
			fmt.Println("Error inserting PictureLocations",
				pic.ChecksumPicture, pic.PictureName, err)
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	fmt.Println("Commited pic: md5=", pic.Md5, pic.PictureName, "CP=", pic.ChecksumPicture)
	n := atomic.AddUint32(&sqlInsertCounter, 1)
	if n%20 == 0 {
		fmt.Print(".")
	}
	if n%100 == 0 {
		fmt.Print("\n")
	}

	return nil
}

func (dc *DataConfig) MySQLConnection() (string, string) {
	return "mysql", fmt.Sprintf("%s:%s@tcp(%s)/Bitgarten", dc.User, dc.Password, dc.URL)
}

func (dc *DataConfig) PostgresConnection() (string, string) {
	return "postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dc.URL, dc.Port, dc.User, dc.Password, "Bitgarten")
}

func (dc *DataConfig) PostgresXConnection() (string, string) {
	// urlExample := "postgres://username:password@localhost:5432/database_name"
	return "pgx", fmt.Sprintf("postgres://%s:%s@%s:%d/%s", dc.User, dc.Password, dc.URL, dc.Port, "Bitgarten")
}

func Display() error {
	dc := &DataConfig{User: "admin", Password: "Testtkn1+", URL: "lion.fritz.box", Port: 5432}
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
