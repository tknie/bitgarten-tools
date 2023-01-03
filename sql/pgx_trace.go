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
