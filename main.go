package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/evanphx/wildcat"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/panjf2000/gnet/v2"
)

var (
	errMsg      = "Internal Server Error"
	errMsgBytes = []byte(errMsg)
)

type routerStruct struct {
	routes map[string]func(gnet.Conn, []byte, *httpCodec) gnet.Action
}

type PayloadTransaction struct {
	Valor     int    `json:"valor"`
	Tipo      string `json:"tipo"`
	Descricao string `json:"descricao"`
}

func (r *routerStruct) findHandler(path string) (func(gnet.Conn, []byte, *httpCodec) gnet.Action, bool) {
	for registeredPath, handler := range r.routes {
		if len(registeredPath) <= len(path) && registeredPath == path[:len(registeredPath)] {
			return handler, true
		}
	}
	return nil, false
}

var router = routerStruct{
	routes: map[string]func(gnet.Conn, []byte, *httpCodec) gnet.Action{
		"/hello":    helloHandler,
		"/clientes": clientHandler,
	},
}

func writeResponse(hc *httpCodec, content string) {
	hc.buf = append(hc.buf, "HTTP/1.1 200 OK\r\nServer: gnet\r\nContent-Type: application/json	\r\nDate: "...)
	hc.buf = time.Now().AppendFormat(hc.buf, "Mon, 02 Jan 2006 15:04:05 GMT")
	contentLength := len(content)
	formattedString := fmt.Sprintf("\r\nContent-Lenght: %d\r\n\r\n%s", contentLength, content)
	hc.buf = append(hc.buf, formattedString...)
}

func Config() *pgxpool.Config {
	const defaultMaxConns = int32(4)
	const defaultMinConns = int32(0)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 30
	const defaultHealthCheckPeriod = time.Minute
	const defaultConnectTimeout = time.Second * 5

	// Your own Database URL
	const DATABASE_URL string = "postgres://admin:123@localhost:5432/rinha?"

	dbConfig, err := pgxpool.ParseConfig(DATABASE_URL)
	if err != nil {
		log.Fatal("Failed to create a config, error: ", err)
	}

	dbConfig.MaxConns = defaultMaxConns
	dbConfig.MinConns = defaultMinConns
	dbConfig.MaxConnLifetime = defaultMaxConnLifetime
	dbConfig.MaxConnIdleTime = defaultMaxConnIdleTime
	dbConfig.HealthCheckPeriod = defaultHealthCheckPeriod
	dbConfig.ConnConfig.ConnectTimeout = defaultConnectTimeout

	return dbConfig
}

func generateDbConn() *pgxpool.Pool {
	connPool, err := pgxpool.NewWithConfig(context.Background(), Config())
	if err != nil {
		log.Fatal("Error while creating connection to the database!!")
	}

	connection, err := connPool.Acquire(context.Background())
	if err != nil {
		log.Fatal("Error while acquiring connection from the database pool!!")
	}
	defer connection.Release()

	err = connection.Ping(context.Background())
	if err != nil {
		log.Fatal("Could not ping database")
	}

	fmt.Println("Connected to the database!!")
	return connPool
}

func clientHandler(c gnet.Conn, body []byte, hc *httpCodec) gnet.Action {
	pattern := regexp.MustCompile(`^/clientes/(\d+)/\w*`)

	// Try to match the path against the pattern
	path := string(hc.parser.Path)
	match := pattern.FindStringSubmatch(path)
	if match == nil {
		return gnet.None
	}

	id, _ := strconv.Atoi(match[1])
	strId := match[1]

	if path == "/clientes/"+strId+"/transacoes" {
		pgConn := generateDbConn()
		transacoes(pgConn, body, id)
		writeResponse(hc, "{\"response\": \"transacoes id"+strId+"\"}")
	} else if path == "/clientes/"+strId+"/extrato" {
		pgConn := generateDbConn()
		extrato(pgConn, body, id)
		writeResponse(hc, "{\"response\": \"extract id"+strId+"\"}")
	} else {
		writeResponse(hc, "{\"response\": \"not found\"}")
	}

	return gnet.None
}

func extrato(pgConn *pgxpool.Pool, body []byte, client_id int) {
	log.Print("Olá")
}

func transacoes(pgConn *pgxpool.Pool, body []byte, client_id int) {
	log.Print("Olaa")
	var payload PayloadTransaction
	err := json.Unmarshal(body, &payload)

	if err != nil {
		log.Fatalf("Erro ao deserializar JSON: %v", err)
	}
	if reflect.ValueOf(payload).IsZero() {
		log.Fatal("Erro ao deserializar JSON")
	}

	if payload.Tipo == "d" {
		_, err = pgConn.Exec(context.Background(), "select debitar($1, $2, $3)", client_id, payload.Valor, payload.Descricao)
		if err != nil {
			log.Fatal(err)
		}
	} else if payload.Tipo == "c" {
		_, err = pgConn.Exec(context.Background(), "select creditar($1, $2, $3)", client_id, payload.Valor, payload.Descricao)
		if err != nil {
			log.Fatal(err)
		}
	}

}

func helloHandler(c gnet.Conn, body []byte, hc *httpCodec) gnet.Action {
	writeResponse(hc, "{\"hello\": \"Hello World!\"}")
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
	body := buf[headerOffset:]
	bodyLen := int(hc.parser.ContentLength())
	if bodyLen == -1 {
		bodyLen = 0
	}
	buf = buf[headerOffset+bodyLen:]
	if len(buf) > 0 {
		goto pipeline
	}

	handler(c, body, hc)
	c.Write(hc.buf)
	hc.buf = hc.buf[:0]
	c.Close()
	return gnet.None
}

func main() {
	var port int
	var multicore bool

	// Example command: go run main.go --port 8080 --multicore=true
	flag.IntVar(&port, "port", 3000, "server port")
	flag.BoolVar(&multicore, "multicore", true, "multicore")
	flag.Parse()

	hs := &httpServer{addr: fmt.Sprintf("tcp://127.0.0.1:%d", port), multicore: multicore}
	// Start serving!
	log.Println("server exits:", gnet.Run(hs, hs.addr, gnet.WithMulticore(multicore)))
}
