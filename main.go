package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"text/template"
)

///
/// Flags to set at startup
///
var (
	version      = "0.1"
	versionFloat = float32(0.1)
	addr         = flag.String("addr", ":8989", "http service address")
	port         = flag.String("port", "", "Serial COM Port")
	baud         = flag.String("baud", "115200", "Baud Rate")
)

// serialHander passes the template
// to the http request.
func serialHandler(c http.ResponseWriter, req *http.Request) {
	//homeTemplate.Execute(c, req.Host)
	t, _ := template.ParseFiles("serial.html")
	t.Execute(c, nil)
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
