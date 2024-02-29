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

type TransactionResult struct {
	Valor   int
	Error   bool
	Message string
	Limite  int
}

type ExtractResult struct {
	Saldo                  int
	Limite                 int
	TransactionValue       int
	TransactionType        string
	TransactionDescription string
	TransactionDate        time.Time
}

func handler404(c gnet.Conn, body []byte, hc *httpCodec) gnet.Action {
	writeResponse(hc, "{\"message\":\"404 not found\"}", 404)
	return gnet.None
}

func (r *routerStruct) findHandler(path string) (func(gnet.Conn, []byte, *httpCodec) gnet.Action, bool) {
	for registeredPath, handler := range r.routes {
		if len(registeredPath) <= len(path) && registeredPath == path[:len(registeredPath)] {
			return handler, true
		}
	}
	return handler404, true
}

var router = routerStruct{
	routes: map[string]func(gnet.Conn, []byte, *httpCodec) gnet.Action{
		"/hello":    helloHandler,
		"/clientes": clientHandler,
	},
}

func writeResponse(hc *httpCodec, content string, status_code int) {
	formattedString := fmt.Sprintf("HTTP/1.1 %s OK\r\nServer: gnet\r\nContent-Type: application/json	\r\nDate: ", strconv.Itoa(status_code))
	hc.buf = append(hc.buf, formattedString...)
	hc.buf = time.Now().AppendFormat(hc.buf, "Mon, 02 Jan 2006 15:04:05 GMT")
	contentLength := len(content)
	formattedString = fmt.Sprintf("\r\nContent-Lenght: %d\r\n\r\n%s", contentLength, content)
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
	const DATABASE_URL string = "postgres://admin:123@db:5432/rinha?"

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
		log.Print(err)
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
		msg, status_code := transacoes(pgConn, body, id)
		pgConn.Close()
		writeResponse(hc, msg, status_code)
	} else if path == "/clientes/"+strId+"/extrato" {
		pgConn := generateDbConn()
		msg, status_code := extrato(pgConn, body, id)
		pgConn.Close()
		writeResponse(hc, msg, status_code)
	}

	return gnet.None
}

func extrato(pgConn *pgxpool.Pool, body []byte, client_id int) (string, int) {
	a, err := pgConn.Query(context.Background(), "select get_cliente_data_raw($1)", client_id)
	if err != nil {
		return "{\"message\": \"Erro na query sql\"}", 422
	}
	var result ExtractResult
	var resultList []ExtractResult
	for a.Next() {
		a.Scan(&result)
		resultList = append(resultList, result)
	}
	a.Close()
	if reflect.ValueOf(result).IsZero() {
		return "{\"message\": \"usuário não encontrado\"}", 404
	}
	// returnMsg :=
	dataExtrato := time.Now().Format(time.RFC3339Nano)
	returnMsg := fmt.Sprintf("{\"saldo\": { \"total\": %d, \"data_extrato\": \"%s\", \"limite\": %d}, \"ultimas_transacoes\": [", result.Saldo, dataExtrato, result.Limite)
	for i, v := range resultList {
		if v.TransactionType != "" {
			returnMsg += fmt.Sprintf("{\"valor\": %d, \"tipo\": \"%s\", \"descricao\": \"%s\", \"realizada_em\": \"%s\"}", v.TransactionValue, v.TransactionType, v.TransactionDescription, v.TransactionDate)
			if i+1 != len(resultList) {
				returnMsg += ","
			}
		}
	}
	returnMsg += "]}"
	return returnMsg, 200
}

func transacoes(pgConn *pgxpool.Pool, body []byte, client_id int) (string, int) {
	var payload PayloadTransaction
	err := json.Unmarshal(body, &payload)

	if err != nil {
		return "{\"message\": \"erro ao deserializar JSON\"}", 422
	}

	if reflect.ValueOf(payload).IsZero() {
		return "{\"message\": \"erro ao deserializar JSON\"}", 422
	}

	if payload.Valor <= 0 {
		return "{\"message\": \"valor nao pode ser negativo ou zero\"}", 422
	}

	if len(payload.Descricao) > 10 {
		return "{\"message\": \"descricao contem mais de 10chars\"}", 422
	}

	if payload.Tipo == "d" {
		a, err := pgConn.Query(context.Background(), "select debitar($1, $2, $3)", client_id, payload.Valor, payload.Descricao)
		if err != nil {
			return "{\"message\": \"Erro na query sql\"}", 422
		}
		var result TransactionResult
		for a.Next() {
			a.Scan(&result)
		}
		a.Close()
		if result.Error {
			formatedMessage := fmt.Sprintf("{\"message\": \"%s\"}", result.Message)
			return formatedMessage, 422
		} else {
			formatedMessage := fmt.Sprintf("{\"limite\": \"%d\", \"saldo\": \"%d\"}", result.Limite, result.Valor)
			return formatedMessage, 200
		}

	} else if payload.Tipo == "c" {
		a, err := pgConn.Query(context.Background(), "select creditar($1, $2, $3)", client_id, payload.Valor, payload.Descricao)
		if err != nil {
			log.Fatal(err)
		}
		var result TransactionResult
		for a.Next() {
			a.Scan(&result)
		}
		a.Close()
		if result.Error {
			formatedMessage := fmt.Sprintf("{\"message\": \"%s\"}", result.Message)
			return formatedMessage, 422
		} else {
			formatedMessage := fmt.Sprintf("{\"limite\": \"%d\", \"saldo\": \"%d\"}", result.Limite, result.Valor)
			return formatedMessage, 200
		}
	} else {
		return "{\"message\": \"tipo invalido\"}", 422
	}

}

func helloHandler(c gnet.Conn, body []byte, hc *httpCodec) gnet.Action {
	writeResponse(hc, "{\"hello\": \"Hello World!\"}", 200)
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

	hs := &httpServer{addr: fmt.Sprintf("tcp://0.0.0.0:%d", port), multicore: multicore}
	// Start serving!
	log.Println("server exits:", gnet.Run(hs, hs.addr, gnet.WithMulticore(multicore)))
}
