## 0x1 Reading Paper

通过[【译文】Raft协议：In Search of an Understandable Consensus Algorithm (Extended Version) 大名鼎鼎的分布式共识算法 - 知乎 (zhihu.com)](https://zhuanlan.zhihu.com/p/524885008)阅读完了论文。

Raft协议感觉目标很简单：保证分布式系统的一致性和可用性，在阅读时，我联想到之前看的ARIES论文，感觉思维有很多共通之处，比如如何通过非易失性存储来保证持久性。但是ARIES中是单个机器崩溃导致内存内容丢失，通过硬盘上的LOGs来重做数据库，并且ABORT掉未提交的记录并写入CLR。Raft中，可能是多台机器崩溃，这个时候就要考虑在崩溃期间，其他机器增加log的操作了，因为集群不会因为少数几台机器崩溃而整体不可用。

在ARIES中，通过Commit类型的记录标识一个事务的提交，只要该commit log写入，无论是否落盘，就能告诉客户端：我已经将你的操作commit(持久化)了。而在RAFT里面，在当期TERM中，Majority的机器append了这条log,就能算做已经提交了。

## 0x11 一些疑问

但是在阅读时，有几个问题没有理解到：

1. 什么叫做提交？![](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blog20230911162249.png)

比如上图从c->e的 状态，term 4被提交是很好理解的，但是2也会被间接commit，客户端怎么知道2被commit了呢？

貌似答案是客户端并不在乎是否提交，或者是可以将Commit的日志发送给client。

一个方式是client可以重传失败的操作，并且给这个操作一个唯一的编号。比如在(e)的情况里面，2是被commit了。在(d)中，2没有commit。客户端在没有收到2被commit的消息超时之后，可以尝试重传2的操作。比如这个操作有一个唯一的序列号：23234，Raft在收到该序列号后，在(e)的情况下，检查到该序列已经被commit了，就不会再做这个操作。

详细的解释其实在guide中有->https://thesquareplanet.com/blog/students-guide-to-raft/#applying-client-operations

> One simple way to solve this problem is to record where in the Raft log the client’s operation appears when you insert it. Once the operation at that index is sent to `apply()`, you can tell whether or not the client’s operation succeeded based on whether the operation that came up for that index is in fact the one you put there. If it isn’t, a failure has happened and an error can be returned to the client.

比如client一开始连的是leader-A, leader-A需要在插入时记录客户端操作在Raft log中的哪里。当这个操作将要被apply的时候，通过判断该索引位置的操作是不是你放的（比如这个时候索引已经变成Leader-B强制Leader-A同步的了），如果操作来源不匹配，返回客户端一个错误。

2. 在选举时，如果有比较高term的follower拒绝投票，candidate是否立马退出选举？

参考了[raft选举策略 - 知乎 (zhihu.com)](https://zhuanlan.zhihu.com/p/306110683)。答案应该是如果candidate或者leader发现了存在term更高的节点，会立即掉入follower状态。

![img](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogv2-9396d411668ae75ab83379236cecaa39_r.jpg)

3. 如何保证term不重复？



## 0x12 一些细节

> Make sure you reset your election timer *exactly* when Figure 2 says you should. Specifically, you should *only* restart your election timer if a) you get an `AppendEntries` RPC from the *current* leader (i.e., if the term in the `AppendEntries` arguments is outdated, you should *not* reset your timer); b) you are starting an election; or c) you *grant* a vote to another peer.

重设超时时间：

1. 从当前leader接受信息
2. 投票给其他candidate
3. 开始选举

> - If you get an `AppendEntries` RPC with a `prevLogIndex` that points beyond the end of your log, you should handle it the same as if you did have that entry but the term did not match (i.e., reply false).

follower需要检查index是否匹配

> - If a leader sends out an `AppendEntries` RPC, and it is rejected, but *not because of log inconsistency* (this can only happen if our term has passed), then you should immediately step down, and *not* update `nextIndex`. If you do, you could race with the resetting of `nextIndex` if you are re-elected immediately.

如果leader被拒绝，立马下台（进入选举状态）

> From experience, we have found that by far the simplest thing to do is to first record the term in the reply (it may be higher than your current term), and then to compare the current term with the term you sent in your original RPC. If the two are different, drop the reply and return. *Only* if the two terms are the same should you continue processing the reply.

