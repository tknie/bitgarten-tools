/*
* Copyright © 2018-2024 private, Darmstadt, Germany and/or its licensors
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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/tknie/log"

	"github.com/ogen-go/ogen/http"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/tknie/bitgartentools/api"
)

var bitgartenUrl = os.Getenv("BITGARTEN_SERVER")
var bitgartenLocation = "tmp/media/"

func init() {
	location := os.Getenv("BITGARTEN_LOCATION")
	if location != "" {
		bitgartenLocation = location
	}
}

func CheckRestClient(md5 string) (bool, error) {
	if md5 == "" {
		debug.PrintStack()
		log.Log.Fatalf("Error md5 empy in check rest")
	}
	ctx := context.Background()
	c, err := api.NewClient(bitgartenUrl, &sec{})
	if err != nil {
		log.Log.Debugf("Error creating client: %v", err)
		return false, err
	}
	res, err := c.BrowseLocation(ctx, api.BrowseLocationParams{Path: filepath.Clean(bitgartenLocation) + "/" + md5})
	if err != nil {
		log.Log.Errorf("Browse location failed: %v", err)
		return false, err
	}
	switch res.(type) {
	case *api.BrowseLocationOK:
		return true, nil
	case *api.BrowseLocationNotFound:
		return false, nil
	default:
		log.Log.Errorf("Unknown ERROR: %v", err)
		log.Log.Errorf("RES  : %T %v", res, res)
	}
	return false, fmt.Errorf("ERROR WEB")
}

func DownloadToTitle(md5 string, title string) error {
	ctx := context.Background()
	c, err := api.NewClient(bitgartenUrl, &sec{})
	if err != nil {
		log.Log.Debugf("Error creating client: %v", err)
		return err
	}
	fmt.Println("Downloading", md5, "to", title)
	params := api.DownloadFileParams{Path: filepath.Clean(bitgartenLocation) + "/" + md5}
	d, err := c.DownloadFile(ctx, params)
	if err != nil {
		fmt.Println("Error downloading file:", err)
		return err
	}
	switch res := d.(type) {
	case *api.DownloadFileOK:
		f, err := os.Create(title)
		if err != nil {
			fmt.Println("Error creating file:", err)
			return err
		}
		defer f.Close()
		dst := bufio.NewWriter(f)
		defer dst.Flush()
		var n int64
		n, err = io.Copy(dst, res.Data)
		if err != nil {
			fmt.Println("Error copying file:", err)
			return err
		}
		fmt.Println("Downloaded bytes:", n)
	case *api.DownloadFileForbidden, *api.DownloadFileUnauthorized:
		fmt.Println("Error permission file:")
		return fmt.Errorf("Permission Error")
	default:
		fmt.Printf("Error downloading file: %T\n", d)
		return fmt.Errorf("Error download file type %T:", d)
	}
	return nil
}

func StoreRestClient(md5 string, media []byte) error {
	log.Log.Debugf("Store REST available binary %s of length %d", md5, len(media))
	ctx := context.Background()
	c, err := api.NewClient(bitgartenUrl, &sec{})
	if err != nil {
		fmt.Println("Error client", err)
		return err
	}
	buffer := bytes.NewBuffer(media)
	request := &api.UploadFileReq{UploadFile: http.MultipartFile{Name: md5, File: buffer, Size: int64(len(media))}}
	params := api.UploadFileParams{Path: filepath.Clean(bitgartenLocation) + "/" + md5}
	res, err := c.UploadFile(ctx, request, params)
	if err != nil {
		return err
	}
	switch res.(type) {
	case *api.StatusResponse:
		return nil
	default:
	}
	log.Log.Errorf("ERROR %s: %v", params.Path, err)
	log.Log.Errorf("RES  : %T %v", res, res)
	return fmt.Errorf("ERROR WEB")
}

type sec struct{}

func (sec *sec) BasicAuth(ctx context.Context, operationName string) (api.BasicAuth, error) {
	a := api.BasicAuth{Username: os.Getenv("BITGARTEN_USERNAME"), Password: os.Getenv("BITGARTEN_PASSWORD")}
	return a, nil
}
func (sec *sec) BearerAuth(ctx context.Context, operationName string) (api.BearerAuth, error) {
	a := api.BearerAuth{}
	return a, ogenerrors.ErrSkipClientSecurity

}
func (sec *sec) TokenCheck(ctx context.Context, operationName string) (api.TokenCheck, error) {
	a := api.TokenCheck{}
	return a, ogenerrors.ErrSkipClientSecurity
}
