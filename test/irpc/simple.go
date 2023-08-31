package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
)

type Args struct {
	A, B int
}
type Quotient struct {
	Quo, Rem int
}

type Arith int

func (t *Arith) Multiply(args *Args, reply *int) error {
	*reply = args.A * args.B
	return nil
}
func (t *Arith) Divide(args *Args, quo *Quotient) error {
	if args.B == 0 {
		return errors.New("divide by zero")
	}
	quo.Quo = args.A / args.B
	quo.Rem = args.A % args.B
	return nil
}

func main() {
	arith := new(Arith)
	err := rpc.Register(arith)
	if err != nil {
		return
	}
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", ":12345")
	if err != nil {
		return
	}
	go func() {
		err := http.Serve(l, nil)
		if err != nil {

		}
	}()
	serverAddress := "localhost"
	client, err := rpc.DialHTTP("tcp", serverAddress+":12345")
	if err != nil {
		log.Fatal("dialing:", err)
	}

	args := Args{7, 8}
	var reply int
	err = client.Call("Arith.Multiply", args, &reply)
	fmt.Printf("Arith: %d*%d=%d", args.A, args.B, reply)
}
