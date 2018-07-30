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


// Control for chattyprint ← or →
type source int
const (
	server source = iota
	client
)

// Supported connection protocols
type protocol int
const (
	tcp protocol = iota
	unix
)

// Operation types for chattyprint formatting
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
	wstat
	remove
	write
)

// Control for Read()
type mode int
const (
	nowrite mode = iota
	nolist
	nomode
)


// Parses the dial string input to net.Dial() format
func parsedialstr(dialstr string) (proto, address string) {
	proto = "tcp"
	address = dialstr
	if strings.HasPrefix(dialstr, "unix!") {
		// Dialing into a unix namespace
		proto = "unix"
		address = dialstr[5:]
	} else if strings.HasPrefix(dialstr, "tcp!") {
		proto = "tcp"
		address = dialstr[4:]
	} else {
		address = dialstr
	}

	// Should add a default port at some point (for cwfs/fossil/venti/hjfs)
	if parts := strings.Split(address, "!"); len(parts) > 1 {
		address = ""
		for p, v := range parts {
			if p == len(parts) - 1 {
				address += ":" + v
			} else if p == 0 {
				address += v
			} else {
				address += "!" + v
			}
		}

	} else {
		// As per srv(4), use 9fs default port
		address += ":564"
	}

	return
}

// Returns the names in path (typically args)
func mknames(path string) []string {
	return strings.Split(strings.TrimSpace(strings.Trim(path, "/")), "/")
}

// Converts a p9p.Fid to a string and so forth
func f2s(fid p9p.Fid) string {
	return fmt.Sprint(fid)
}

