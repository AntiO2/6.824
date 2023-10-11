package shardctrler

import (
	"6.824/raft"
	"log"
	"sort"
	"time"
)
import "6.824/labrpc"
import "sync"
import "6.824/labgob"

const Debug = true
const scTimeOut = 500 * time.Millisecond

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type ApplyResult struct {
	WrongLeader bool
	Err         Err
	Cfg         Config
	Term        int
}
type CommandContext struct {
	CommandId int
	Reply     ApplyResult
}

type ShardCtrler struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	// Your data here.
	configs     []Config // indexed by config num
	clientReply map[int64]CommandContext
	replyChMap  map[int]chan ApplyResult // log的index对应的返回信息。
	ConfigIdNow int
	freeGid     []int // 空闲的gid
}

type Op struct {
	// Your data here.
	OpType    OpTypeT
	OpArgs    interface{}
	CommandId int
	ClientId  int64
}

func (sc *ShardCtrler) Join(args *JoinArgs, reply *JoinReply) {
	// Your code here.
	sc.mu.Lock()
	if clientReply, ok := sc.clientReply[args.ClientId]; ok {
		if clientReply.CommandId == args.CommandId {
			reply.WrongLeader = clientReply.Reply.WrongLeader
			reply.Err = clientReply.Reply.Err
			sc.mu.Unlock()
			return
		} else if args.CommandId < clientReply.CommandId {
			reply.Err = ErrOutDate // 已经没有了之前执行的记录。
			sc.mu.Unlock()
			return
		}
	}
	sc.mu.Unlock()
	op := Op{
		OpType:    OpJoin,
		OpArgs:    *args,
		ClientId:  args.ClientId, // 这里冗余信息
		CommandId: args.CommandId,
	}
	index, term, isLeader := sc.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}
	DPrintf("Leader[%d] Receive Op\t:%v\tIndex[%d]", sc.me, op, index)
	replyCh := make(chan ApplyResult, 1)
	sc.mu.Lock()
	sc.replyChMap[index] = replyCh
	sc.mu.Unlock()
	select {
	case replyMsg := <-replyCh:
		{
			if term != replyMsg.Term {
				reply.WrongLeader = true
			} else {
				reply.Err = replyMsg.Err
				reply.WrongLeader = false
			}
		}
	case <-time.After(scTimeOut):
		{
			reply.WrongLeader = true
		}
	}
	go sc.CloseIndexCh(index)
}

func (sc *ShardCtrler) Leave(args *LeaveArgs, reply *LeaveReply) {
	// Your code here.
	sc.mu.Lock()
	if clientReply, ok := sc.clientReply[args.ClientId]; ok {
		if clientReply.CommandId == args.CommandId {
			reply.WrongLeader = clientReply.Reply.WrongLeader
			reply.Err = clientReply.Reply.Err
			sc.mu.Unlock()
			return
		} else if args.CommandId < clientReply.CommandId {
			reply.Err = ErrOutDate // 已经没有了之前执行的记录。
			sc.mu.Unlock()
			return
		}
	}
	sc.mu.Unlock()
	op := Op{
		OpType:    OpLeave,
		OpArgs:    *args,
		ClientId:  args.ClientId, // 这里冗余信息
		CommandId: args.CommandId,
	}
	index, term, isLeader := sc.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}
	DPrintf("Leader[%d] Receive Op\t:%v\tIndex[%d]", sc.me, op, index)
	replyCh := make(chan ApplyResult, 1)
	sc.mu.Lock()
	sc.replyChMap[index] = replyCh
	sc.mu.Unlock()
	select {
	case replyMsg := <-replyCh:
		{
			if term != replyMsg.Term {
				reply.WrongLeader = true
			} else {
				reply.Err = replyMsg.Err
				reply.WrongLeader = false
			}
		}
	case <-time.After(scTimeOut):
		{
			reply.WrongLeader = true
		}
	}
	go sc.CloseIndexCh(index)
}

