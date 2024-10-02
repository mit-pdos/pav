// Auto-generated from spec "github.com/mit-pdos/pav/rpc/testdata/types/types.go"
// using compiler "github.com/mit-pdos/pav/rpc".
package rpc

import (
	"github.com/mit-pdos/pav/marshalutil"
	"github.com/tchajed/marshal"
)

func (o *args) Encode() []byte {
	var b = make([]byte, 0)
	b = marshal.WriteBool(b, o.a1)
	b = marshalutil.WriteByte(b, o.a2)
	b = marshal.WriteInt(b, o.a3)
	b = marshalutil.WriteSlice1D(b, o.a4)
	b = marshal.WriteBytes(b, o.a5)
	b = marshalutil.WriteSlice2D(b, o.a6)
	b = marshalutil.WriteSlice3D(b, o.a7)
	return b
}
func (o *args) Decode(b0 []byte) ([]byte, bool) {
	a1, b1, err1 := marshalutil.ReadBool(b0)
	if err1 {
		return nil, true
	}
	a2, b2, err2 := marshalutil.ReadByte(b1)
	if err2 {
		return nil, true
	}
	a3, b3, err3 := marshalutil.ReadInt(b2)
	if err3 {
		return nil, true
	}
	a4, b4, err4 := marshalutil.ReadSlice1D(b3)
	if err4 {
		return nil, true
	}
	a5, b5, err5 := marshalutil.ReadBytes(b4, 16)
	if err5 {
		return nil, true
	}
	a6, b6, err6 := marshalutil.ReadSlice2D(b5)
	if err6 {
		return nil, true
	}
	a7, b7, err7 := marshalutil.ReadSlice3D(b6)
	if err7 {
		return nil, true
	}
	o.a1 = a1
	o.a2 = a2
	o.a3 = a3
	o.a4 = a4
	o.a5 = a5
	o.a6 = a6
	o.a7 = a7
	return b7, false
}
