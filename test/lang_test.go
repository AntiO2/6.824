package test

import (
	"log"
	"testing"
)

func TestDefer(t *testing.T) {
	i := 1
	if i == 1 {
		log.Println("1")
		defer log.Println("2")
	}
	log.Println("3")
	{
		log.Println("4")
		defer log.Println("5")
	}
	defer log.Println("6")
}
