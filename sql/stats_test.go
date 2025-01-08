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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixUnicodeforJson(t *testing.T) {
	msg := normalizeString("")
	assert.Equal(t, "", msg)
	msg = normalizeString(`/Users/tkn/Pictures/Bilder-SAG/mctkn02/bkgrnd/30832-gnome_in_love.jpg->loading EXIF sub-IFD: exif: sub-IFD ExifIFDPointer decode failed: zero length tag value
`)
	assert.Equal(t, "/Users/tkn/Pictures/Bilder-SAG/mctkn02/bkgrnd/30832-gnome_in_love.jpg->loading EXIF sub-IFD: exif: sub-IFD ExifIFDPointer decode failed: zero length tag value", msg)
	msg = normalizeString(`/Users/tkn/Pictures/182B13EB-4C15-406A-BE8C-DA00A4B7B078.heic->error reading 'ftyp' box: got box type '\x00\x84\x00\b' instead`)
	assert.Equal(t, "/Users/tkn/Pictures/182B13EB-4C15-406A-BE8C-DA00A4B7B078.heic->error reading 'ftyp' box: got box type '008400\\b' instead", msg)
}
