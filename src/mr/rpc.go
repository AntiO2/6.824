package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import "os"
import "strconv"

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}
type RpcIdT int64 // RpcIdT 是通过时间戳生成的, 指示一个唯一的RpcId
type ReqArgs struct {
	ReqId     RpcIdT
	ReqOp     WorkType
	ReqTaskId TaskIdT
}

// ResArgs 是RPC的返回
// Response
type ResArgs struct {
	ResId      RpcIdT
	ResOp      WorkType
	ResTaskId  TaskIdT // 分配的任务编号
	ResContent string
	ReduceNumN int // 有n个reduce
	MapNumM    int // 有M个map任务
}
type WorkType int

// TaskIdT 是对任务的编号
type TaskIdT int

// 枚举工作类型
const (
	WorkNothing    WorkType = iota
	WorkReq                 // worker申请工作
	WorkMap                 // 分配worker进行map操作
	WorkReduce              // 分配worker进行reduce操作
	WorkDone                // [[unused]]master所有的工作完成
	WorkTerminate           // 工作中止
	WorkMapDone             // Worker完成了map操作
	WorkReduceDone          // Worker完成了reduce操作
)

// Rpc exports struct we need
type Rpc struct {
	Req ReqArgs
	Res ResArgs
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
