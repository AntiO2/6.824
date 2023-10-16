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
)

type OpTypeT string

const (
	NewConfig       OpTypeT = "NewConfig"
	PutOperation            = "Put"
	GetOperation            = "Get"
	AppendOperation         = "Append"
	PullNewData             = "PullNewData"
)

type ShardStatus string

const (
	Invalid      ShardStatus = "Invalid"
	Serving                  = "Serving"
	Pulling                  = "Pulling"
	WaitPull                 = "WaitPull"
	ServingButGC             = "ServingButGC"
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
