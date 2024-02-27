package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/evanphx/wildcat"
	"github.com/panjf2000/gnet/v2"
)

var (
	errMsg      = "Internal Server Error"
	errMsgBytes = []byte(errMsg)
)

type routerStruct struct {
	routes map[string]func(gnet.Conn, []byte, *httpCodec) gnet.Action
}

func (r *routerStruct) findHandler(path string) (func(gnet.Conn, []byte, *httpCodec) gnet.Action, bool) {
	handler, ok := r.routes[path]
	return handler, ok
}

var router = routerStruct{
	routes: map[string]func(gnet.Conn, []byte, *httpCodec) gnet.Action{
		"/hello": helloHandler,
	},
}

func helloHandler(c gnet.Conn, buf []byte, hc *httpCodec) gnet.Action {
	hc.buf = append(hc.buf, "HTTP/1.1 200 OK\r\nServer: gnet\r\nContent-Type: application/json	\r\nDate: "...)
	hc.buf = time.Now().AppendFormat(hc.buf, "Mon, 02 Jan 2006 15:04:05 GMT")
	content := "{\"hello\": \"Hello World!\"}"
	contentLength := len(content)
	formattedString := fmt.Sprintf("\r\nContent-Lenght: %d\r\n\r\n%s", contentLength, content)
	hc.buf = append(hc.buf, formattedString...)
	return gnet.None
}

type httpServer struct {
	gnet.BuiltinEventEngine

	addr      string
	multicore bool
	eng       gnet.Engine
}

type httpCodec struct {
	parser *wildcat.HTTPParser
	buf    []byte
}

func (hs *httpServer) OnBoot(eng gnet.Engine) gnet.Action {
	hs.eng = eng
	log.Printf("echo server with multi-core=%t is listening on %s\n", hs.multicore, hs.addr)
	return gnet.None
}

func (hs *httpServer) OnOpen(c gnet.Conn) ([]byte, gnet.Action) {
	c.SetContext(&httpCodec{parser: wildcat.NewHTTPParser()})
	return nil, gnet.None
}

func (hs *httpServer) OnTraffic(c gnet.Conn) gnet.Action {
	hc := c.Context().(*httpCodec)
	buf, _ := c.Next(-1)

pipeline:
	headerOffset, err := hc.parser.Parse(buf)
	if err != nil {
		c.Write(errMsgBytes)
		return gnet.Close
	}
	path := string(hc.parser.Path)
	log.Println(string(path))
	handler, ok := router.findHandler(path)

	if !ok {
		c.Write([]byte("404 Not Found"))
		return gnet.Close
	}

	bodyLen := int(hc.parser.ContentLength())
	if bodyLen == -1 {
		bodyLen = 0
	}
	buf = buf[headerOffset+bodyLen:]
	if len(buf) > 0 {
		goto pipeline
	}

	handler(c, buf, hc)
	c.Write(hc.buf)
	hc.buf = hc.buf[:0]
	c.Close()
	return gnet.None
}

func main() {
	var port int
	var multicore bool

	// Example command: go run main.go --port 8080 --multicore=true
	flag.IntVar(&port, "port", 9080, "server port")
	flag.BoolVar(&multicore, "multicore", true, "multicore")
	flag.Parse()

	hs := &httpServer{addr: fmt.Sprintf("tcp://127.0.0.1:%d", port), multicore: multicore}
	// Start serving!
	log.Println("server exits:", gnet.Run(hs, hs.addr, gnet.WithMulticore(multicore)))
}
