package sql

import (
	"fmt"
	"log"
	"tux-lobload/store"
)

func (di *DatabaseInfo) CheckExists(pic *store.Pictures) {
	pic.Available = store.NoAvailable
	rows, err := di.db.Query("select p.Sha256Checksum, pl.picturehost, pl.picturedirectory, pl.picturename FROM Pictures p, PictureLocations pl where p.ChecksumPicture=$1 AND p.ChecksumPicture = pl.ChecksumPicture",
		pic.ChecksumPicture)
	if err != nil {
		fmt.Println("Query error:", err)
		log.Fatalf("Query error database call...%v", err)
		// return false
	}
	defer rows.Close()
	sha := ""
	host := ""
	directory := ""
	name := ""
	for rows.Next() {
		pic.Available = store.PicAvailable
		rows.Scan(&sha, &host, &directory, &name)
		rows.Close()
		if sha != pic.ChecksumPictureSHA {
			fmt.Println("SHA mismatch", pic.PictureName)
			err := fmt.Errorf("SHA mismatch %s/%s", pic.Directory, pic.PictureName)
			IncError(err)
			pic.Available = store.NoAvailable
		}
		if host == hostname && directory == pic.Directory && name == pic.PictureName {
			pic.Available = store.BothAvailable
			return
		}
	}

}
