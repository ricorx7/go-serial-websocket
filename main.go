package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"text/template"

	"github.com/gorilla/websocket"
)

///
/// Flags to set at startup
///
var (
	version      = "0.1"
	versionFloat = float32(0.1)
	addr         = flag.String("addr", ":8989", "http service address")
	port         = flag.String("port", "COM6", "Serial COM Port")
	baud         = flag.String("baud", "115200", "Baud Rate")
)

// serialHander passes the template
// to the http request.
func serialHandler(c http.ResponseWriter, req *http.Request) {
	//homeTemplate.Execute(c, req.Host)
	t, _ := template.ParseFiles("serial.html")
	t.Execute(c, nil)
}

// wsHandler is the Websocket handler in the HTTP server.
// This will start websocket connection.  It will then
// start the reader and writer for the websocket.
func wsHandler(w http.ResponseWriter, r *http.Request) {
	log.Print("Started a new websocket handler")

	// Create a websocket and check it was created properly
	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		return
	}

	// Make a async channel to create the websocket connection
	// This will block until the buffer is full
	c := &websocketConn{send: make(chan []byte, 256*10), ws: ws}

	// Register the connection with echo
	echo.register <- c
	defer func() { echo.unregister <- c }()

	// GoRoutine for the writer
	go c.writer()

	// Reader
	c.reader()
}

// main will start the application.
func main() {
	// Parse the flags
	flag.Parse()

	// setup logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Display the flags
	log.Println("Port:" + *port)
	log.Println("Baud:" + *baud)
	log.Println("Addr: " + *addr)

	// Convert the baud rate to int
	baudInt, err := strconv.Atoi(*baud)
	if err != nil {
		log.Println("Baud rate give is bad")
		return
	}

	// Start Echo
	go echo.init(port, baudInt)

	// HTTP server
	http.HandleFunc("/serial", serialHandler) // Display the websocket data
	http.HandleFunc("/ws", wsHandler)         // wsHandler in websocketConn.go.  Creates websocket
	if err := http.ListenAndServe(*addr, nil); err != nil {
		fmt.Printf("Error trying to bind to port: %v, so exiting...", err)
		log.Fatal("Error ListenAndServe:", err)
	}
}
