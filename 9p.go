package main

import (
	"github.com/docker/go-p9p"
	"golang.org/x/net/context"
	"fmt"
	"os"
	"flag"
	"log"
	"strings"
	"net"
)

var chatty bool
var noauth bool
var addr string
var aname string
var cmd string
var args []string


/* A program for connecting to 9p file servers and performing client ops */
func main() {
	usage := "usage: 9p [-Dn] [-a address] [-A aname] cmd args..."
	ctx := context.Background()

	log.SetOutput(os.Stderr)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
	}
	flag.BoolVar(&chatty, "D", false, "chatty")
	flag.BoolVar(&noauth, "n", false, "no auth")
	flag.StringVar(&addr, "a", "nil", "address")
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
	case "cd":
	default:
		log.Fatal("Error: Specify a valid operation to perform.")
	}

	// Dial to 9p server
	proto := "tcp"
	if strings.HasPrefix(addr, "unix!") {
		// Dialing into a unix namespace
		proto = "unix"
		addr = addr[5:]
	}

	conn, err := net.Dial(proto, addr)
	if err != nil {
		log.Fatal("Error: Dial failed with ", err)
	}

	session, err := p9p.NewSession(ctx, conn)
	if err != nil {
		log.Fatal("Error: 9p session failed with ", err)
	}

	fmt.Println(session.Version())
}

