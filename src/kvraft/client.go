package kvraft

import (
	"6.824/labrpc"
	"sync"
)
import "crypto/rand"
import "math/big"

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	clientId             int64 // client的唯一id
	lastAppliedCommandId int   // 最后一次command的id
	lastFoundLeader      int   // 最后一次发现的leader
	mu                   sync.Mutex
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.lastFoundLeader = 0
	ck.lastAppliedCommandId = 0
	ck.clientId = nrand()

	return ck
}

// Get
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) Get(key string) string {
	// You will have to modify this function.
	commandId := ck.lastAppliedCommandId + 1
	args := GetArgs{
		Key:       key,
		ClientId:  ck.clientId,
		CommandId: commandId,
	}
	serverId := ck.lastFoundLeader
	serverNum := len(ck.servers)
	for ; ; serverId = (serverId + 1) % serverNum {
		var reply GetReply
		DPrintf("Client Send [%v] Get Op:[%v]", serverId, args)
		ok := ck.servers[serverId].Call("KVServer.Get", &args, &reply) // 调用Server的Get Handler
		if !ok || reply.Err == ErrWrongLeader || reply.Err == ErrTimeout {
			continue
		}
		// 如果响应成功。
		ck.lastFoundLeader = serverId
		ck.lastAppliedCommandId = commandId
		if reply.Err == ErrNoKey {
			return ""
		}
		return reply.Value
	}
}

// PutAppend shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	commandId := ck.lastAppliedCommandId + 1
	args := PutAppendArgs{
		Key:       key,
		Value:     value,
		Op:        op,
		ClientId:  ck.clientId,
		CommandId: commandId,
	}

	serverId := ck.lastFoundLeader
	serverNum := len(ck.servers)
	for ; ; serverId = (serverId + 1) % serverNum {
		var reply PutAppendReply
		DPrintf("Client Send [%v] PutAppend Op:[%v]", serverId, args)
		ok := ck.servers[serverId].Call("KVServer.PutAppend", &args, &reply)
		if !ok || reply.Err == ErrWrongLeader || reply.Err == ErrTimeout {
			continue
		}
		ck.lastFoundLeader = serverId
		ck.lastAppliedCommandId = commandId
		return
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, PutOperation)
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, AppendOperation)
}
