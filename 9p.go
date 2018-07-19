package main

import (
	"github.com/docker/go-p9p"
	"context"
	"fmt"
	"os"
	"flag"
	"log"
	"strings"
	"net"
sc	"strconv"
	"text/tabwriter"
	"container/list"
	"io"
	"bytes"
)

const timeFormat string = "01-02-2006 15:04:05 MST"
var debug func(source, op, ...string)
var chatty bool
var noauth bool
var addr string
var aname string
var uname string
var msize int
var pversion string
var rfid p9p.Fid
var nfid p9p.Fid
var cmd string
var args []string
var session p9p.Session
var ctx context.Context


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
	rerror
	flush
	attach
	walk
	open
	create
	read
	clunk
	stat
)

// Control for Read()
type mode int
const (
	nowrite mode = iota
	nolist
	nomode
)


// Returns the names in path (typically args)
func mknames(path string) []string {
	return strings.Split(strings.TrimSpace(strings.Trim(path, "/")), "/")
}

// Converts a p9p.Fid to a string and so forth
func f2s(fid p9p.Fid) string {
	return sc.Itoa(int(fid))
}

// If chatty is enabled, print out 9p transactions. go-p9p does not provide this, sadly. 
func chattyPrint(s source, o op, extras ...string) {
	var msg string = "<nil>"
	arrow := '←'
	if s == client {
		arrow = '→'
	}

	switch o {
		case version:
			if s == client {
				msg = "Tversion"
			}
			msg = "Rversion"
			log.Printf("%c %s msize=%d version=%s", arrow, msg, msize, pversion)

		case clunk:
			if s == client {
				msg = "Tclunk"
				log.Printf("%c %s fid=%s", arrow, msg, extras[0])
				break
			}
			msg = "Rclunk"
			log.Printf("%c %s", arrow, msg)

		case walk:
			if s == client {
				msg = "Twalk"
				log.Printf("%c %s fid=%s newfid=%s", arrow, msg, extras[0], extras[1])
				break
			}
			msg = "Rwalk"
			log.Printf("%c %s qids=%v", arrow, msg, extras)

		case attach:
			if s == client {
				msg = "Tattach"
				afid := extras[1]
				if afid == f2s(p9p.NOFID) {
					afid = "<nil>"
				}
				log.Printf("%c %s fid=%s afid=%s uname=\"%s\" aname=\"%s\"", arrow, msg, extras[0], afid, uname, aname)
				break
			}
			msg = "Rattach"
			log.Printf("%c %s qid=%s", arrow, msg, extras[0])

		case open:
			if s == client {
				msg = "Topen"
				log.Printf("%c %s fid=%s mode=%s", arrow, msg, extras[0], extras[1])
				break
			}
			msg = "Ropen"
			log.Printf("%c %s qid=%s iounit=%s", arrow, msg, extras[0], extras[1])

		case read:
			if s == client {
				msg = "Tread"
				log.Printf("%c %s fid=%s offset=%s iounit=%s", arrow, msg, extras[0], extras[1], extras[2])
				break
			}
			msg = "Rread"
			log.Printf("%c %s iounit=%s", arrow, msg, extras[0])

		case stat:
			if s == client {
				msg = "Tstat"
				log.Printf("%c %s fid=%s", arrow, msg, extras[0])
				break
			}
			msg = "Rstat"
			log.Printf("%c %s dir=%s", arrow, msg, extras[0])

		case rerror:
			msg = "Rerror"
			log.Printf("%c %s %s", arrow, msg, extras[0])

		default:
			log.Println(arrow)
	}
}

/* We wrap all the p9p.Session functions to let us */
func Version() (int, string) {
	debug(client, version)
    msize, pversion = session.Version()
    debug(server, version)
	return msize, pversion
}

func Clunk(fid p9p.Fid) (err error) {
	debug(client, clunk, f2s(fid))
	err = session.Clunk(ctx, fid)
	if err != nil {
		debug(server, rerror, err.Error())
		return
	}
	debug(server, clunk, f2s(fid))
	return
}