func (sc *ShardCtrler) Move(args *MoveArgs, reply *MoveReply) {
	// Your code here.
	sc.mu.Lock()
	if clientReply, ok := sc.clientReply[args.ClientId]; ok {
		if clientReply.CommandId == args.CommandId {
			reply.WrongLeader = clientReply.Reply.WrongLeader
			reply.Err = clientReply.Reply.Err
			sc.mu.Unlock()
			return
		} else if args.CommandId < clientReply.CommandId {
			reply.Err = ErrOutDate // 已经没有了之前执行的记录。
			sc.mu.Unlock()
			return
		}
	}
	sc.mu.Unlock()
	op := Op{
		OpType:    OpMove,
		OpArgs:    *args,
		ClientId:  args.ClientId, // 这里冗余信息
		CommandId: args.CommandId,
	}
	index, term, isLeader := sc.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}
	DPrintf("Leader[%d] Receive Op\t:%v\tIndex[%d]", sc.me, op, index)
	replyCh := make(chan ApplyResult, 1)
	sc.mu.Lock()
	sc.replyChMap[index] = replyCh
	sc.mu.Unlock()
	select {
	case replyMsg := <-replyCh:
		{
			if term != replyMsg.Term {
				reply.WrongLeader = true
			} else {
				reply.Err = replyMsg.Err
				reply.WrongLeader = false
			}
		}
	case <-time.After(scTimeOut):
		{
			reply.WrongLeader = true
		}
	}
	go sc.CloseIndexCh(index)
}

func (sc *ShardCtrler) Query(args *QueryArgs, reply *QueryReply) {
	// Your code here.
	sc.mu.Lock()
	if clientReply, ok := sc.clientReply[args.ClientId]; ok {
		if clientReply.CommandId == args.CommandId {
			reply.WrongLeader = clientReply.Reply.WrongLeader
			reply.Err = clientReply.Reply.Err
			sc.mu.Unlock()
			return
		} else if args.CommandId < clientReply.CommandId {
			reply.Err = ErrOutDate // 已经没有了之前执行的记录。
			sc.mu.Unlock()
			return
		}
	}
	sc.mu.Unlock()
	op := Op{
		OpType:    OpQuery,
		OpArgs:    *args,
		ClientId:  args.ClientId, // 这里冗余信息
		CommandId: args.CommandId,
	}
	index, term, isLeader := sc.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}
	DPrintf("Leader[%d] Receive Op\t:%v\tIndex[%d]", sc.me, op, index)
	replyCh := make(chan ApplyResult, 1)
	sc.mu.Lock()
	sc.replyChMap[index] = replyCh
	sc.mu.Unlock()
	select {
	case replyMsg := <-replyCh:
		{
			if term != replyMsg.Term {
				reply.WrongLeader = true
			} else {
				reply.Config = replyMsg.Cfg
				reply.Err = replyMsg.Err
				reply.WrongLeader = false
			}
		}
	case <-time.After(scTimeOut):
		{
			reply.WrongLeader = true
		}
	}
	go sc.CloseIndexCh(index)
}

// the tester calls Kill() when a ShardCtrler instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
func (sc *ShardCtrler) Kill() {
	sc.rf.Kill()
	// Your code here, if desired.
}

// needed by shardkv tester
func (sc *ShardCtrler) Raft() *raft.Raft {
	return sc.rf
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant shardctrler service.
// me is the index of the current server in servers[].
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister) *ShardCtrler {
	sc := new(ShardCtrler)
	sc.me = me

	sc.configs = make([]Config, 1)
	sc.configs[0].Groups = map[int][]string{}

	labgob.Register(Op{})
	labgob.Register(QueryArgs{})
	labgob.Register(LeaveArgs{})
	labgob.Register(JoinArgs{})
	labgob.Register(MoveArgs{})
	sc.applyCh = make(chan raft.ApplyMsg)
	sc.rf = raft.Make(servers, me, persister, sc.applyCh)

	// Your code here.
	sc.clientReply = make(map[int64]CommandContext)
	sc.replyChMap = make(map[int]chan ApplyResult)
	sc.freeGid = make([]int, 1)
	sc.freeGid[0] = 1
	go sc.handleApplyMsg()
	return sc
}

func (sc *ShardCtrler) handleApplyMsg() {
	for !sc.Killed() {
		applymsg := <-sc.applyCh
		if applymsg.CommandValid {
			sc.applyCommand(applymsg)
			continue
		}
		panic("Invalid Apply Msg")
	}
}

func (sc *ShardCtrler) Killed() bool {
	return false
}

