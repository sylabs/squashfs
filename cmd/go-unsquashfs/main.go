package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sylabs/squashfs"
)

func main() {
	verbose := flag.Bool("v", false, "Verbose")
	ignore := flag.Bool("ip", false, "Ignore Permissions and extract all files/folders with 0755")
	flag.Parse()
	if len(flag.Args()) < 2 {
		fmt.Println("Please provide a file name and extraction path")
		os.Exit(0)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	r, err := squashfs.NewReader(f)
	if err != nil {
		panic(err)
	}
	op := squashfs.DefaultOptions()
	op.Verbose = *verbose
	op.IgnorePerm = *ignore
	n := time.Now()
	err = r.ExtractWithOptions(flag.Arg(1), op)
	if err != nil {
		panic(err)
	}
	fmt.Println("Took:", time.Since(n))
}
