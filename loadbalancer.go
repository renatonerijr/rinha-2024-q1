package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/evanphx/wildcat"
	roundrobin "github.com/hlts2/round-robin"
	"github.com/panjf2000/gnet/v2"
)

type httpServer struct {
	gnet.BuiltinEventEngine
	addr         string
	multicore    bool
	eng          gnet.Engine
	loadBalancer roundrobin.RoundRobin // Added load balancer instance
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

func writeResponse(hc *httpCodec, content string, status_code int) {
	formattedString := fmt.Sprintf("HTTP/1.1 %s OK\r\nServer: gnet\r\nContent-Type: application/json	\r\nDate: ", strconv.Itoa(status_code))
	hc.buf = append(hc.buf, formattedString...)
	hc.buf = time.Now().AppendFormat(hc.buf, "Mon, 02 Jan 2006 15:04:05 GMT")
	contentLength := len(content)
	formattedString = fmt.Sprintf("\r\nContent-Lenght: %d\r\n\r\n%s", contentLength, content)
	hc.buf = append(hc.buf, formattedString...)
}

func (hs *httpServer) OnTraffic(c gnet.Conn) gnet.Action {
	// Get the next target URL using the load balancer
	targetUrl := hs.loadBalancer.Next().String()

	// Extract HTTP request data from the connection
	codec := c.Context().(*httpCodec)
	buf, _ := c.Next(-1)

	_, err := codec.parser.Parse(buf)
	if err != nil {
		log.Println("Error parsing request:", err)
		return gnet.Close
	}

	// Create a new HTTP client
	client := http.Client{}
	headerOffset, err := codec.parser.Parse(buf)
	body := buf[headerOffset:]

	buffer := bytes.NewBuffer([]byte(body))
	reqUrl := "http:" + targetUrl + string(codec.parser.Path)
	// Create a new request to the backend server
	newReq, err := http.NewRequest(string(codec.parser.Method), reqUrl, buffer)
	if err != nil {
		log.Println("Error creating new request:", err)
		return gnet.Close
	}

	// Copy headers from the original request
	for _, h := range codec.parser.Headers {
		if string(h.Value) != "" {
			newReq.Header.Add(string(h.Name), string(h.Value))
		}
	}

	// Send the request to the backend and receive the response
	resp, err := client.Do(newReq)
	if err != nil {
		log.Println("Error sending request to backend:", err)
		return gnet.Close
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error decoding request from backend:", err)
		return gnet.Close
	}

	// Forward the response back to the client
	writeResponse(codec, string(bodyBytes), 200)
	c.Write(codec.buf)
	c.Close()
	return gnet.None
}

func main() {
	var port int
	var multicore bool
	loadBalancer, _ := roundrobin.New(
		&url.URL{Host: "api01:3000"},
		&url.URL{Host: "api02:3000"},
	)

	// Example command: go run main.go --port 8080 --multicore=true
	flag.IntVar(&port, "port", 9999, "server port")
	flag.BoolVar(&multicore, "multicore", true, "multicore")
	flag.Parse()

	hs := &httpServer{addr: fmt.Sprintf("tcp://0.0.0.0:%d", port), multicore: multicore, loadBalancer: loadBalancer}
	// Start serving!
	log.Println("server exits:", gnet.Run(hs, hs.addr, gnet.WithMulticore(multicore)))
}
