/*
* Copyright © 2023-2026 private, Darmstadt, Germany and/or its licensors
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
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type x struct {
}

func (x *x) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	fmt.Printf("SQL     : %s\n", data.SQL)
	if len(data.Args) > 0 {
		fmt.Printf("SQL args:%#v\n", data.Args)
	}
	return ctx
}

func (x *x) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	fmt.Printf("Tag : %#v\n", data.CommandTag)
	fmt.Printf("Error : %v\n", data.Err)
}
