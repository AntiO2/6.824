package shardctrler

//
// Shardctrler clerk.
//

import (
	"6.824/labrpc"
	"sync"
)
import "time"
import "crypto/rand"
import "math/big"

type Clerk struct {
	servers []*labrpc.ClientEnd
	// Your data here.
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
	// Your code here.
	ck.lastFoundLeader = 0
	ck.lastAppliedCommandId = 0
	ck.clientId = nrand()
	return ck
}

func (ck *Clerk) Query(num int) Config {
	commandId := ck.lastAppliedCommandId + 1
	args := &QueryArgs{}
	// Your code here.
	args.Num = num
	args.ClientId = ck.clientId
	args.CommandId = commandId
	serverId := ck.lastFoundLeader
	serverNum := len(ck.servers)
	for ; ; serverId = (serverId + 1) % serverNum {
		// try each known server.
		DPrintf("Client Send [%v] Query Op:[%v]", serverId, args)
		var reply QueryReply
		ok := ck.servers[serverId].Call("ShardCtrler.Query", args, &reply)
		if ok && reply.WrongLeader == false {
			DPrintf("Client[%d] Success Query:[%v] ValueL[%v]", ck.clientId, args.Num, reply.Config)
			ck.lastFoundLeader = serverId
			ck.lastAppliedCommandId = commandId
			return reply.Config
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ck *Clerk) Join(servers map[int][]string) {

	args := &JoinArgs{}
	// Your code here.
	args.Servers = servers
	commandId := ck.lastAppliedCommandId + 1
	args.ClientId = ck.clientId
	args.CommandId = commandId
	serverId := ck.lastFoundLeader
	serverNum := len(ck.servers)
	for ; ; serverId = (serverId + 1) % serverNum {
		// try each known server.
		DPrintf("Client Send [%v] Join Op:[%v]", serverId, args)
		var reply JoinReply
		ok := ck.servers[serverId].Call("ShardCtrler.Join", args, &reply)
		if ok && reply.WrongLeader == false {
			DPrintf("Client[%d] Success Join:[%v]", ck.clientId, args.Servers)
			ck.lastFoundLeader = serverId
			ck.lastAppliedCommandId = commandId
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ck *Clerk) Leave(gids []int) {
	args := &LeaveArgs{}
	// Your code here.
	args.GIDs = gids
	commandId := ck.lastAppliedCommandId + 1
	args.ClientId = ck.clientId
	args.CommandId = commandId
	serverId := ck.lastFoundLeader
	serverNum := len(ck.servers)
	for ; ; serverId = (serverId + 1) % serverNum {
		DPrintf("Client Send [%v] Leave Op:[%v]", serverId, args)
		var reply LeaveReply
		ok := ck.servers[serverId].Call("ShardCtrler.Leave", args, &reply)
		if ok && reply.WrongLeader == false {
			DPrintf("Client[%d] Success Leave:[%v]", ck.clientId, args.GIDs)
			ck.lastFoundLeader = serverId
			ck.lastAppliedCommandId = commandId
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ck *Clerk) Move(shard int, gid int) {
	args := &MoveArgs{}
	// Your code here.
	args.Shard = shard
	args.GID = gid

	commandId := ck.lastAppliedCommandId + 1
	args.ClientId = ck.clientId
	args.CommandId = commandId
	serverId := ck.lastFoundLeader
	serverNum := len(ck.servers)

	for ; ; serverId = (serverId + 1) % serverNum {
		// try each known server.
		DPrintf("Client Send [%v] Join Move: GID[%v] SHARD[%v]", serverId, args.GID, args.Shard)
		var reply MoveReply
		ok := ck.servers[serverId].Call("ShardCtrler.Move", args, &reply)
		if ok && reply.WrongLeader == false {
			DPrintf("Client[%d] Success Move GID[%v] SHARD[%v]", ck.clientId, args.GID, args.Shard)
			ck.lastFoundLeader = serverId
			ck.lastAppliedCommandId = commandId
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
