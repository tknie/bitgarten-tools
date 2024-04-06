package sql

import (
	"fmt"
	"os"

	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

func DatabaseConnect() (*DatabaseInfo, error) {
	sourceUrl := os.Getenv("POSTGRES_URL")
	pwd := os.Getenv("POSTGRES_PASSWORD")
	fmt.Println("Connect : " + sourceUrl)
	connSource, err := Connect(sourceUrl, pwd)
	if err != nil {
		fmt.Println("Error opening connection:", err)
		fmt.Println("Set POSTGRES_URL and/or POSTGRES_PASSWORD to define remote database")
		return nil, err
	}
	return connSource, nil
}

func DatabaseLocation() (*common.Reference, string, error) {
	url := os.Getenv("POSTGRES_URL")

	ref, passwd, err := common.NewReference(url)
	if err != nil {
		fmt.Println("URL error:", err)
		return nil, "", err
	}
	if passwd == "" {
		passwd = os.Getenv("POSTGRES_PASSWORD")
	}
	return ref, passwd, nil
}

func DatabaseHandler() (common.RegDbID, error) {
	ref, passwd, err := DatabaseLocation()
	if err != nil {
		return 0, err
	}
	log.Log.Debugf("Connect to %s:%d", ref.Host, ref.Port)
	id, err := flynn.Handler(ref, passwd)
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return 0, err
	}
	return id, nil
}
