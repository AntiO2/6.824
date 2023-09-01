package main

import (
	"fmt"
	"runtime"
)

type s struct {
	x int
	y int
}

func main() {
	fmt.Println("Hello Go")
	test := s{20, 54}
	fmt.Println(test)
	showOS()
}

func showOS() {
	os := runtime.GOOS
	fmt.Println("当前操作系统是:", os)
}
