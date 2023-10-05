package kvraft

import (
	"bytes"
	"encoding/gob"
)

type KvDataBase struct {
	KvData map[string]string
}

func (kv *KvDataBase) Init(snapshot []byte) {
	kv.KvData = make(map[string]string)
	kv.InstallSnapshot(snapshot)
}
func (kv *KvDataBase) Get(key string) (value string, ok bool) {
	if value, ok := kv.KvData[key]; ok {
		return value, ok
	}
	return "", ok
}

func (kv *KvDataBase) Put(key string, value string) (newValue string) {
	kv.KvData[key] = value
	return value
}

func (kv *KvDataBase) Append(key string, arg string) (newValue string) {
	if value, ok := kv.KvData[key]; ok {
		newValue := value + arg
		kv.KvData[key] = newValue
		return newValue
	}
	kv.KvData[key] = arg
	return arg
}
func (kv *KvDataBase) GenSnapshot() []byte {
	w := new(bytes.Buffer)
	encode := gob.NewEncoder(w)
	err := encode.Encode(kv.KvData)
	if err != nil {
		return nil
	}
	return w.Bytes()
}
func (kv *KvDataBase) InstallSnapshot(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(data)
	encode := gob.NewDecoder(r)
	err := encode.Decode(&kv.KvData)
	if err != nil {
		panic(err.Error())
	}
}
