package kvraft

import (
	"6.824/labgob"
	"6.824/labrpc"
	"6.824/raft"
	"bytes"
	"encoding/gob"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	OpType    string
	Key       string
	Value     string
	ClientId  int64
	CommandId int
}
type ApplyResult struct {
	ErrMsg Err
	Value  string
	Term   int
}
type CommandContext struct {
	CommandId int
	Reply     ApplyResult
}

type KVServer struct {
	mu sync.Mutex
	me int
	rf *raft.Raft

	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	clientReply      map[int64]CommandContext
	replyChMap       map[int]chan ApplyResult // log的index对应的返回信息。
	lastAppliedIndex int                      // kv数据库中最后应用的index.
	kvdb             KvDataBase
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	kv.mu.Lock()
	if clientReply, ok := kv.clientReply[args.ClientId]; ok {
		if args.CommandId == clientReply.CommandId {
			reply.Value = clientReply.Reply.Value
			reply.Err = clientReply.Reply.ErrMsg
			kv.mu.Unlock()
			return
		} else if args.CommandId < clientReply.CommandId {
			reply.Err = ErrOutDate // 已经没有了之前执行的记录。
			kv.mu.Unlock()
			return
		}
	}
	kv.mu.Unlock()
	op := Op{
		OpType:    GetOperation,
		Key:       args.Key,
		Value:     "",
		CommandId: args.CommandId,
		ClientId:  args.ClientId,
	}

	index, term, isLeader := kv.rf.Start(op)

	if !isLeader {
		reply.Err = ErrWrongLeader
		return
	}
	DPrintf("Leader[%d] Receive Op\t:%v\tIndex[%d]", kv.me, op, index)
	replyCh := make(chan ApplyResult, 1)
	kv.mu.Lock()
	kv.replyChMap[index] = replyCh
	kv.mu.Unlock()
	select {
	case replyMsg := <-replyCh:
		{
			if term != replyMsg.Term {
				// 已经进入之后的term，leader改变（当前server可能仍然是leader，但是已经是几个term之后了）
				reply.Err = ErrWrongLeader
				return
			} else {
				reply.Value = replyMsg.Value
			}
		}
	case <-time.After(KVTimeOut):
		{
			reply.Err = ErrTimeout
			return
		}
	}
	go kv.CloseIndexCh(index)
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	DPrintf("Server [%d] Receive Request\t:%v", kv.me, args)
	kv.mu.Lock()
	if clientReply, ok := kv.clientReply[args.ClientId]; ok {
		if args.CommandId == clientReply.CommandId {
			DPrintf("Server [%d] Receive Same Command [%d]", kv.me, args.CommandId)
			kv.mu.Unlock()
			return
		} else if args.CommandId < clientReply.CommandId {
			DPrintf("Server  [%d] Receive OutDate Command [%d]", kv.me, args.CommandId)
			reply.Err = ErrOutDate // 已经没有了之前执行的记录。
			kv.mu.Unlock()
			return
		}
	}
	kv.mu.Unlock()
	op := Op{
		OpType:    args.Op,
		Key:       args.Key,
		Value:     args.Value,
		CommandId: args.CommandId,
		ClientId:  args.ClientId,
	}

	index, term, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.Err = ErrWrongLeader
		DPrintf("Client[%d] Isn't Leader", kv.me)
		return
	}
	DPrintf("Leader[%d] Receive Op\t:%v\tIndex[%d]", kv.me, op, index)
	replyCh := make(chan ApplyResult, 1)
	kv.mu.Lock()
	kv.replyChMap[index] = replyCh
	kv.mu.Unlock()
	select {
	case replyMsg := <-replyCh:
		{
			if term != replyMsg.Term {
				// 已经进入之后的term，leader改变（当前server可能仍然是leader，但是已经是几个term之后了）
				reply.Err = ErrWrongLeader
			} else {
				reply.Err = replyMsg.ErrMsg
			}
		}
	case <-time.After(KVTimeOut):
		{
			reply.Err = ErrTimeout
			return
		}
	}
	go kv.CloseIndexCh(index)
}

// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)
	kv.replyChMap = make(map[int]chan ApplyResult)
	kv.clientReply = make(map[int64]CommandContext)
	// You may need initialization code here.

	kv.kvdb.Init()
	kv.ReadSnapshot(persister.ReadSnapshot())
	kv.lastAppliedIndex = int(kv.rf.GetLastIncludeIndex())
	go kv.handleApplyMsg()
	go kv.makeSnapshot()
	return kv
}
func (kv *KVServer) CloseIndexCh(index int) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	indexCh, ok := kv.replyChMap[index]
	if !ok {
		return
	}
	close(indexCh)
	delete(kv.replyChMap, index)
	return
}

func (kv *KVServer) handleApplyMsg() {
	for !kv.killed() {
		applyMsg := <-kv.applyCh

		if applyMsg.CommandValid {
			kv.applyCommand(applyMsg)
			continue
		}
		if applyMsg.SnapshotValid {
			kv.applySnapshot(applyMsg)
			continue
		}
		_, err := DPrintf("Error Apply Msg: [%v]", applyMsg)
		if err != nil {
			return
		}
	}
}
func (kv *KVServer) applyCommand(msg raft.ApplyMsg) {
	var applyResult ApplyResult
	op := msg.Command.(Op)
	applyResult.Term = msg.CommandTerm
	kv.mu.Lock()
	defer kv.mu.Unlock()
	commandContext, ok := kv.clientReply[op.ClientId]
	if ok && commandContext.CommandId >= op.CommandId {
		// 该指令已经被应用过。
		// applyResult = commandContext.Reply
		return
	}
	switch op.OpType {
	case GetOperation:
		{
			value, ok := kv.kvdb.Get(op.Key)
			if ok {
				applyResult.Value = value
			} else {
				applyResult.Value = ""
				applyResult.ErrMsg = ErrNoKey
			}
		}
	case PutOperation:
		{
			value := kv.kvdb.Put(op.Key, op.Value)
			applyResult.Value = value
		}
	case AppendOperation:
		{
			value := kv.kvdb.Append(op.Key, op.Value)
			applyResult.Value = value
		}
	default:
		DPrintf("Error Op Type %s", op.OpType)
	}
	kv.lastAppliedIndex = msg.CommandIndex
	ch, ok := kv.replyChMap[msg.CommandIndex]
	if ok {
		ch <- applyResult
	}
	kv.clientReply[op.ClientId] = CommandContext{
		CommandId: op.CommandId,
		Reply:     applyResult,
	}
}

func (kv *KVServer) applySnapshot(msg raft.ApplyMsg) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if kv.rf.CondInstallSnapshot(msg.SnapshotTerm, msg.SnapshotIndex, msg.Snapshot) {
		kv.lastAppliedIndex = msg.SnapshotIndex
		kv.ReadSnapshot(msg.Snapshot)
	}
}

func (kv *KVServer) makeSnapshot() {
	for !kv.killed() {
		time.Sleep(time.Millisecond * 100)
		kv.mu.Lock()
		sizeNow := kv.rf.GetRaftStateSize()
		// DPrintf("Server[%d] Raft Size Is [%d] LastIncludeIndex [%d]", kv.me, sizeNow, kv.rf.GetLastIncludeIndex())
		if sizeNow > kv.maxraftstate && kv.maxraftstate != -1 {
			DPrintf("Server[%d] Start to Make Snapshot", kv.me)
			kv.rf.Snapshot(kv.lastAppliedIndex, kv.GenSnapshot())
			DPrintf("Server[%d] Raft Size Is [%d] After Snapshot", kv.me, kv.rf.GetRaftStateSize())
		}
		kv.mu.Unlock()
	}
}

func (kv *KVServer) GenSnapshot() []byte {
	w := new(bytes.Buffer)
	encode := gob.NewEncoder(w)
	if encode.Encode(kv.clientReply) != nil ||
		encode.Encode(kv.kvdb.KvData) != nil {
		panic("Can't Generate Snapshot")
		return nil
	}
	return w.Bytes()
}
func (kv *KVServer) ReadSnapshot(snapshot []byte) {
	if snapshot == nil || len(snapshot) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(snapshot)
	encode := gob.NewDecoder(r)
	if encode.Decode(&kv.clientReply) != nil ||
		encode.Decode(&kv.kvdb.KvData) != nil {
		panic("Can't Read Snapshot")
	}
}
