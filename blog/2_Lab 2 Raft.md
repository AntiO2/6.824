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
	if reply.Term > rf.currentTerm {
		rf.setTerm(reply.Term)
		// rf.setElectionTime()
		rf.status = follower
		return false
	}
	if reply.Term < rf.currentTerm {
		// 过期得rpc
		return false
	}
	if !reply.VoteGranted {
		return false
	}
	*countVote++
	if *countVote > rf.peerNum/2 &&
		rf.status == candidate &&
		rf.currentTerm == args.CandidateTerm {
		// 投票成功，转为leader
		convert.Do(func() {
			log.Printf("[%d] become leader in term [%d]", rf.me, rf.currentTerm)
			/**
			nextIndex for each server, Index of the next log entry to send to that server (initialized to leader last log Index + 1)
			*/
			nextIndex := rf.getLastLogIndex() + 1
			rf.status = leader
			for i := 0; i < rf.votedFor; i++ {
				rf.nextIndex[i] = nextIndex
				rf.matchIndex[i] = 0
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
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: rf.getLastLogIndex(),
				PrevLogTerm:  rf.getLastLogTerm(),
				Entries:      nil,
				LeaderCommit: rf.commitIndex,
			}
			if heartBeat {
				go func(rf *Raft, args *appendEntryArgs, peerId int) {
					client := rf.peers[peerId]
					var reply appendEntryReply
					client.Call("Raft.AppendEntriesRPC", args, &reply)
					rf.mu.Lock()
					defer rf.mu.Unlock()
					if reply.Term > rf.currentTerm {
						rf.setTerm(reply.Term)
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

```go
func (rf *Raft) AppendEntriesRPC(args *appendEntryArgs, reply *appendEntryReply) {
    rf.mu.Lock()
    defer rf.mu.Unlock()
    reply.Success = false
    reply.Term = rf.currentTerm
    if args.Term > rf.currentTerm {
       rf.status = follower
       rf.setElectionTime() // check(AntiO2)
       rf.setTerm(args.Term)
       return
    }
    if args.Term < rf.currentTerm {
       return
    }
    rf.setElectionTime()
    if rf.status == candidate {
       rf.status = follower
    }
    if args.Entries == nil || len(args.Entries) == 0 {
       // do heartBeat
       reply.Success = true
       return
    }
}
```

记得完成GetState()，用于检测状态。

Debug时遇到的坑：

- 无论是收到AppendEntry的RPC时，发现了更高的term，还是RPC回复中的参数发现了更高的term，都要立即转入follower状态。

- > - The tester requires that the leader send heartbeat RPCs no more than ten times per second.

  好像心跳时间一秒不能超过10次。当我一秒发送100次心跳，能维护只有一个Leader，但是每隔100ms就会出现多个Leader的情况。

比如在测试中出现的这种情况：s3在term8断开连接，在之后还自称leader.

![image-20230915164048988](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230915164048988.png)

![image-20230915164454972](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230915164454972.png)

再看提示：

> - The paper's Section 5.2 mentions election timeouts in the range of 150 to 300 milliseconds. Such a range only makes sense if the leader sends heartbeats considerably more often than once per 150 milliseconds. Because the tester limits you to 10 heartbeats per second, you will have to use an election timeout larger than the paper's 150 to 300 milliseconds, but not too large, because then you may fail to elect a leader within five seconds.

因为Leader每100ms发送一次超时信息，来不及在Follower进入超时选举状态之前更新，所以要增大超时选举的时间。（要求在超时时间中，发送more often than once） 这里我将超时时间设置成了300~500ms.

再仔细检查，发现忽略了一种情况：就是收到投票请求时，发现了更高的term，忘了转变为follower状态。



- 当我在Windows环境下，使用`-race`测试，会报错。上网看了下Issue好像amd会出现这个问题。
- 发现了没有收到多数票也能赢得选举的问题，然后才检查到判定多数派的选票数应该至少是`(peerNum+2)/2`

![image-20230915172619453](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230915172619453.png)



在测试时发现一次错误Log如下

```tex
2023/09/15 18:03:20 Disconnect : [2] [5] [3]
2023/09/15 18:03:21 [4] Time Out,Start Election Term [4]
2023/09/15 18:03:21 [3] Time Out,Start Election Term [4]
2023/09/15 18:03:21 [1] Time Out,Start Election Term [4]
2023/09/15 18:03:21 [0] Time Out,Start Election Term [4]
2023/09/15 18:03:21 [6] Vote To [4] In term [4]
2023/09/15 18:03:21 [2] Time Out,Start Election Term [4]
2023/09/15 18:03:21 [4] Time Out,Start Election Term [5]
2023/09/15 18:03:21 [1] Time Out,Start Election Term [5]
2023/09/15 18:03:21 [0] Time Out,Start Election Term [5]
2023/09/15 18:03:21 [6] Vote To [1] In term [5]
2023/09/15 18:03:21 [3] Time Out,Start Election Term [5]
2023/09/15 18:03:21 [2] Time Out,Start Election Term [5]
2023/09/15 18:03:22 [3] Time Out,Start Election Term [6]
2023/09/15 18:03:22 [0] Time Out,Start Election Term [6]
2023/09/15 18:03:22 [4] Time Out,Start Election Term [6]
2023/09/15 18:03:22 [1] Vote To [0] In term [6]
2023/09/15 18:03:22 [6] Vote To [0] In term [6]
2023/09/15 18:03:22 [2] Time Out,Start Election Term [6]
2023/09/15 18:03:22 [4] Time Out,Start Election Term [7]
2023/09/15 18:03:22 [6] Time Out,Start Election Term [7]
2023/09/15 18:03:22 [0] Time Out,Start Election Term [7]
2023/09/15 18:03:22 [1] Vote To [4] In term [7]
2023/09/15 18:03:22 [3] Time Out,Start Election Term [7]
2023/09/15 18:03:22 [2] Time Out,Start Election Term [7]
2023/09/15 18:03:22 [4] Time Out,Start Election Term [8]
2023/09/15 18:03:22 [1] Time Out,Start Election Term [8]
2023/09/15 18:03:22 [6] Vote To [1] In term [8]
2023/09/15 18:03:22 [0] Vote To [1] In term [8]
2023/09/15 18:03:22 [3] Time Out,Start Election Term [8]
2023/09/15 18:03:23 [2] Time Out,Start Election Term [8]
2023/09/15 18:03:23 [4] Time Out,Start Election Term [9]
2023/09/15 18:03:23 [0] Time Out,Start Election Term [9]
2023/09/15 18:03:23 [6] Vote To [0] In term [9]
2023/09/15 18:03:23 [1] Vote To [0] In term [9]
2023/09/15 18:03:23 [3] Time Out,Start Election Term [9]
2023/09/15 18:03:23 [0] Time Out,Start Election Term [10]
2023/09/15 18:03:23 [6] Time Out,Start Election Term [10]
2023/09/15 18:03:23 [2] Time Out,Start Election Term [9]
2023/09/15 18:03:23 [1] Vote To [0] In term [10]
2023/09/15 18:03:23 [4] Vote To [6] In term [10]
2023/09/15 18:03:23 [3] Time Out,Start Election Term [10]
2023/09/15 18:03:24 [0] Time Out,Start Election Term [11]
2023/09/15 18:03:24 [6] Time Out,Start Election Term [11]
2023/09/15 18:03:24 [4] Vote To [0] In term [11]
2023/09/15 18:03:24 [1] Vote To [0] In term [11]
2023/09/15 18:03:24 [2] Time Out,Start Election Term [10]
2023/09/15 18:03:24 [3] Time Out,Start Election Term [11]
2023/09/15 18:03:24 [0] Time Out,Start Election Term [12]
2023/09/15 18:03:24 [1] Time Out,Start Election Term [12]
2023/09/15 18:03:24 [6] Vote To [0] In term [12]
2023/09/15 18:03:24 [4] Vote To [0] In term [12]
2023/09/15 18:03:24 [2] Time Out,Start Election Term [11]
2023/09/15 18:03:24 [3] Time Out,Start Election Term [12]
2023/09/15 18:03:24 [1] Time Out,Start Election Term [13]
2023/09/15 18:03:24 [0] Time Out,Start Election Term [13]
2023/09/15 18:03:24 [6] Vote To [1] In term [13]
2023/09/15 18:03:24 [4] Vote To [1] In term [13]
2023/09/15 18:03:25 [2] Time Out,Start Election Term [12]
2023/09/15 18:03:25 [0] Time Out,Start Election Term [14]
2023/09/15 18:03:25 [3] Time Out,Start Election Term [13]
2023/09/15 18:03:25 [6] Time Out,Start Election Term [14]
2023/09/15 18:03:25 [4] Time Out,Start Election Term [14]
2023/09/15 18:03:25 [1] Vote To [6] In term [14]
2023/09/15 18:03:25 [2] Time Out,Start Election Term [13]
2023/09/15 18:03:25 [1] Time Out,Start Election Term [15]
2023/09/15 18:03:25 [0] Time Out,Start Election Term [15]
2023/09/15 18:03:25 [3] Time Out,Start Election Term [14]
2023/09/15 18:03:25 [6] Vote To [0] In term [15]
2023/09/15 18:03:25 [4] Vote To [1] In term [15]
2023/09/15 18:03:25 [2] Time Out,Start Election Term [14]
--- FAIL: TestManyElections2A (6.33s)
    config.go:398: expected one leader, got none
```

这里好像都成了candidate，然后没有在5s投票出来有效的leader。从Log看的出来，这里2、5、3都下线了，在集群里面只有0,1,4,6服务器在线。也就是说0，1，4，6中，必须有一个服务器得到4票（全票）才能当选。

比如出现了这种情况：

```go
2023/09/15 18:03:21 [1] Time Out,Start Election Term [5]
2023/09/15 18:03:21 [0] Time Out,Start Election Term [5]
2023/09/15 18:03:21 [6] Vote To [1] In term [5]
```

1和0几乎同时开始新的选举。在6投票给1之前，0也开始选举，就会出现分票的情况。

为了看的更清楚，将log的时间格式改为毫秒级别

```tex
2023/09/15 18:15:17 Disconnect : [6] [4] [5]
2023/09/15 18:15:17.497307 [5] Time Out,Start Election Term [8]
2023/09/15 18:15:17.497370 [3] Time Out,Start Election Term [8]
2023/09/15 18:15:17.497607 [1] Time Out,Start Election Term [8]
2023/09/15 18:15:17.498068 [0] Vote To [3] In term [8]
2023/09/15 18:15:17.498081 [2] Vote To [1] In term [8]
2023/09/15 18:15:17.900286 [3] Time Out,Start Election Term [9]
2023/09/15 18:15:17.900529 [5] Time Out,Start Election Term [9]
2023/09/15 18:15:17.900757 [2] Time Out,Start Election Term [9]
2023/09/15 18:15:17.900814 [0] Vote To [3] In term [9]
2023/09/15 18:15:17.900904 [1] Time Out,Start Election Term [9]
2023/09/15 18:15:18.302221 [2] Time Out,Start Election Term [10]
2023/09/15 18:15:18.302400 [0] Time Out,Start Election Term [10]
2023/09/15 18:15:18.302467 [3] Time Out,Start Election Term [10]
2023/09/15 18:15:18.302693 [1] Time Out,Start Election Term [10]
2023/09/15 18:15:18.403276 [5] Time Out,Start Election Term [10]
2023/09/15 18:15:18.703948 [3] Time Out,Start Election Term [11]
2023/09/15 18:15:18.704030 [1] Time Out,Start Election Term [11]
2023/09/15 18:15:18.704036 [0] Time Out,Start Election Term [11]
2023/09/15 18:15:18.704627 [2] Vote To [3] In term [11]
2023/09/15 18:15:18.904608 [5] Time Out,Start Election Term [11]
2023/09/15 18:15:19.206448 [3] Time Out,Start Election Term [12]
2023/09/15 18:15:19.206490 [0] Time Out,Start Election Term [12]
2023/09/15 18:15:19.206784 [2] Time Out,Start Election Term [12]
2023/09/15 18:15:19.207301 [1] Time Out,Start Election Term [12]
2023/09/15 18:15:19.306922 [5] Time Out,Start Election Term [12]
2023/09/15 18:15:19.608320 [0] Time Out,Start Election Term [13]
2023/09/15 18:15:19.608388 [3] Time Out,Start Election Term [13]
2023/09/15 18:15:19.608350 [1] Time Out,Start Election Term [13]
2023/09/15 18:15:19.611556 [2] Vote To [1] In term [13]
2023/09/15 18:15:19.708899 [5] Time Out,Start Election Term [13]
2023/09/15 18:15:20.010722 [0] Time Out,Start Election Term [14]
2023/09/15 18:15:20.011113 [3] Time Out,Start Election Term [14]
2023/09/15 18:15:20.011701 [1] Time Out,Start Election Term [14]
2023/09/15 18:15:20.011758 [2] Vote To [0] In term [14]
2023/09/15 18:15:20.111205 [5] Time Out,Start Election Term [14]
2023/09/15 18:15:20.413353 [1] Time Out,Start Election Term [15]
2023/09/15 18:15:20.413558 [2] Time Out,Start Election Term [15]
2023/09/15 18:15:20.414091 [3] Vote To [1] In term [15]
2023/09/15 18:15:20.414073 [0] Vote To [2] In term [15]
2023/09/15 18:15:20.514419 [5] Time Out,Start Election Term [15]
2023/09/15 18:15:20.816345 [1] Time Out,Start Election Term [16]
2023/09/15 18:15:20.816657 [3] Time Out,Start Election Term [16]
2023/09/15 18:15:20.816823 [0] Time Out,Start Election Term [16]
2023/09/15 18:15:20.819032 [2] Vote To [1] In term [16]
2023/09/15 18:15:21.017935 [5] Time Out,Start Election Term [16]
2023/09/15 18:15:21.218446 [1] Time Out,Start Election Term [17]
2023/09/15 18:15:21.218512 [0] Time Out,Start Election Term [17]
2023/09/15 18:15:21.218845 [2] Time Out,Start Election Term [17]
2023/09/15 18:15:21.219796 [3] Vote To [1] In term [17]
2023/09/15 18:15:21.419438 [5] Time Out,Start Election Term [17]
2023/09/15 18:15:21.620428 [0] Time Out,Start Election Term [18]
2023/09/15 18:15:21.620592 [2] Time Out,Start Election Term [18]
2023/09/15 18:15:21.620687 [3] Time Out,Start Election Term [18]
2023/09/15 18:15:21.622159 [1] Vote To [0] In term [18]
2023/09/15 18:15:21.821781 [5] Time Out,Start Election Term [18]
--- FAIL: TestManyElections2A (8.43s)
    config.go:398: expected one leader, got none
```

这次是0，1，2，3必须投票给同一个Server。

然后发现了一些不对的地方

```tex
2023/09/15 18:15:20.011701 [1] Time Out,Start Election Term [14]
2023/09/15 18:15:20.011758 [2] Vote To [0] In term [14]

2023/09/15 18:15:20.413353 [1] Time Out,Start Election Term [15]
2023/09/15 18:15:20.413558 [2] Time Out,Start Election Term [15]
```

可以看到，1的超时选举时间重设为了401.652ms,2的超时选举时间重设为了401.800ms,两者太相近了。只要这种情况发生，就会出现一直分票的情况。这大概是不够随机的原因，300-500ms的超时间隔太近了。然后我尝试将超时时间在$[300,900]ms$ ，解决了这个问题

最后，使用`go test -run TestManyElections2A -race -count 100 -timeout 1800s `，运行100次测试。

![image-20230915200039030](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20230915200039030.png)
