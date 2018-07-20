# 9p

A set of tools for interacting with 9p filesystems written in Go.

## Index

* 9p -- 9p client application which can perform several basic operations.

## Examples

### 9p

To perform an `ls(1)` operation on the `/` directory of a local 9p server on port 5640:

	./9p -a 'localhost!5640' ls /

To listen to the /grid/ radio:

	./9p -a 'plan-nue.youkai.pw!4458' read radio | mplayer -cache 2048 -

To listen to the /grid/ radio from a unix socket in `namespace`:

	srv 'tcp!plan-nue.youkai.pw!4458' radio
	./9p -a 'unix!/tmp/ns.'$USER'.:0/radio' read radio |  mplayer -cache 2048 -

## Notes

Tested on go 1.10.x/amd64 on Windows 10 and Slackware64 14.2

## Bugs

Oh yeah.