func Walk(fid, newfid p9p.Fid, names ...string) (nwqid []p9p.Qid, err error) {
	debug(client, walk, f2s(fid), f2s(newfid))
	nwqid, err = session.Walk(ctx, fid, newfid, names...)
	if err != nil {
		debug(server, rerror, err.Error())
		return
	}

	// This is the only place that qids need to be made into strings, so we use a closure.
	debug(server, walk,
		func() []string {
			qids := make([]string, 0, len(nwqid))
			for _, qid := range nwqid {
				qids = append(qids, qid.String())
			}
			return qids
		}()...)
	return
}

// We might want to pass in aname, but I'm not sure yet
func Attach(fid, afid p9p.Fid) (qid p9p.Qid, err error) {
	debug(client, attach, f2s(fid), f2s(afid))
	qid, err = session.Attach(ctx, fid, afid, uname, aname)
	if err != nil {
		debug(server, rerror, err.Error())
	}
	debug(server, attach, qid.String())
	return
}

// Open a file/dir
func Open(fid p9p.Fid, mode p9p.Flag) (qid p9p.Qid, iounit uint32, err error) {
	err = nil
	iounit = 0

	debug(client, open, f2s(fid), sc.Itoa(int(mode)))
	qid, iounit, err = session.Open(ctx, fid, mode)
	if err != nil {
		debug(server, rerror, err.Error())
		return
	}

	if iounit < 1 {
		// size of message max minus fcall io header (Rread)
		iounit = uint32(msize - 24)
	}

	debug(server, open, qid.String(), sc.Itoa(int(iounit)))

	return
}

// Read bytes from a file -- This needs an argument to not use the list and return []byte (in the case of streams)
func Read(m mode) ([]byte, error) {
	nfid++
	var fid p9p.Fid = nfid
	defer Clunk(fid)

	// Walk -- don't need []Qid's for now
	names := mknames(args[0])
	_, err := Walk(rfid, fid, names...)
	if err != nil {
		log.Fatal("Error, walk for open failed: ", err)
	}

	// Open -- don't need Qid for now
	_, width, err := Open(fid, p9p.OREAD)
	if err != nil {
		log.Fatal("Error, Open failed: ", err)
	}
	buf := make([]byte, width)
	// To return, we need to expand this dynamically
	bytelist := list.New()
	bytelist.Init()

	// Read -- might have to loop through msize-ish chunks using offsets (see: 9p.c in p9p)
	var offset int64 = 0
	// count in this fn is the sum of bytes read
	var count int = 0
	var n int = 1
	for ;; offset += int64(n) {
			buf = make([]byte, width)
			debug(client, read, f2s(fid), sc.Itoa(int(offset)), sc.Itoa(int(width)))
			n, err = session.Read(ctx, fid, buf, offset)
			//fmt.Fprintln(os.Stderr, "Read: ", n, err)
			count += n

			if n < 0 {
				log.Fatal("Error, read error: ", err)
			}
			if err != nil {
				debug(server, rerror, err.Error())
			} else {
				debug(server, read, sc.Itoa(n))
			}

			if n == 0 {
				break
			}
			
			if m != nolist {
				bytelist.PushBack(buf[:n])
			}
			
			if m != nowrite {
				// Output
				nout, err := os.Stdout.Write(buf[:n])
				if nout < 0 || err != nil {
					log.Fatal("Error, read output error: ", err)
				}
			}
	}

	var allbytes []byte
	if m != nolist {
		// Compose all bytes written to a single []byte to return (maybe make this optional for performance?)
		allbytes = make([]byte, 0, count)
		for bytelist.Front() != nil {
			bytes := bytelist.Remove(bytelist.Front()).([]byte)
			for _, b := range bytes {
				allbytes = append(allbytes, b)
			}
		}
	}
		
	return allbytes, nil
}

