package main

import (
	"log"
	"strings"

	"github.com/tidwall/doppio"
	"github.com/tidwall/redcon"
)

type conn struct {
	is   doppio.InputStream
	addr string
}

func main() {
	var conns = make(map[int]*conn)
	var keys = make(map[string]string)
	var events doppio.Events
	events.Serving = func(wake func(id int) bool) (action doppio.Action) {
		log.Printf("serving at tcp port 6380")
		log.Printf("serving on unix socket")
		return
	}
	events.Opened = func(id int, addr string) (out []byte, opts doppio.Options, action doppio.Action) {
		conns[id] = &conn{}
		return
	}
	events.Closed = func(id int) (action doppio.Action) {
		delete(conns, id)
		return
	}
	events.Data = func(id int, in []byte) (out []byte, action doppio.Action) {
		c := conns[id]
		data := c.is.Begin(in)
		var n int
		var complete bool
		var err error
		var args [][]byte
		for action == doppio.None {
			complete, args, _, data, err = redcon.ReadNextCommand(data, args[:0])
			if err != nil {
				action = doppio.Close
				out = redcon.AppendError(out, err.Error())
				break
			}
			if !complete {
				break
			}
			if len(args) > 0 {
				n++
				switch strings.ToUpper(string(args[0])) {
				default:
					out = redcon.AppendError(out, "ERR unknown command '"+string(args[0])+"'")
				case "PING":
					out = redcon.AppendString(out, "PONG")
				case "SHUTDOWN":
					out = redcon.AppendString(out, "OK")
					action = doppio.Shutdown
				case "QUIT":
					out = redcon.AppendString(out, "OK")
					action = doppio.Close
				case "GET":
					if len(args) != 2 {
						out = redcon.AppendError(out, "ERR wrong number of arguments for '"+string(args[0])+"' command")
					} else {
						val, ok := keys[string(args[1])]
						if !ok {
							out = redcon.AppendNull(out)
						} else {
							out = redcon.AppendBulkString(out, val)
						}
					}
				case "SET":
					if len(args) != 3 {
						out = redcon.AppendError(out, "ERR wrong number of arguments for '"+string(args[0])+"' command")
					} else {
						keys[string(args[1])] = string(args[2])
						out = redcon.AppendString(out, "OK")
					}
				case "DEL":
					if len(args) < 2 {
						out = redcon.AppendError(out, "ERR wrong number of arguments for '"+string(args[0])+"' command")
					} else {
						var n int
						for i := 1; i < len(args); i++ {
							if _, ok := keys[string(args[1])]; ok {
								n++
								delete(keys, string(args[1]))
							}
						}
						out = redcon.AppendInt(out, int64(n))
					}
				case "FLUSHDB":
					keys = make(map[string]string)
					out = redcon.AppendString(out, "OK")
				}
			}
		}
		c.is.End(data)
		return
	}
	err := doppio.Serve(events, "tcp://0.0.0.0:6380", "unix://socket")
	if err != nil {
		log.Fatal(err)
	}
}
