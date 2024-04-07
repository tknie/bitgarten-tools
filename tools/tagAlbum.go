package tools

import (
	"fmt"
	"tux-lobload/sql"

	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

type TagAlbumParameter struct {
	ListSource bool
}

func TagAlbum(parameter *TagAlbumParameter) {
	connSource, err := sql.DatabaseConnect()
	if err != nil {
		return
	}

	if parameter.ListSource {
		err = connSource.ListAlbums()
		if err != nil {
			fmt.Println("List albums error:", err)
			return
		}
		return
	}

	albums, err := connSource.GetAlbums()
	if err != nil {
		fmt.Println("Error reading albums:", err)
		return
	}
	log.Log.Debugf("Received Albums count = %d", len(albums))
	for _, a := range albums {
		log.Log.Debugf("Work on Album -> %s", a.Title)
		if a.Title != sql.DefaultAlbum {
			a, err = connSource.ReadAlbum(a.Title)
			if err != nil {
				fmt.Println("Error reading album:", err)
				return
			}
			a.Display()
			id, err := connSource.Open()
			if err != nil {
				fmt.Println("Error opening:", err)
				return
			}
			for _, p := range a.Pictures {
				fmt.Println(p.Description + " " + p.ChecksumPicture)
				list := [][]any{{
					p.ChecksumPicture,
					"bitgarten",
				}}
				input := &common.Entries{
					Fields: []string{
						"checksumpicture",
						"tagname",
					},
					Values: list}
				_, err = id.Insert("picturetags", input)
				if err != nil {
					fmt.Println("Error inserting:", err)
					return
				}
			}

		}
	}
}
