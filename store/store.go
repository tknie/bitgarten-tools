package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/tknie/adabas-go-api/adatypes"
)

var URL string
var Credentials string
var PictureName string

// Store store record
type Store struct {
	Store []interface{}
}

// StoreResponse response information
type StoreResponse struct {
	// NrStored number of stored entries
	NrStored int64 `json:"NrStored,omitempty"`
	// Stored stored json
	Stored []int64 `json:"Stored"`
}

// SendJSON send json data to server
func SendJSON(mapName string, jsonStr []byte) (*StoreResponse, error) {
	mapURL := URL + "/" + mapName
	adatypes.Central.Log.Debugf("URL:>", mapURL)

	req, err := http.NewRequest("POST", mapURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		return nil, err
	}
	c := strings.Split(Credentials, ":")
	req.SetBasicAuth(c[0], c[1])
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Client do errorx:", err)
		fmt.Println(resp, err)
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		if !strings.Contains(string(body), "ADAGE62000") {
			fmt.Println("URL  :", resp.Status)
			fmt.Println("response Status  :", mapURL)
			fmt.Println("response Headers :", resp.Header)
			fmt.Println("response Body    :", string(body))
			fmt.Println("Malformed request:", mapURL)
		} else {
			fmt.Println("Record already stored")
			return nil, fmt.Errorf("Record already stored %d", resp.StatusCode)
		}
		//return nil, fmt.Errorf("Malformed call %d", resp.StatusCode)
		panic(fmt.Sprintf("Malformed call %d", resp.StatusCode))
	}
	s := &StoreResponse{}
	err = json.Unmarshal(body, s)
	if err != nil {
		return nil, err
	}
	return s, nil
}
