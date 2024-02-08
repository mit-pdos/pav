package shared

import (
	"bytes"
	"github.com/tchajed/marshal"
)

type ErrorT = uint64

const (
	// Errors
	ErrNone                  ErrorT = 0
	ErrSome                  ErrorT = 1
	ErrKeyCli_AuditPrefix    ErrorT = 2
	ErrKeyCli_CheckLogPrefix ErrorT = 3
	ErrKeyCli_CheckLogLookup ErrorT = 4
	ErrKeyCli_RegPrefix      ErrorT = 5
	ErrAud_DoPrefix          ErrorT = 6
	ErrKeyCli_RegNoExist     ErrorT = 7
	ErrUnameKey_Decode       ErrorT = 8
	ErrKeyLog_Decode         ErrorT = 9
	// RPCs
	RpcAppendLog uint64 = 1
	RpcGetLog    uint64 = 2
	// Sig
	SigLen uint64 = 69
)

type UnameKey struct {
	Uname uint64
	Key   []byte
}

func (uk *UnameKey) DeepCopy() *UnameKey {
	newKey := make([]byte, len(uk.Key))
	copy(newKey, uk.Key)
	return &UnameKey{Uname: uk.Uname, Key: newKey}
}

func (uk1 *UnameKey) IsEqual(uk2 *UnameKey) bool {
	return uk1.Uname == uk2.Uname && bytes.Equal(uk1.Key, uk2.Key)
}

func (uk *UnameKey) Encode() []byte {
	var b = make([]byte, 0)
	b = marshal.WriteInt(b, uk.Uname)
	b = marshal.WriteInt(b, uint64(len(uk.Key)))
	b = marshal.WriteBytes(b, uk.Key)
	return b
}

func (uk *UnameKey) Decode(b []byte) ([]byte, ErrorT) {
	if len(b) < 8 {
		return nil, ErrUnameKey_Decode
	}
	uname, b := marshal.ReadInt(b)
	if len(b) < 8 {
		return nil, ErrUnameKey_Decode
	}
	l, b := marshal.ReadInt(b)
	if uint64(len(b)) < l {
		return nil, ErrUnameKey_Decode
	}
	key, b := marshal.ReadBytes(b, l)
	uk.Uname = uname
	uk.Key = key
	return b, ErrNone
}

type KeyLog struct {
	log []*UnameKey
}

func NewKeyLog() *KeyLog {
	return &KeyLog{log: make([]*UnameKey, 0)}
}

func (l *KeyLog) DeepCopy() *KeyLog {
	newLog := make([]*UnameKey, 0, len(l.log))
	for _, entry := range l.log {
		newLog = append(newLog, entry.DeepCopy())
	}
	return &KeyLog{log: newLog}
}

func (small *KeyLog) IsPrefix(big *KeyLog) bool {
	if len(big.log) < len(small.log) {
		return false
	}
	ans := true
	for i := 0; i < len(small.log); i++ {
		if !small.log[i].IsEqual(big.log[i]) {
			ans = false
		}
	}
	return ans
}

func (l *KeyLog) Lookup(uname uint64) (uint64, []byte, bool) {
	var idx uint64
	var key []byte
	var ok bool
	for i := l.Len() - 1; i >= 0; i-- {
		if !ok && l.log[i].Uname == uname {
			idx = uint64(i)
			key = l.log[i].Key
			ok = true
		}
	}
	return idx, key, ok
}

func (l *KeyLog) Len() int {
	return len(l.log)
}

func (l *KeyLog) Append(uk *UnameKey) {
	l.log = append(l.log, uk)
}

func (l *KeyLog) Encode() []byte {
	var b = make([]byte, 0)
	b = marshal.WriteInt(b, uint64(l.Len()))
	for i := 0; i < l.Len(); i++ {
		b = marshal.WriteBytes(b, l.log[i].Encode())
	}
	return b
}

func (l *KeyLog) Decode(b []byte) ([]byte, ErrorT) {
	if len(b) < 8 {
		return nil, ErrKeyLog_Decode
	}
	length, b := marshal.ReadInt(b)
	log := make([]*UnameKey, length)
	var err ErrorT
	for i := uint64(0); i < length; i++ {
		log[i] = new(UnameKey)
		var err2 ErrorT
		b, err2 = log[i].Decode(b)
		if err2 != ErrNone {
			err = err2
		}
	}
	l.log = log
	return b, err
}
