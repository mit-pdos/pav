// Auto-generated from spec "github.com/mit-pdos/pav/rpc/testdata/types/types.go"
// using compiler "github.com/mit-pdos/pav/rpc".
package rpc

import (
	"github.com/mit-pdos/pav/marshalutil"
	"github.com/tchajed/marshal"
)

func (o *args) encode() []byte {
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
func (o *args) decode(b0 []byte) ([]byte, errorTy) {
	var b = b0
	a1, b, err := marshalutil.ReadBool(b)
	if err {
		return nil, err
	}
	a2, b, err := marshalutil.ReadByte(b)
	if err {
		return nil, err
	}
	a3, b, err := marshalutil.ReadInt(b)
	if err {
		return nil, err
	}
	a4, b, err := marshalutil.ReadSlice1D(b)
	if err {
		return nil, err
	}
	a5, b, err := marshalutil.ReadBytes(b, 16)
	if err {
		return nil, err
	}
	a6, b, err := marshalutil.ReadSlice2D(b)
	if err {
		return nil, err
	}
	a7, b, err := marshalutil.ReadSlice3D(b)
	if err {
		return nil, err
	}
	o.a1 = a1
	o.a2 = a2
	o.a3 = a3
	o.a4 = a4
	o.a5 = a5
	o.a6 = a6
	o.a7 = a7
	return b, errNone
}