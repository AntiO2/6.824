# [MIT6.824] Spanner: Google’s Globally-Distributed Database

Spanner的主要目标是管理在不同数据中心的复制数据。Spanner可以动态地控制配置，并且在不同的数据中心之前无感知地移动数据，实现资源平衡。



R/W读写事务通过2PC,2PL, Paxos Groups实现

R/O 只读事务在数据中心读。提供外部读写的一致性，当在某一个副本上读时，应当读到最新的数据。

## Organization

 

![image-20231009152010630](./8_Spanner%20Note.assets/image-20231009152010630.png)

Spanner的高层组织架构如图。

1. 假设有3个数据中心，A有数据分片a-m,并运行在一组paxos上复制给B和C。另一组分片n-z可能由另外一组paxos处理。或者是通过RAFT处理（类似于Lab3中的kvserver）。运行多组paxos可以提高并行性。
2. 因为“大多数”的规则，可以很轻松的处理某个数据中心过慢或者某个数据中心宕机的情况，因为大多数raft peer回复成功就行。
3. 优先访问*Replica close to clients*，这里的clients就是图上的S符号（server），因为对于spanner这个基础架构，访问它的client可能是更上层的service，比如课上提到的gmail。同时，gmail服务器可能和某个数据中心在同一个机房（或者同一个城市）里面，通过访问最近的replica，获取R/O事务的高性能。

## Challenges

- Read of local replica must yield fresh data.
  - But local replica may not reflect latest Paxos writes!
  - 读最近的replica,保证读到最新的写结果
- A transaction may involve multiple shards -> multiple Paxos groups.
  - 支持跨分片的事务，具有ACID语义
- Transactions that read multiple records must be serializable.
  - But local shards may reflect different subsets of committed transactions!

## R/W读写事务

读写事务使用之前提到过的2PL和2PC。

![image-20231009154427436](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20231009154427436.png)

还是之前的转账例子。

C生成一个TID(事务编号)，并发送read x给SA，read y给SB。

注意这里的SA和SB都是之前提到过的分片，并不是单一服务器，而是一组Paxos(或者raft)。C需要访问SA和SB的leader, 在leader服务器上维护锁表。如果在此期间leader故障，事务就abort了，因为锁的信息不会经过raft，而是在leader上单机存储，leader故障，锁信息丢失。

![image-20231009155139841](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20231009155139841.png)

之后，C在本地进行写操作，在完成后，把数据发送给TC服务器组 (transaction coordinator)。TC进行2PC。

## Read-only transactions

目标：在本地进行快速的只读事务。no lock, no 2PC, consistency。

- 保证事务的可串行化。

- 外部一致性：External Consistency
  - 如果T2在T1提交之后开始，那么T2**必须**看到T1的修改



### Snapshot Isolation

快照隔离。

为事务分配时间戳（TS）

对于R/W, 为Commit开始的时间分配TS

对于R/O, 为Start的时间分配TS.

然后按照TS顺序实行事务。

每个Replica保存多个键的值和对应的时间戳（多版本）

R/O的事务可能持续一段时间，但是都应当读取事务开始的时间戳。

![image-20231009165558642](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20231009165558642.png)

T2的Read y在现实时间中发生在T3 Wy之后，但是它应当读取T2开始时时间戳y的值，也即是T1 Write y的值。

为了解决在T2开始读取时，replica还没有看到T1写入的问题（可能因为延迟，或者丢包，网络分区之类的）采用了Safe Time机制。Replica必须看到T2开始之后的写入，这样就能保证看到应该读到的值。

也应当等待准备好但还没有提交的事务完成。



在分布式系统中，**必须保证所有时钟都是准确的**。

这对r/o很重要。

- 如果TS偏移地过大（比如本地时钟快了一个小时）， 由于safe time,需要等待很久才能进行读取（比如其他服务器时间正常，需要等待一个小时才能看到safe time）
- TS 过小。破坏外部一致性，比如慢了一个小时，可能读取到一个小时之前的值，而不会看到最新提交的值。



Spanner通过原子钟、周期性和全球时间同步。

![image-20231009172605514](https://antio2-1258695065.cos.ap-chengdu.myqcloud.com/img/blogimage-20231009172605514.png)

但是时钟不可能保证绝对精准，于是引入了时间戳间隔。

首先，向原子钟询问当前时间，然后加减一个可能的误差值，得到一个区间[earliest,latest],协议保证true time在这个区间内。

- Start Rule: 开始事务或者RW提交事务的时间戳选用latest
- Commit Wait 当提交事务的时间戳 小于 now.earliest才能进行提交。