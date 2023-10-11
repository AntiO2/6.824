# [MIT6.824] Distributed Transactions 笔记

> **Atomicity: **
>
> **All-or-Nothing **
>
> **and **
>
> **Before-or-After** 

## 主题

>  distributed transactions = concurrency control + atomic commit

- 在多个服务器上的分片大量数据记录，大量的客户端  
- 客户端应用程序操作通常涉及多次读取和写入     例如：
  - 银行转账：借记卡和贷记卡   
  -  vote：检查是否已经投票，记录投票，增加计数     
  - 在社交图中安装双向链接  
- 我们希望对应用程序编写者隐藏交错和失败 。这是一个传统的数据库问题

如果熟悉数据库的话，应该挺熟悉2PL、ACID这些概念的。

> distributed transactions have two big components:
>   concurrency control (to provide isolation/serializability)
>   atomic commit (to provide atomicity despite failure)



在单机上实现原子操作，但是在多个机器上进行原子操作是困难的。

## 可串行化

> What does serializable mean?
>   you execute some concurrent transactions, which yield results
>     	"results" means both output and changes in the DB
>   the results are serializable if:
>     	- there exists a serial execution order of the transaction that yields the same results as the actual execution
>     (serial means one at a time -- no parallel execution)
>     (this definition should remind you of linearizability)

可串行化定义：存在某种顺序，逐一执行事务能够获得相同的结果。

注意：可串行化可以重排事务的顺序，而可线性化必须按照操作的顺序获取一致的结果。所以这样来看，可串行化要比可线性化弱一些。

## 并发控制

- 悲观：

使用锁进行控制。

常见方式：2PL locking。

 严格锁：在事务开始就申请所有所需要的锁。

- 乐观：

optimistic(no locks)

abort if not serializable

## Two-phase Commit (2PC)



![image-20231008235236389](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20231008235236389.png)

假如服务器A有数据X，服务器B上有数据y.

>   TC sends put(), get(), &c RPCs to A, B
>     The modifications are tentative, only to be installed if commit.
>   TC gets to the end of the transaction.
>   TC sends PREPARE messages to A and B.
>   If A is willing to commit,
>     A responds YES.
>     then A is in "prepared" state.
>   otherwise, A responds NO.
>   Same for B.
>   If both A and B say YES, TC sends COMMIT messages to A and B.
>   If either A or B says NO, TC sends ABORT messages.
>   A/B commit if they get a COMMIT message from the TC.
>     I.e. they write tentative records to the real DB.
>     And release the transaction's locks on their records.
>   A/B acknowledge COMMIT message.

TC发送PUT操作给A和B时，A和B先写入log。当TC准备提交，先向A和B发送prepare信息，如果A和B都准备好了提交并返回给TC，TC就发送COMMIT信息，否则发送ABORT信息。A/B收到COMMIT消息，将数据写入数据库，并释放持有的锁。



总之，还是使用WAL保证了可以正确地恢复。



1. 如果B准备好了提交（回应commit给prepare RPC），但是在之后崩溃了。 此时TC可能都收到了A和B的提交消息，并且发送commit给A和B，A进行了安装。所以B在恢复后必须进行commit。（通过TC重试） **if B voted YES, it must "block": wait for TC decision.**

2. 在COMMIT之后协调器崩溃。将决定的COMMIT ID也写入稳定存储。
3. 可以通过raft来做协调者容错。



对比：

| 协议         | RAFT                                                         | 2PC                                              |
| ------------ | ------------------------------------------------------------ | ------------------------------------------------ |
| 如何确保提交 | 多数获得log                                                  | 所有的数据分片服务器向协调器回复prepare yes信息  |
| 工作的目标   | 所有的raft peer复制相同的logs                                | 不同的服务器上有不同的数据                       |
| 不足         | RAFT不能确保所有服务器都在干事。（因为大多数在做相同的事就行） | 不保证可用性。因为需要确保所有的服务器都做完了事 |
