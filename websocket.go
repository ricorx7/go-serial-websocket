///
/// Websocket connection to send and receive data
/// through a web interface
///

package main

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message.
	writeWait = 10 * time.Second

	// Time allowed to read the next message
	readWaitTime = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (readWaitTime * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// upgrader sets the buffer sizes for the websocket.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// websocketConn struct keeps the Websocket connection.
// It also holds the buffer for the data incoming
// from the websocket.
type websocketConn struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// reader is a Websocket reader
func (wsConn *websocketConn) reader() {
	defer func() {
		log.Print("Close the websocket connection from Reader")
		echo.unregister <- wsConn
		wsConn.ws.Close()
	}()

	// Init the websocket reader
	wsConn.ws.SetReadLimit(maxMessageSize)
	//wsConn.ws.SetReadDeadline(time.Now().Add(readWaitTime))
	wsConn.ws.SetPongHandler(func(string) error { wsConn.ws.SetReadDeadline(time.Now().Add(readWaitTime)); return nil })

	for {
		// Block until a message is received from the websocket
		_, message, err := wsConn.ws.ReadMessage()
		if err != nil {
			if err == io.EOF {
				// Connection is closed with EOF so return
				log.Println("EOF in ws")
				break
			}

			log.Println("Error reading data from ws. " + err.Error())
			break
		}
		log.Println("Websocket message: " + string(message))
		echo.serialBroadcast <- message
	}

}

// write writes a message with the given message type and payload.
func (wsConn *websocketConn) write(mt int, payload []byte) error {
	wsConn.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return wsConn.ws.WriteMessage(mt, payload)
}

// writer is a Websocket writer
func (wsConn *websocketConn) writer() {
	defer func() {
		log.Print("Close the websocket connection from writer")
		wsConn.ws.Close()
	}()

	for {
		select {
		// Block until a message is received to write to the websocket
		case message, ok := <-wsConn.send:
			if !ok {
				log.Println("Message for ws is not OK. ")
				wsConn.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := wsConn.write(websocket.TextMessage, message); err != nil {
				log.Println("Error writing. " + err.Error())
				return
			}
		}
	}
}

// wsHandler is the Websocket handler in the HTTP server.
// This will start websocket connection.  It will then
// start the reader and writer for the websocket.
func wsHandler(w http.ResponseWriter, r *http.Request) {
	log.Print("Started a new websocket handler")

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	// Create a websocket and check it was created properly
	ws, err := upgrader.Upgrade(w, r, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		log.Println("Error opening socket: " + err.Error())
		return
	}

	// Make a async channel to create the websocket connection
	// This will block until the buffer is full
	c := &websocketConn{send: make(chan []byte, 256*10), ws: ws}

	// Register the connection with echo
	echo.register <- c

	log.Println("Create Websocket")

	// GoRoutine for the writer
	go c.writer()

	log.Println("Create Websocket writer")

	// Reader
	c.reader()

	log.Println("Create Websocket reader")
}
