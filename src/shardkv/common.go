package shardkv

import (
	"log"
	"time"
)

// Sharded key/value server.
// Lots of replica groups, each running Raft.
// Shardctrler decides which group serves each shard.
// Shardctrler may change shard assignment from time to time.
//
// You will have to modify these definitions.

const Debug = true

func DPrintf(format string, a ...interface{}) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

const KVTimeOut = 500 * time.Millisecond
const (
	OK             Err = "OK"
	ErrNoKey           = "ErrNoKey"
	ErrWrongGroup      = "ErrWrongGroup"
	ErrWrongLeader     = "ErrWrongLeader"
	ErrOutDate         = "ErrOutDate"
	ErrNotReady        = "ErrNotReady"
)

type OpTypeT string

const (
	NewConfig       OpTypeT = "NewConfig"
	PutOperation    OpTypeT = "Put"
	GetOperation    OpTypeT = "Get"
	AppendOperation OpTypeT = "Append"
	PullNewData     OpTypeT = "PullNewData" // 拉取到了新分片
	ConfirmGC       OpTypeT = "ConfirmGC"   // GC发起者确定 被Pull的group收到了GC请求，可以从GC-SERVE状态转为SERVE
	ConfirmPull     OpTypeT = "ConfirmPull" // 被拉取的服务器确认拉取者已经拉取到，可以从Wait-pull状态转为INVALID状态
)

type ShardStatus int8

const (
	Invalid ShardStatus = iota
	Serving
	Pulling
	WaitPull
	ServingButGC
)

type Err string

// Put or Append
type PutAppendArgs struct {
	// You'll have to add definitions here.
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	ClientId  int64
	CommandId int
}

type PutAppendReply struct {
	Err Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.
	ClientId  int64
	CommandId int
}

type GetReply struct {
	Err   Err
	Value string
}