如果收到一个不同Term RPC的回复（**注意不是请求**），不要继续处理



## 0x2 阅读代码

还是按照BFS的方式阅读lab中Raft框架的源码

### 0x21 labrpc

labrpc模拟了可能丢包、延迟的网络环境。

> net := MakeNetwork() -- holds network, clients, servers. 创建网络
>
> net.AddServer(servername, server) -- adds a named server to network. 添加一个服务器
> net.DeleteServer(servername) -- eliminate the named server. 删除服务器
> net.Connect(endname, servername) -- connect a client to a server.
> net.Enable(endname, enabled) -- enable/disable a client.
> net.Reliable(bool) -- false means drop/delay messages
>
> end := net.MakeEnd(endname) -- create a client end-point, to talk to one server. 
> end.Call("Raft.AppendEntries", &args, &reply) -- send an RPC, wait for reply.
> the "Raft" is the name of the server struct to be called.

其中，Server和Service是两个概念

> srv := MakeServer()
> srv.AddService(svc) -- a server can have multiple services, e.g. Raft and k/v
>   pass srv to net.AddServer()
>
> svc := MakeService(receiverObject) -- obj's methods will handle RPCs
>   much like Go's rpcs.Register()
>   pass svc to srv.AddService()

感觉Service更多的指一组API接口，而Server类似一台主机，上面可能运行多个service

### 0x22 Raft

> A service calls `Make(peers,me,…)` to create a Raft peer. The peers argument is an array of network identifiers of the Raft peers (including this one), for use with RPC. The `me` argument is the index of this peer in the peers array. 

其中，通过阅读`Persister`代码，其中有序列化和反序列化方法。

## 基础数据结构



## Lab2a实现

目标：实现Leader选举。不需要携带log(空append entry)

![image-20230913222616047](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230913222616047.png)

```go
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	candidateTerm termT
	candidateId   int
	lastLogEntry  indexT
	lastLogTerm   termT
}

// RequestVoteReply example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	term termT
	voteGranted bool 
}
```

这里我不确定term和index的数据类型。感觉Index应该用uint（因为有效的index从1开始，可以用0表示无效的index）,但是给的代码中term和index都是int.

> Modify `Make()` to create a background goroutine that will kick off leader election periodically by sending out `RequestVote` RPCs when it hasn't heard from another peer for a while. This way a peer will learn who is the leader, if there is already a leader, or become the leader itself. Implement the `RequestVote()` RPC handler so that servers will vote for one another.

这里首先实现ticker里面的代码。

```C++
func (rf *Raft) ticker() {
    rf.setElectionTime()
    for rf.killed() == false {

       // Your code here to check if a leader election should
       // be started and to randomize sleeping time using
       // time.Sleep().
       time.Sleep(rf.heartBeat)
       rf.mu.Lock()
       if rf.status == leader {
          // 如果是leader状态,发送空包
       }
       if time.Now().After(rf.electionTime) {
          // 如果已经超时， 开始选举
       }
       rf.mu.Unlock()
    }
}
```

然后是实现candidate部分的代码：

参考5.2 Leader election中的描述一步一步做

> To begin an election, a follower increments its current
>
> term and transitions to candidate state.

```go
func (rf *Raft) startElection() {
    rf.setElectionTime()
    rf.status = candidate
    rf.currentTerm++
```

> It then votes for
>
> itself and issues RequestVote RPCs in parallel to each of
>
> the other servers in the cluster. 

```go
rf.votedFor = rf.me
rf.persist()
var convert sync.Once
for i := range rf.peers {
    if i != rf.me {
       var reply RequestVoteReply
       request := RequestVoteArgs{
          candidateTerm: rf.currentTerm,
          candidateId:   rf.me,
          lastLogEntry:  rf.getLastLogIndex(),
          lastLogTerm:   rf.getLastLogTerm(),
       }
       go rf.sendRequestVote(i, &request, &reply, &convert)
    }
}
```

继续往下阅读

> A candidate continues in
>
> this state until one of three things happens: 
>
> - (a) it wins theelection
> - (b) another server establishes itself as leader, or
>
> - (c) a period of time goes by with no winner.

其中，(c) 通过ticker函数解决超时未选举问题

(b) 在接受心跳信息中实现

(a) 在每次收到**有效的**选票后，统计自己是否赢得选举

首先实现a

