package sql

import (
	"fmt"
	"log"
	"tux-lobload/store"
)

const query = `with p(checksumpicture,sha256checksum) as (
	select
		checksumpicture,sha256checksum
	from
		pictures
	where
		checksumpicture = $1)
	select
		l.picturedirectory,
		l.picturename
	from
		picturelocations l,
		p
	where
		l.checksumpicture = p.checksumpicture
		and l.picturename = $3
		and l.picturedirectory = $2
		and l.picturehost = $4
	union
	select
		p.sha256checksum,
		''
	from
		p
	`

func (di *DatabaseInfo) CheckExists(pic *store.Pictures) {
	pic.Available = store.NoAvailable
	rows, err := di.db.Query(query, pic.ChecksumPicture, pic.Directory, pic.PictureName, hostname)
	if err != nil {
		fmt.Println("Query error:", err)
		log.Fatalf("Query error database call...%v", err)
		return
	}
	defer rows.Close()

	dir := ""
	name := ""
	for rows.Next() {
		pic.Available = store.PicAvailable
		err := rows.Scan(&dir, &name)
		if err != nil {
			log.Fatal("Error scanning read location check")
		}

		switch {
		case name == "" && dir != pic.ChecksumPictureSHA:
			fmt.Println("SHA mismatch", pic.PictureName)
			err := fmt.Errorf("SHA mismatch %s/%s", pic.Directory, pic.PictureName)
			IncError("SHA differs "+pic.PictureName, err)
			pic.Available = store.NoAvailable
		case name != "" && dir == pic.Directory && name == pic.PictureName:
			pic.Available = store.BothAvailable
		default:
		}
	}
}
