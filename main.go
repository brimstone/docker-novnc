package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/brimstone/gowebsockify/internal/assetfs"
	"github.com/brimstone/logger"
	"github.com/gorilla/websocket"
	"golang.org/x/tools/godoc/vfs/httpfs"
)

type Config struct {
	ListenPort int
	VNCAddr    string
}

var (
	config Config
)

type program struct{}

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsToTCP(wsConn *websocket.Conn, tcpConn net.Conn) chan error {
	log := logger.New()
	done := make(chan error, 2)
	go func() {
		defer wsConn.Close()
		defer tcpConn.Close()
		for {
			t, m, err := wsConn.ReadMessage()
			if err != nil {
				done <- err
				return
			}
			if t == websocket.BinaryMessage {
				_, err = tcpConn.Write(m)
				if err != nil {
					done <- err
					return
				}
			} else {
				log.Println("invalid message", t, m)
			}
		}
		done <- nil
	}()
	return done
}

func tcpToWs(tcpConn net.Conn, wsConn *websocket.Conn) chan error {
	done := make(chan error, 2)
	go func() {
		defer wsConn.Close()
		defer tcpConn.Close()
		data := make([]byte, 4096)
		for {
			l, err := tcpConn.Read(data)
			if err != nil {
				done <- err
				return
			}
			err = wsConn.WriteMessage(websocket.BinaryMessage, data[0:l])
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	return done
}

func handleProxyConnection(w http.ResponseWriter, r *http.Request) {
	log := logger.New()
	log.Info("Connected.",
		log.Field("remote", r.RemoteAddr),
	)
	conn, err := wsupgrader.Upgrade(w, r, http.Header{"Sec-WebSocket-Protocol": {"binary"}})
	if err != nil {
		log.Error("Upgrade error",
			log.Field("err", err),
		)
		return
	}
	defer conn.Close()

	conn2, err := net.Dial("tcp", config.VNCAddr)
	if err != nil {
		log.Println("connect tcp", err)
		conn.WriteJSON(map[string]interface{}{"error": "connect failed"})
		return
	}
	defer conn2.Close()

	done1 := tcpToWs(conn2, conn)
	done2 := wsToTCP(conn, conn2)

	// wait
	log.Println("done2", <-done2)
	log.Println("done1", <-done1)
	log.Println("disconnect")
}

func run() {
	log := logger.New()
	mux := http.NewServeMux()

	mux.HandleFunc("/websockify", handleProxyConnection)
	mux.Handle("/", http.FileServer(httpfs.New(assetfs.New())))

	listenAddr := ":" + fmt.Sprint(config.ListenPort)
	log.Info("start server",
		log.Field("address", listenAddr),
	)
	http.ListenAndServe(listenAddr, logger.HTTP(mux))
}

func main() {
	log := logger.New()
	var err error
	config.ListenPort = 9000
	if p := os.Getenv("PORT"); p != "" {
		config.ListenPort, err = strconv.Atoi(p)
		if err != nil {
			panic(err)
		}
	}
	flag.StringVar(&config.VNCAddr, "vnc", "127.0.0.1:5900", "host:port for vnc server")
	flag.IntVar(&config.ListenPort, "port", config.ListenPort, "Port for listening")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		run()
		return
	}
	go run()
	cmd := exec.Command(args[0], args[1:]...)
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Waiting for command to finish...")
	err = cmd.Wait()
	log.Printf("Command finished with error: %v", err)

}
