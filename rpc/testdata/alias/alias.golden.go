// Auto-generated from spec "github.com/mit-pdos/pav/rpc/testdata/alias/alias.go"
// using compiler "github.com/mit-pdos/pav/rpc".
package rpc

import (
	"github.com/mit-pdos/pav/marshalutil"
	"github.com/tchajed/marshal"
)

func (o *arg) encode() []byte {
	var b = make([]byte, 0)
	b = marshal.WriteInt(b, o.x)
	b = marshal.WriteInt(b, o.y)
	return b
}
func (o *arg) decode(b0 []byte) ([]byte, errorTy) {
	var b = b0
	x, b, err := marshalutil.ReadInt(b)
	if err {
		return nil, err
	}
	y, b, err := marshalutil.ReadInt(b)
	if err {
		return nil, err
	}
	o.x = x
	o.y = y
	return b, errNone
}