func (sc *ShardCtrler) applyCommand(applymsg raft.ApplyMsg) {
	var applyResult ApplyResult
	op := applymsg.Command.(Op)
	applyResult.Term = applymsg.CommandTerm
	switch op.OpType {
	case OpJoin:
		{
			args := op.OpArgs.(JoinArgs)
			sc.applyJoin(&args, &applyResult)
		}
	case OpLeave:
		{
			args := op.OpArgs.(LeaveArgs)
			sc.applyLeave(&args, &applyResult)
		}
	case OpMove:
		{
			args := op.OpArgs.(MoveArgs)
			sc.applyMove(&args, &applyResult)
		}
	case OpQuery:
		{
			args := op.OpArgs.(QueryArgs)
			sc.applyQuery(&args, &applyResult)
		}
	default:
		DPrintf("Error Op Type %v", op.OpType)
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	ch, ok := sc.replyChMap[applymsg.CommandIndex]
	if ok {
		ch <- applyResult
	}
	sc.clientReply[op.ClientId] = CommandContext{
		CommandId: op.CommandId,
		Reply:     applyResult,
	}
}
func (sc *ShardCtrler) CloseIndexCh(index int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	indexCh, ok := sc.replyChMap[index]
	if !ok {
		return
	}
	close(indexCh)
	delete(sc.replyChMap, index)
	return
}

func (sc *ShardCtrler) applyJoin(args *JoinArgs, result *ApplyResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	newConfig := Config{}
	newConfig.Num = sc.ConfigIdNow + 1
	newConfig.Groups = make(map[int][]string)
	prevConfig := sc.configs[sc.ConfigIdNow]
	for key, value := range prevConfig.Groups {
		// copy map
		newConfig.Groups[key] = value
	}
	for i, gid := range prevConfig.Shards {
		// copy shard
		newConfig.Shards[i] = gid
	}
	for gid, servers := range args.Servers {
		newConfig.Groups[gid] = servers
	}
	sc.reBalanceShard(&newConfig)
	sc.ConfigIdNow = newConfig.Num
	sc.configs = append(sc.configs, newConfig)
	result.WrongLeader = false
	result.Err = ""
	DPrintf("[%d] Join Config:[%v]", sc.me, newConfig)
}

func (sc *ShardCtrler) getNextGid(config *Config) int {
	if len(sc.freeGid) == 0 {
		return len(config.Groups)
	} else {
		gid := sc.freeGid[0]
		sc.freeGid = sc.freeGid[1:]
		return gid
	}
}

/*
*
reBalanceShard 重新分配Shard给不同的GID
*/
func (sc *ShardCtrler) reBalanceShard(config *Config) {
	groupsNum := len(config.Groups) // 有效的group数量
	if groupsNum == 0 {
		return
	}
	if groupsNum == 1 {
		// 将所有的分片分配给该group
		for key, _ := range config.Groups {
			if key != 0 {
				for i := 0; i < 10; i++ {
					config.Shards[i] = key
				}
			}
			return
		}
	}
	sMin := NShards / groupsNum // 每个group最少的shard数量
	sMax := sMin                // 最多的shard数量，比如10个shard分配给3个group, 可以分配为 3 3 4
	if NShards%groupsNum > 0 {
		sMax++
	}
	type groupShard struct {
		gid      int
		shardNum int
	}
	assignShards := make(map[int][]int)   // 记录每个group初始有哪些shard
	invalidQueue := make([]groupShard, 0) // 即将leave或者无效的shard
	lowerQueue := make([]groupShard, 0)   // 记录过少的group
	upperQueue := make([]groupShard, 0)   // 记录过多的group
	maxBackupQueue := make([]int, 0)      // 记录刚好到达smax,备用的gid
	minBackupQueue := make([]int, 0)      // 记录刚好到达smin,备用的gid
	for key, _ := range config.Groups {
		if key != 0 {
			// 初始化有效的gid所含有的shard统计数组
			assignShards[key] = make([]int, 0)
		}
	}
	for shard, gid := range config.Shards {
		shardsList, ok := assignShards[gid]
		if ok {
			assignShards[gid] = append(shardsList, shard) // 统计该gid分配的shards
		} else {
			// 该gid无效（已经leave了）
			invalidQueue = append(invalidQueue, groupShard{
				gid:      gid,
				shardNum: shard,
			})
		}
	}
	assignGids := make([]int, 0)
	for gid := range assignShards {
		assignGids = append(assignGids, gid)
	}
	sort.Ints(assignGids)
	for _, gid := range assignGids {
		shards := assignShards[gid]
		l := len(shards)
		if l > sMax {
			upperQueue = append(upperQueue, groupShard{
				gid:      gid,
				shardNum: l,
			})
		}
		if l == sMax {
			maxBackupQueue = append(maxBackupQueue, gid)
		}
		if l == sMin {
			minBackupQueue = append(minBackupQueue, gid)
		}
		if l < sMin {
			lowerQueue = append(lowerQueue, groupShard{
				gid:      gid,
				shardNum: l,
			})
		}
	}
	for i, groupInfo := range lowerQueue {
		// 先保证满足smin(join情况)
		for lowerQueue[i].shardNum < sMin {
			if len(invalidQueue) > 0 {
				sc.moveShard(invalidQueue[0].gid, groupInfo.gid, invalidQueue[0].shardNum, config)
				invalidQueue = invalidQueue[1:]
				lowerQueue[i].shardNum++
				continue
			}
			if len(upperQueue) > 0 {
				sc.moveShard(upperQueue[0].gid, groupInfo.gid, assignShards[upperQueue[0].gid][0], config)
				assignShards[upperQueue[0].gid] = assignShards[upperQueue[0].gid][1:]
				upperQueue[0].shardNum--
				if upperQueue[0].shardNum == sMax {
					maxBackupQueue = append(maxBackupQueue, upperQueue[0].gid)
					upperQueue = upperQueue[1:]
				}
				lowerQueue[i].shardNum++
				continue
			}
			if len(maxBackupQueue) > 0 {
				// 从backup里面填
				sc.moveShard(maxBackupQueue[0], groupInfo.gid, assignShards[maxBackupQueue[0]][0], config)
				assignShards[maxBackupQueue[0]] = assignShards[maxBackupQueue[0]][1:]
				maxBackupQueue = maxBackupQueue[1:]
				lowerQueue[i].shardNum++
				continue
			}
			DPrintf("Cant Equally ReBalance")
			return
		}
		if lowerQueue[i].shardNum == sMin {
			minBackupQueue = append(minBackupQueue, groupInfo.gid)
		}
	}
	for _, invalidShard := range invalidQueue {
		if len(minBackupQueue) > 0 {
			sc.moveShard(invalidShard.gid, minBackupQueue[0], invalidShard.shardNum, config)
			minBackupQueue = minBackupQueue[1:]
		}
		log.Println("Can't assign invalidShard")
	}

}

/*
*
将shard从一个gid移动到另外一个gid
*/
func (sc *ShardCtrler) moveShard(gidBefore int, gidAfter int, shardNum int, config *Config) {
	config.Shards[shardNum] = gidAfter
}

func (sc *ShardCtrler) applyLeave(args *LeaveArgs, result *ApplyResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	newConfig := Config{}
	newConfig.Num = sc.ConfigIdNow + 1
	newConfig.Groups = make(map[int][]string)
	prevConfig := sc.configs[sc.ConfigIdNow]
	for key, value := range prevConfig.Groups {
		// copy map
		newConfig.Groups[key] = value
	}
	for i, gid := range prevConfig.Shards {
		// copy shard
		newConfig.Shards[i] = gid
	}
	for _, gid := range args.GIDs {
		delete(newConfig.Groups, gid)
	}
	sc.reBalanceShard(&newConfig)
	sc.ConfigIdNow = newConfig.Num
	sc.configs = append(sc.configs, newConfig)
	result.WrongLeader = false
	result.Err = ""
	DPrintf("[%d] Leave Config:[%v]", sc.me, newConfig)
}

func (sc *ShardCtrler) applyMove(args *MoveArgs, result *ApplyResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	newConfig := Config{}
	newConfig.Num = sc.ConfigIdNow + 1
	newConfig.Groups = make(map[int][]string)
	prevConfig := sc.configs[sc.ConfigIdNow]
	for key, value := range prevConfig.Groups {
		// copy map
		newConfig.Groups[key] = value
	}
	for i, gid := range prevConfig.Shards {
		// copy shard
		newConfig.Shards[i] = gid
	}
	sc.moveShard(prevConfig.Shards[args.Shard], args.GID, args.Shard, &newConfig)
	sc.ConfigIdNow = newConfig.Num
	sc.configs = append(sc.configs, newConfig)
	result.WrongLeader = false
	result.Err = ""
	DPrintf("[%d] Move Config:[%v]", sc.me, newConfig)
}

func (sc *ShardCtrler) applyQuery(args *QueryArgs, result *ApplyResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	queryNum := args.Num
	if queryNum == -1 || queryNum > sc.ConfigIdNow {
		queryNum = sc.ConfigIdNow
	}
	result.Cfg = sc.configs[queryNum]
	result.WrongLeader = false
	result.Err = ""
}
