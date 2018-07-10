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

var debug func(source, op, ...string)
var chatty bool
var noauth bool
var addr string
var aname string
var uname string
var msize int
var pversion string
var rfid p9p.Fid
var cmd string
var args []string


// Control for chattyPrint ← or →
type source int
const (
	server source = iota
	client
)

// Operation types for chattyPrint formatting
type op int
const (
	version op = iota
	auth
	Rerror
	flush
	attach
	walk
	open
	create
	read
)


// If chatty is enabled, print out 9p transactions. go-p9p does not provide this, sadly. 
func chattyPrint(s source, o op, extras ...string) {
	var msg string = "<nil>"
	arrow := '←'
	if s == client {
		arrow = '→'
	}

	switch o {
		case version:
			msg = "Rversion"
			if s == client {
				msg = "Tversion"
			}

			log.Printf("%c %s msize=%d version=%s\n", arrow, msg, msize, pversion)

		default:
			log.Println(arrow)
	}
}

/* We wrap all the p9p.Session functions to let us */
func Version(session p9p.Session) (int, string) {
	debug(client, version)
    msize, pversion = session.Version()
    debug(server, version)
	return msize, pversion
}


/* A program for connecting to 9p file servers and performing client ops. */
func main() {
	// Use 9p2000 by default, maybe extend this later if 9p2020 is finished
	pversion = "9p2000"
	debug = chattyPrint
	usage := "usage: 9p [-Dn] [-a address] [-A aname] [-u user] cmd args..."
	ctx := context.Background()

	log.SetOutput(os.Stderr)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
	}
	flag.BoolVar(&chatty, "D", false, "chatty")
	flag.BoolVar(&noauth, "n", false, "no auth")
	flag.StringVar(&addr, "a", "", "address")
	flag.StringVar(&aname, "A", "", "aname")
	flag.StringVar(&uname, "u", "none", "user")

	flag.Parse()
	cmd = flag.Arg(0)
	if cmd == "" {
		// No arguments outside of flags
		flag.Usage()
		log.Fatal("Error, Specify an operation to perform.")
	}
	args = flag.Args()[1:]
	if len(args) < 1 {
		// Require a path for all operations
		flag.Usage()
		log.Fatal("Error, Specify a path to apply the operation to.")
	}

	if !chatty {
		// No-op for debug if we don't want to be chatty
		debug = func(source, op, ...string) {}
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
		log.Fatal("Error, Dial failed with: ", err)
	}

	session, err := p9p.NewSession(ctx, conn)
	if err != nil {
		log.Fatal("Error, 9p session failed with: ", err)
	}

	debug(client, version)
	msize, pversion = session.Version()
	debug(server, version)

	// Attach root
	var fid p9p.Fid = 0
	rfid = fid

	rqid, err := session.Attach(ctx, fid, p9p.NOFID, uname, "/")
	if err != nil {
		log.Fatal("Error, Root Attach failed with: ", err)
	}
	fmt.Println("Root Qid: ", rqid)
	defer session.Clunk(ctx, fid)
	
	// Walk root so that we can clunk it later(?)
	fid++
	_, err = session.Walk(ctx, rfid, fid)
	if err != nil {
		log.Fatal("Error, Root Walk failed with: ", err)
	}
	defer session.Clunk(ctx, fid)
	
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
		log.Fatal("Error, Specify a valid operation to perform.")
	}

}

// List the files in a directory, takes flag arguments (wip)
/*func ls(ctx context.Context, fid p9p.Fid, args ...string) error {
	targetfid := c.nextfid
	c.nextfid++
	components := strings.Split(strings.Trim(p, "/"), "/")
	if _, err := c.session.Walk(ctx, c.rootfid, targetfid, components...); err != nil {
		return err
	}
	defer c.session.Clunk(ctx, targetfid)
	
	_, iounit, err := c.session.Open(ctx, targetfid, p9p.OREAD)
	if err != nil {
		return err
	}
	
	if iounit < 1 {
		msize, _ := c.session.Version()
		iounit = uint32(msize - 24) // size of message max minus fcall io header (Rread)
	}
	
	p := make([]byte, iounit)
	
	n, err := c.session.Read(ctx, targetfid, p, 0)
	if err != nil {
		return err
	}
	
	rd := bytes.NewReader(p[:n])
	codec := p9p.NewCodec() // TODO(stevvooe): Need way to resolve codec based on session.
	for {
		var d p9p.Dir
		if err := p9p.DecodeDir(codec, rd, &d); err != nil {
			if err == io.EOF {
				break
			}
	
			return err
		}
	
		fmt.Fprintf(wr, "%v\t%v\t%v\t%s\n", os.FileMode(d.Mode), d.Length, d.ModTime, d.Name)
	}
	
	if len(ps) > 1 {
		fmt.Fprintln(wr, "")
	}
}*/