// If chatty is enabled, print out 9p transactions. go-p9p does not provide this, sadly. 
func chattyprint(s source, o op, extras ...string) {
	var msg string = "<nil>"
	arrow := '←'
	if s == client {
		arrow = '→'
	}

	switch o {
		case version:
			if s == client {
				msg = "Tversion"
			} else {
				msg = "Rversion"
			}
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
			
		case create:
			if s == client {
				msg = "Tcreate"
				log.Printf("%c %s fid=%s name=%s perm=%s mode=%s", arrow, msg, extras[0], extras[1], extras[2], extras[3])
				break
			}
			msg = "Rcreate"
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
		
		case wstat:
			if s == client {
				msg = "Twstat"
				log.Printf("%c %s fid=%s", arrow, msg, extras[0])
				break
			}
			msg = "Rwstat"
			log.Printf("%c %s dir=%s", arrow, msg, extras[0])

		case write:
			if s == client {
				msg = "Twrite"
				log.Printf("%c %s fid=%s offset=%s iounit=%s count=%s", arrow, msg, extras[0], extras[1], extras[2], extras[3])
				break
			}
			msg = "Rwrite"
			log.Printf("%c %s iounit=%s", arrow, msg, extras[0])
			
		case remove:
			if s == client {
				msg = "Tremove"
				log.Printf("%c %s fid=%s", arrow, msg, extras[0])
				break
			}
			msg = "Rremove"
			log.Printf("%c %s", arrow, msg)

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

	debug(client, open, f2s(fid), fmt.Sprint(mode))
	qid, iounit, err = session.Open(ctx, fid, mode)
	if err != nil {
		debug(server, rerror, err.Error())
		return
	}

	if iounit < 1 {
		// size of message max minus fcall io header (Rread)
		iounit = uint32(msize - 24)
	}

	debug(server, open, qid.String(), fmt.Sprint(iounit))

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
			debug(client, read, f2s(fid), fmt.Sprint(offset), fmt.Sprint(width))
			n, err = session.Read(ctx, fid, buf, offset)
			//fmt.Fprintln(os.Stderr, "Read: ", n, err)
			count += n

			if n < 0 {
				log.Fatal("Error, read error: ", err)
			}
			if err != nil {
				debug(server, rerror, err.Error())
			} else {
				debug(server, read, fmt.Sprint(n))
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
	debug = chattyprint

	usage := `usage: 9p [-Dn] [-a address] [-A aname] [-u user] cmd args...
	commands available:
		read
		stat
		ls
		open
		create
		rm
		mkdir
		write
		chmod
	`
	
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
	proto, addr := parsedialstr(addr)

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
		if len(args) > 1 {
			log.Fatal("Error, write takes a single argument.")
		}
		Write()

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
		if len(args) != 2 {
			log.Fatal("Error, create takes a file path and a permission mode.")
		}
		fmode64, err := sc.ParseUint(args[1], 10, 32)
		if err != nil {
			log.Fatal("Error, invalid file mode for permission set.")
		}
		Creat(p9p.OREAD, uint32(fmode64))
	
	case "mkdir":
		if len(args) != 2 {
			log.Fatal("Error, mkdir takes a file path and a permission mode.")
		}
		fmode64, err := sc.ParseUint(args[1], 10, 32)
		if err != nil {
			log.Fatal("Error, invalid file mode for permission set.")
		}
		Creat(p9p.OREAD, uint32(fmode64) | uint32(os.ModeDir))
	
	case "rm":
		if len(args) > 1 {
			log.Fatal("Error, rm takes a single argument.")
		}
		Remove()

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
	
	case "chmod":
		if len(args) != 2 {
			log.Fatal("Error, chmod takes a file path and a permission mode.")
		}
		Chmod()

	default:
		log.Fatal("Error, Specify a valid operation to perform.")
	}

}


// Call wstat and change mode on a file
func Chmod() error {
	var dir p9p.Dir
	//dir, err := Stat(nowrite)
	odir, err := Stat(nowrite)
	mode64, err := sc.ParseUint(args[1], 10, 32)
	dir = odir
	dir.Mode = uint32(mode64)
	dir.Name = odir.Name
	dir.UID = odir.UID
	dir.GID = odir.GID
	dir.MUID = odir.MUID
	
	nfid++
	fid := nfid
	defer Clunk(fid)

	// Walk -- extract the name in the path to mod
	names := mknames(args[0])

	_, err = Walk(rfid, fid, names...)
	if err != nil {
		log.Fatal("Error, unable to walk for wstat: ", err)
	}
	
	// Open
	_, _, err = Open(fid, p9p.ORDWR)
	if err != nil {
		log.Fatal("Error, unable to open for wstat: ", err)
	}
	debug(client, wstat, f2s(fid))
	fmt.Fprintln(os.Stderr, odir)
	fmt.Fprintln(os.Stderr, dir)
	err = session.WStat(ctx, fid, dir)
	if err != nil {
		debug(server, rerror, err.Error())
	}
	debug(server, wstat, dir.String())

	return nil
}

// Remove a file
func Remove() error {
	nfid++
	fid := nfid
	defer Clunk(fid)

	// Walk
	names := mknames(args[0])
	_, err := Walk(rfid, fid, names...)
	if err != nil {
		log.Fatal("Error, unable to walk for remove: ", err)
	}
	
	// Open
	_, _, err = Open(fid, p9p.ORDWR)
	if err != nil {
		log.Fatal("Error, unable to open for remove: ", err)
	}
	debug(client, remove, f2s(fid))
	err = session.Remove(ctx, fid)
	if err != nil {
		debug(server, rerror, err.Error())
	}
	debug(server, remove)
	
	return nil
}

// Write to a file
func Write() error {
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
	_, width, err := Open(fid, p9p.OWRITE)
	if err != nil {
		log.Fatal("Error, Open failed: ", err)
	}
	buf := make([]byte, width)

	// Write -- might have to loop through msize-ish chunks using offsets (see: 9p.c in p9p)
	var offset int64 = 0
	// count in this fn is the sum of bytes read
	var count int = 0
	var n int = 1
	for ;; offset += int64(n) {
			buf = make([]byte, width)			
			n, err = os.Stdin.Read(buf)
			if n < 0 || err != nil {
				log.Print("Error, read input error: ", err)
			}
						
			count += n

			if n == 0 {
				break
			}
			
			// Output
			debug(client, write, f2s(fid), fmt.Sprint(offset), fmt.Sprint(width), fmt.Sprint(n))
			nout, err := session.Write(ctx, fid, buf[:n], offset)
			
			if nout < 0 {
				log.Fatal("Error, write error: ", err)
			}
			if err != nil {
				debug(server, rerror, err.Error())
			} else {
				debug(server, write, fmt.Sprint(n))
			}
	}
			
	return nil
}

// Create a file
func Creat(mode p9p.Flag, perm uint32) (qid p9p.Qid, iounit uint32, err error) {
	err = nil
	iounit = 0
	
	// Walk -- extract the name in the path to make
	names := mknames(args[0])
	tomake := names[len(names)-1]
	names = names[:len(names)-1]

	// BUG?: We cannot make files in "/" on jsonfs, but can in child dirs
	var fid = rfid
	if len(names) > 0 {
		// We are not in "/"
		nfid++
		fid = nfid
		_, err = Walk(rfid, fid, names...)
		if err != nil {
			log.Fatal("Error, unable to walk for create: ", err)
		}
		defer Clunk(fid)
	}

	// Create
	debug(client, create, f2s(fid), tomake, fmt.Sprint(perm), fmt.Sprint(mode))
	qid, iounit, err = session.Create(ctx, fid, tomake, perm, mode)
	if err != nil {
		debug(server, rerror, err.Error())
		return
	}

	if iounit < 1 {
		// size of message max minus fcall io header (Rread)
		iounit = uint32(msize - 24)
	}

	debug(server, create, qid.String(), fmt.Sprint(iounit))

	return
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