// Stat a file
func Stat(m mode) (info p9p.Dir, err error) {
	wr := tabwriter.NewWriter(os.Stdout, 0, 8, 8, ' ', 0)
	nfid++
	fid := nfid
	defer Clunk(fid)

	names := mknames(args[0])
	_, err = Walk(rfid, fid, names...)
	if err != nil {
		log.Fatal("Error, walk for stat failed: ", err)
		return
	}

	debug(client, stat, f2s(fid))
	info, err = session.Stat(ctx, fid)
	debug(server, stat, info.String())
	if err != nil {
		log.Fatal("Error, stat failed: ", err)
		return
	}

	if m != nowrite {
		fmt.Fprintf(wr, "%v\t%v\t%v\t%s\n", os.FileMode(info.Mode), info.Length, info.ModTime.Format(timeFormat), info.Name)
	}
	
	wr.Flush()

	return info, nil
}


/* A program for connecting to 9p file servers and performing client ops. */
func main() {
	// Use 9p2000 by default, maybe extend this later if 9p2020 is finished
	pversion = "9p2000"
	debug = chattyPrint
	usage := "usage: 9p [-Dn] [-a address] [-A aname] [-u user] cmd args..."
	ctx = context.Background()

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

	session, err = p9p.NewSession(ctx, conn)
	if err != nil {
		log.Fatal("Error, 9p session failed with: ", err)
	}

	msize, pversion = Version()

	// Attach root
	nfid = 0
	rfid = nfid

	aname = "/"
	// root Qid is not being used, yet 
	_, err = Attach(nfid, p9p.NOFID)
	if err != nil {
		log.Fatal("Error, Root Attach failed with: ", err)
	}
	//fmt.Println("Root Qid: ", rqid)
	defer Clunk(nfid)
	
	// Walk root so that we can clunk it later(?)
	nfid++
	_, err = Walk(rfid, nfid)
	if err != nil {
		log.Fatal("Error, Root Walk failed with: ", err)
	}
	defer Clunk(nfid)

	// Parse commands for the operation to perform
	switch cmd {
	case "read":
		if len(args) > 1 {
			log.Fatal("Error, read takes a single argument.")
		}
		Read(nolist)

	case "write":

	case "stat":
		if len(args) > 1 {
			log.Fatal("Error, stat takes a single argument.")
		}
		Stat(nomode)

	case "rdwr":

	case "ls":
		if len(args) > 1 {
			log.Fatal("Error, ls takes a single argument.")
		}
		Ls()

	case "create":

	case "mkdir":

	case "rm":

	case "open":
		if len(args) > 1 {
			log.Fatal("Error, open takes a single argument.")
		}
		// Walk
		names := mknames(args[0])
		nfid++
		fid := nfid
		_, err = Walk(rfid, fid, names...)
		if err != nil {
			log.Fatal("Error, unable to walk for open: ", err)
		}
		defer Clunk(fid)

		// Open
		_, _, err = Open(fid, p9p.OREAD)
		if err != nil {
			log.Fatal("Error, unable to open for open: ", err)
		}

	default:
		log.Fatal("Error, Specify a valid operation to perform.")
	}

}


// List files in a directory
func Ls() error {
	wr := tabwriter.NewWriter(os.Stdout, 0, 8, 8, ' ', 0)
	
	dir, err := Stat(nowrite)
	if err != nil {
		log.Fatal("Error, failed stat for ls: ", err)
	}
	allbytes, err := Read(nowrite)
	if err != nil {
		log.Fatal("Error, failed read for ls: ", err)
	}
	rd := bytes.NewReader(allbytes)
	codec := p9p.NewCodec()
	
	for {
		err = p9p.DecodeDir(codec, rd, &dir)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				log.Fatal("Error, dir decode failed for ls: ", err)
			}
		}
		name := dir.Name
		if os.FileMode(dir.Mode).IsDir() {
			name += "/"
		}
		fmt.Fprintf(wr, "%v\t%v\t%v\t%s\n", os.FileMode(dir.Mode), dir.Length, dir.ModTime.Format(timeFormat), name)
	}
	
	wr.Flush()
	
	return nil
}


