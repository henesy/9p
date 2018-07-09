package main

import (
//	"github.com/docker/go-p9p"
	"fmt"
	"os"
	"flag"
	"log"
)

var chatty bool
var noauth bool
var address string
var aname string
var cmd string
var args []string


/* A program for connecting to 9p file servers and performing client ops */
func main() {
	usage := "usage: 9p [-Dn] [-a address] [-A aname] cmd args..."

	log.SetOutput(os.Stderr)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
	}
	flag.BoolVar(&chatty, "D", false, "chatty")
	flag.BoolVar(&noauth, "n", false, "no auth")
	flag.StringVar(&address, "a", "nil", "address")
	flag.StringVar(&aname, "A", "nil", "aname")

	flag.Parse()
	cmd = flag.Arg(0)
	if cmd == "" {
		// No arguments outside of flags
		flag.Usage()
		log.Fatal("Error: Specify an operation to perform.")
	}
	args = flag.Args()[1:]
	if len(args) < 1 {
		// Require a path for all operations
		flag.Usage()
		log.Fatal("Error: Specify a path to apply the operation to.")
	}

	// Parse commands for the operation to perform
	switch cmd {
	case "read":
	case "readfd":
	case "write":
	case "writefd":
	case "stat":
	case "rdwr":
	case "ls":
	case "create":
	case "rm":
	case "open":
	case "openfd":
	default:
		flag.Usage()
		log.Fatal("Error: Specify a valid operation to perform.")
	}
}