```go
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply, convert *sync.Once, countVote *int) bool {
    ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
    if !ok {
       return ok
    }
    rf.mu.Lock()
    defer rf.mu.Unlock()
    if reply.term > rf.currentTerm {
       rf.setTerm(reply.term)
       rf.status = follower
       return false
    }
    if reply.term < rf.currentTerm {
       // 过期得rpc
       return false
    }
    if !reply.voteGranted {
       return false
    }
    *countVote++
    if *countVote >= rf.peerNum/2 &&
       rf.status == candidate &&
       rf.currentTerm == args.candidateTerm {
       // 投票成功，转为leader
       convert.Do(func() {
          /**
          nextIndex for each server, index of the next log entry to send to that server (initialized to leader last log index + 1)
          */
          nextIndex := rf.getLastLogIndex() + 1
          rf.status = leader
          for i := 0; i < rf.votedFor; i++ {
             rf.nextIndex[i] = nextIndex
             rf.matchIndex[i] = 0
          }
          rf.appendEntries(true)
       })
    }
    return ok
}
```

相对应的，实现投票规则。

- 如果在该term投了票，并且不是该candidateId, 则拒绝该次投票
- 如果term < currentTerm  拒绝投票
- 如果candidate的参数至少和当前follower一样新，则投票

```go
// RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.candidateTerm > rf.currentTerm {
		rf.setTerm(args.candidateTerm)
	}
	reply.term = rf.currentTerm
	if rf.currentTerm > args.candidateTerm {
		reply.voteGranted = false
		return
	}
	lastLogTerm := rf.getLastLogTerm()
	lastLogIndex := rf.getLastLogIndex()
	if (rf.votedFor == -1 || rf.votedFor == args.candidateId) &&
		(lastLogTerm < args.lastLogTerm || lastLogIndex <= args.lastLogEntry) {
		reply.voteGranted = true

		rf.votedFor = args.candidateId
		rf.persist()
		rf.setElectionTime()
	} else {
		reply.voteGranted = false
	}
}
```

接下来，实现发送心跳信息。

首先实现appendEntry需要的数据结构

```go
ype appendEntryArgs struct {
    term         termT  //leader’s term
    leaderId     int    // so follower can redirect clients
    prevLogIndex indexT //index of log entry immediately preceding new ones
    prevLogTerm  termT  // term of prevLogIndex entry
    entries      []Log  // log entries to store (empty for heartbeat; may send more than one for efficiency)
    leaderCommit indexT // leader’s commitIndex
}

type appendEntryReply struct {
    term    termT // currentTerm, for leader to update itself
    success bool  // true if follower contained entry matching prevLogIndex and prevLogTerm
}

func (rf *Raft) AppendEntriesRPC(args appendEntryArgs, reply appendEntryReply) {
    
}
```

实现leader发送心跳信息(在lab2A中，prevLog的参数都还不对)

```go
func (rf *Raft) appendEntries(heartBeat bool) {

    for i := range rf.peers {
       if i != rf.me {
          args := appendEntryArgs{
             term:         rf.currentTerm,
             leaderId:     rf.me,
             prevLogIndex: rf.getLastLogIndex(),
             prevLogTerm:  rf.getLastLogTerm(),
             entries:      nil,
             leaderCommit: rf.commitIndex,
          }
          if heartBeat {
             go func(rf *Raft, args *appendEntryArgs, peerId int) {
                client := rf.peers[peerId]
                var reply appendEntryReply
                client.Call("Raft.AppendEntriesRPC", args, &reply)
                rf.mu.Lock()
                defer rf.mu.Unlock()
                if reply.term > rf.currentTerm {
                   rf.setTerm(reply.term)
                   rf.setElectionTime()
                   rf.status = follower
                   return
                }
             }(rf, &args, i)
          }
       }
    }
}
```



最后根据Figure2 实现RPC Handler(不包含Log,仅心跳)

> Receiver implementation:
>
> 1. Reply false if term < currentTerm (§5.1)
> 2. Reply false if log doesn’t contain an entry at prevLogIndex
>
> whose term matches prevLogTerm (§5.3)
>
> 3. If an existing entry conflicts with a new one (same index
>
> but different terms), delete the existing entry and all that
>
> follow it (§5.3)
>
> 4. Append any new entries not already in the log
> 5. If leaderCommit > commitIndex, set commitIndex =
>
> min(leaderCommit, index of last new entry)
