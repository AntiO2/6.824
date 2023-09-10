package main

//
// simple sequential MapReduce.
//
// go run mrsequential.go wc.so pg*.txt
//

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)
import "6.824/mr"
import "plugin"
import "os"
import "log"
import "sort"

// ByKey for sorting by key.
type ByKey []mr.KeyValue

// Len for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

func main() {
	if len(os.Args) < 3 {
		_, err := fmt.Fprintf(os.Stderr, "Usage: mrsequential xxx.so inputfiles...\n")
		if err != nil {
			return
		}
		os.Exit(1)
	}
	path, _ := os.Getwd()
	fmt.Println("Work Path is", path)
	dir, _ := ioutil.ReadDir("/home/anti/projects/6.824/src/main")
	fmt.Println(dir)
	mapf, reducef := loadPlugin(os.Args[1]) // 通过命令行参数，指定map和reduce函数。go run -race mrsequential.go wc.so pg*.txt 使用wc中的函数

	log.Println("Start Read file, arg num is", len(os.Args))
	//
	// read each input file,
	// pass it to Map,
	// accumulate the intermediate Map output.
	//
	var intermediate []mr.KeyValue
	for _, filename := range os.Args[2:] {
		log.Printf("%v\n", filename)
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			log.Fatalf("cannot read %v", filename)
		}
		err = file.Close()
		if err != nil {
			return
		}
		kva := mapf(filename, string(content))
		intermediate = append(intermediate, kva...)
	}

	//for _, filePattern := range os.Args[2:] {
	//	files, err := filepath.Glob(filePattern)
	//	if err != nil {
	//		return
	//	}
	//	for _, filename := range files[:] {
	//		file, err := os.Open(filename)
	//		if err != nil {
	//			log.Fatalf("cannot open %v", filename)
	//		}
	//		content, err := io.ReadAll(file)
	//		if err != nil {
	//			log.Fatalf("cannot read %v", filename)
	//		}
	//		err = file.Close()
	//		if err != nil {
	//			return
	//		}
	//		kva := mapf(filename, string(content))
	//		intermediate = append(intermediate, kva...)
	//	}
	//
	//}

	//
	// a big difference from real MapReduce is that all the
	// intermediate data is in one place, intermediate[],
	// rather than being partitioned into NxM buckets.
	//

	sort.Sort(ByKey(intermediate))
	{
		oname := "mr-tmp-0"
		ofile, err := os.Create(oname)
		if err != nil {
			log.Fatal("Can't Create Intermediate File: ", oname)
		}
		defer func(file *os.File, oname string) {
			err := file.Close()
			if err != nil {
				log.Fatal("Can't Close Intermediate File", oname)
			}
		}(ofile, oname)
		encoder := json.NewEncoder(ofile)
		for _, kv := range intermediate {
			err := encoder.Encode(kv)
			if err != nil {
				log.Fatal("Encode Error: ", err)
				return
			}
		}
	}
	oname := "mr-out-0"
	ofile, _ := os.Create(oname)

	//
	// call Reduce on each distinct key in intermediate[],
	// and print the result to mr-out-0.
	//
	i := 0
	log.Println("Total kv len: ", len(intermediate))
	cnt := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		var values []string
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		cnt++
		output := reducef(intermediate[i].Key, values)

		// this is the correct format for each line of Reduce output.

		_, fprintf := fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)
		if fprintf != nil {
			return
		}

		i = j
	}
	log.Println("Unique key count: ", cnt)
	ofile.Close()
}

// loadPlugin
// load the application Map and Reduce functions
// from a plugin file, e.g. ../mrapps/wc.so
func loadPlugin(filename string) (func(string, string) []mr.KeyValue, func(string, []string) string) {
	p, err := plugin.Open(filename)
	if err != nil {
		log.Fatalf("cannot load plugin %v err: %v", filename, err.Error())
	}
	xmapf, err := p.Lookup("Map")
	if err != nil {
		log.Fatalf("cannot find Map in %v", filename)
	}
	mapf := xmapf.(func(string, string) []mr.KeyValue)
	xreducef, err := p.Lookup("Reduce")
	if err != nil {
		log.Fatalf("cannot find Reduce in %v", filename)
	}
	reducef := xreducef.(func(string, []string) string)

	return mapf, reducef
}
