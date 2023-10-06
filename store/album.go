/*
* Copyright © 2018-2023 private, Darmstadt, Germany and/or its licensors
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

package store

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/tknie/log"
	"golang.org/x/net/html"
)

// Picture picture description
type Picture struct {
	Description string `adabas:"::PD"`
	Name        string `adabas:"::PN"`
	Md5         string `adabas:"::PM"`
	Interval    uint32 `adabas:"::PI"`
	MIMEType    string `adabas:"::MI"`
	Width       uint32 `adabas:"::WI"`
	Height      uint32 `adabas:"::HE"`
	Fill        string `adabas:"::PT"`
}

// Album album information
type Album struct {
	Index            uint64     `adabas:":isn"`
	path             string     `xml:"-" json:"-" adabas:":ignore"`
	fileName         string     `xml:"-" json:"-" adabas:":ignore"`
	file             *os.File   `xml:"-" json:"-" adabas:":ignore"`
	Directory        string     `adabas:"::DI"`
	Date             int64      `adabas:"::DT"`
	Key              string     `adabas:"::KY"`
	Title            string     `adabas:"::TI"`
	AlbumDescription string     `adabas:"::TD"`
	Thumbnail        string     `adabas:"::TH"`
	Pictures         []*Picture `adabas:"::ET"`
}

// AlbumName name of map for album
var AlbumName string

func createStringMd5(input string) string {
	h := md5.New()
	io.WriteString(h, input)
	return fmt.Sprintf("%X", h.Sum(nil))
}

func searchAttributeClass(attr []html.Attribute, cl string) string {
	for _, a := range attr {
		if a.Key == cl {
			return a.Val
		}
	}
	return ""
}

func getEntries(doc *html.Node, name string, cl string) ([]*html.Node, error) {
	b := make([]*html.Node, 0)
	var f func(*html.Node)
	f = func(n *html.Node) {
		clFound := true
		if n.Type == html.ElementNode && cl != "" {
			c := searchAttributeClass(n.Attr, "class")
			if c == "" {
				clFound = false
			} else {
				clFound = strings.Contains(c, cl)
			}
		}
		if n.Type == html.ElementNode && n.Data == name && clFound {
			b = append(b, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	if len(b) > 0 {
		return b, nil
	}
	return nil, errors.New("Missing <" + name + "> in the node tree")
}

func renderNode(n *html.Node) string {
	var buf bytes.Buffer
	w := io.Writer(&buf)
	html.Render(w, n)
	return buf.String()
}

func (album *Album) e(div *html.Node, interval string) error {
	var p Picture
	px := &PictureBinary{MetaData: &PictureMetadata{Fill: "1"}}
	i, err := strconv.Atoi(interval)
	if err != nil {
		return err
	}
	p.Interval = uint32(i)
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "div":
				c := searchAttributeClass(n.Attr, "class")
				switch {
				case strings.HasPrefix(c, "fill"):
					p.Fill = c
					s := searchAttributeClass(n.Attr, "style")
					p.Name = s
					var re = regexp.MustCompile(`(?ms)background.*'img/(.*)'.*`)
					x := re.FindStringSubmatch(p.Name)[1]
					p.Name = album.Directory + "/" + x
					px.FileName = album.path + "/img/" + x
					p.Md5 = createStringMd5(p.Name)
					px.MetaData.Md5 = p.Md5
					px.MetaData.PictureName = p.Name
					re = regexp.MustCompile(`(?m)([^/]*)/.*`)
					d := re.FindStringSubmatch(p.Name)[1]
					fmt.Printf("Directory: %s -> %s", p.Name, d)
					px.MetaData.Directory = p.Name
					px.MetaData.Title = x
					p.MIMEType = "image/jpeg"
					px.MetaData.MIMEType = p.MIMEType

				case strings.Contains(c, "carousel-caption"):
					p.Description = renderNode(n)
				}
			case n.Data == "source":
				s := searchAttributeClass(n.Attr, "src")
				var re = regexp.MustCompile(`(?m)img/(.*)`)
				r := re.FindStringSubmatch(s)
				x := r[1]
				p.Name = album.Directory + "/" + x
				px.MetaData.PictureName = p.Name
				px.MetaData.Title = x
				p.Md5 = createStringMd5(p.Name)
				px.MetaData.Md5 = p.Md5
				m := searchAttributeClass(n.Attr, "type")
				p.MIMEType = m
				px.FileName = album.path + "/img/" + x
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(div)
	var reDescription = regexp.MustCompile(`(?ms).*<div.*h2>(.*)</h2.*</div.*`)
	m := reDescription.FindStringSubmatch(p.Description)
	if len(m) > 1 {
		p.Description = m[1]
	}
	fmt.Println("Store picture", p.Name)
	px.StorePicture()

	// Set picture width and height
	p.Width = px.MetaData.Width
	p.Height = px.MetaData.Height
	album.Pictures = append(album.Pictures, &p)
	return nil
}

func (album *Album) readFile() error {
	log.Log.Debugf("Extract album information: %s", album.fileName)
	file, err := os.Open(album.fileName)
	if err != nil {
		return err
	}

	doc, derr := html.Parse(file)
	if derr != nil {
		fmt.Println("Error parse file", err)
		return err
	}
	bn, err := getEntries(doc, "title", "")
	if err != nil {
		fmt.Println("Error read body", err)
		return err
	}
	log.Log.Debugf("Result title : ", renderNode(bn[0]))
	var re = regexp.MustCompile(`(?ms)<title>(.*)</title>`)
	album.Title = re.FindStringSubmatch(renderNode(bn[0]))[1]
	if album.Title == "" {
		album.Title = album.Directory
	}
	album.Key = createStringMd5(album.Title)
	fmt.Println(album.Title, "->", album.Key)
	album.Pictures = make([]*Picture, 0)
	bn, err = getEntries(doc, "div", "item")
	if err != nil {
		fmt.Println("Error read body", err)
		return err
	}
	log.Log.Debugf("Result items : ", len(bn))
	for _, x := range bn {
		d := searchAttributeClass(x.Attr, "data-interval")
		album.e(x, d)
	}
	return nil
}

// EvaluateIndex evaluate index for file
func EvaluateIndex(fileName string) error {
	ke := strings.LastIndex(fileName, "/")
	key := fileName[:ke]
	path := key
	ke = strings.LastIndex(key, "/")
	key = key[ke+1:]
	log.Log.Debugf("Key: %s", key)
	fmt.Println("Path:", path)
	a := &Album{path: path, fileName: fileName, Directory: key}
	a.readFile()
	s := &Store{}
	if len(a.Pictures) > 0 {
		a.Thumbnail = a.Pictures[0].Md5
	}
	s.Store = append(s.Store, a)
	jsonAlbum, err := json.Marshal(s)
	if err != nil {
		return err
	}
	fmt.Println("Title:", a.Title)
	//fmt.Println(string(jsonAlbum), err)
	_, err = SendJSON(AlbumName, jsonAlbum)
	return err
}
