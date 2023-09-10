package mr

import (
	"log"
	"sync"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

type status int // 用于指示worker的状态

const (
	notStart status = iota
	running
	taskDone
)
const workMaxTime = 12 * time.Second

type Coordinator struct {
	// Your definitions here.
	nReduce          int // Reduce数量
	mMap             int // Map数量
	taskDone         bool
	reduceTaskStatus []status
	mapTaskStatus    []status
	// runningMap 是当前正在running的rpcId
	// 想一下这种情况：第一个worker没有在10秒内返回结果，于是master开始把同样的任务返回给了第二个worker,此时又过了几秒，比如两秒钟
	// 那么master如何判断是第二个worker完成了任务，还是第一个worker呢？
	runningMap    []RpcIdT
	runningReduce []RpcIdT
	mapTasks      chan TaskIdT // 待开始的map
	reduceTasks   chan TaskIdT // 待开始的reduce
	files         []string     // 要进行task的文件
	mapCnt        int          // 已完成的map数量
	reduceCnt     int          // 已完成的reduce数量
	latch         *sync.Cond
}

// Your code here -- RPC handlers for the worker to call.

// Example
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

// Appoint 用于worker请求一个任务
func (c *Coordinator) Appoint(request *ReqArgs, reply *ResArgs) error {
	reply.ResId = request.ReqId
	reply.MapNumM = c.mMap
	reply.ReduceNumN = c.nReduce

	c.latch.L.Lock()
	done := c.taskDone
	c.latch.L.Unlock()
	if done {
		reply.ResOp = WorkDone
		return nil
	}
	switch request.ReqOp {
	case WorkReq:
		{
			// 请求一个任务
			c.latch.L.Lock()
			if len(c.mapTasks) > 0 {
				// 如果map任务还没有完全分配 分配一个map worker
				taskId := <-c.mapTasks
				reply.ResTaskId = taskId
				reply.ResContent = c.files[taskId]
				reply.ResOp = WorkMap
				c.runningMap[taskId] = reply.ResId
				c.mapTaskStatus[taskId] = running
				c.latch.L.Unlock()
				go c.checkDone(WorkMap, reply.ResTaskId)
				log.Printf("Assign map \t%d to \t%d\n", reply.ResTaskId, reply.ResId)
				return nil
			}
			if c.mapCnt < c.mMap {
				// 如果map任务已经全部分配完了，但是还没有运行完成，还不能开始reduce
				// worker需要暂时等待一下
				reply.ResOp = WorkNothing
				c.latch.L.Unlock()
				log.Println("Map All assigned but not done")
				return nil
			}
			if len(c.reduceTasks) > 0 {
				// 已经确定完成了所有map，还没有分配完reduce
				taskId := <-c.reduceTasks
				reply.ResTaskId = taskId
				reply.ResOp = WorkReduce
				c.runningReduce[taskId] = reply.ResId
				c.reduceTaskStatus[taskId] = running
				c.latch.L.Unlock()
				go c.checkDone(WorkReduce, reply.ResTaskId)
				log.Printf("Assign reduce \t%d to \t%d\n", reply.ResTaskId, reply.ResId)
				return nil
			}
			// 如果分配完了所有的reduce,但是还没有done.worker需要等待
			reply.ResOp = WorkNothing
			log.Println("Reduce All assigned but not done")
			c.latch.L.Unlock()
			return nil
		}
	case WorkMapDone:
		{

			c.latch.L.Lock()
			defer c.latch.L.Unlock()
			if c.runningMap[request.ReqTaskId] != request.ReqId || c.mapTaskStatus[request.ReqTaskId] != running {
				// 说明该map已经被abort
				reply.ResOp = WorkTerminate
				return nil
			}
			log.Printf("Work Map \t%d done by \t%d\n", request.ReqTaskId, request.ReqId)
			c.mapTaskStatus[request.ReqTaskId] = taskDone
			c.mapCnt++
		}
	case WorkReduceDone:
		{
			c.latch.L.Lock()
			defer c.latch.L.Unlock()
			if c.runningReduce[request.ReqTaskId] != request.ReqId || c.reduceTaskStatus[request.ReqTaskId] != running {
				// 说明该map已经被abort
				reply.ResOp = WorkTerminate
				return nil
			}
			c.reduceTaskStatus[request.ReqTaskId] = taskDone
			c.reduceCnt++
			log.Printf("Work Reduce \t%d done by \t%d\n", request.ReqTaskId, request.ReqId)
			if c.reduceCnt == c.nReduce {
				c.taskDone = true
				reply.ResOp = WorkDone
			}
		}
	default:
		return nil
	}
	return nil
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	log.Println("Launching Server")
	e := rpc.Register(c)
	if e != nil {
		log.Fatal("register error:", e)
	}
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	_ = os.Remove(sockname)

	l, e := net.Listen("unix", sockname)
	go func(l net.Listener) {
		for {
			time.Sleep(5 * time.Second)
			if c.Done() {
				err := l.Close()
				if err != nil {
					log.Fatal("close error:", err)
				}
			}
		}
	}(l)

	if e != nil {
		log.Fatal("listen error:", e)
	}
	go func() {
		err := http.Serve(l, nil)
		if err != nil {
			log.Fatal("server error:", err)
		}
	}()
}

// Done main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	c.latch.L.Lock()
	defer c.latch.L.Unlock()
	// Your code here.
	return c.taskDone
}

// checkDone 检查任务是否完成
func (c *Coordinator) checkDone(workType WorkType, t TaskIdT) {
	time.Sleep(workMaxTime)
	c.latch.L.Lock()
	defer c.latch.L.Unlock()
	switch workType {
	case WorkMap:
		{
			if c.mapTaskStatus[t] != taskDone {
				c.mapTaskStatus[t] = notStart
				c.mapTasks <- t
			}
		}
	case WorkReduce:
		{
			if c.reduceTaskStatus[t] != taskDone {
				// 如果没有完成任务
				c.reduceTaskStatus[t] = notStart
				c.reduceTasks <- t
			}
		}
	default:
		log.Panicf("Try Check Invalid WorkType %v\n", workType)
	}

}

// MakeCoordinator create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	log.Println("Launching Master Factory")
	c := Coordinator{}
	c.nReduce = nReduce
	c.mMap = len(files) // 每个file对应一个map
	c.taskDone = false

	c.files = files

	c.mapTasks = make(chan TaskIdT, c.mMap)
	c.mapTaskStatus = make([]status, c.mMap)
	c.runningMap = make([]RpcIdT, c.mMap)
	c.reduceTaskStatus = make([]status, nReduce)
	c.reduceTasks = make(chan TaskIdT, nReduce)
	c.runningReduce = make([]RpcIdT, nReduce)
	c.latch = sync.NewCond(&sync.Mutex{})

	for i := 0; i < c.mMap; i++ {
		c.mapTasks <- TaskIdT(i)
		c.runningMap[i] = -1
		c.mapTaskStatus[i] = notStart
	}
	for i := 0; i < c.nReduce; i++ {
		c.reduceTasks <- TaskIdT(i)
		c.runningReduce[i] = -1
		c.reduceTaskStatus[i] = notStart
	}
	c.server()
	return &c
}
