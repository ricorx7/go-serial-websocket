///
/// Websocket connection to send and receive data
/// through a web interface
///

package main

import (
	"log"

	"github.com/gorilla/websocket"
)

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
	for {
		_, message, err := wsConn.ws.ReadMessage()
		if err != nil {
			break
		}
		log.Println(string(message))
		echo.serialBroadcast <- message
	}
	wsConn.ws.Close()
}

// writer is a Websocket writer
func (wsConn *websocketConn) writer() {
	for message := range wsConn.send {
		//log.Println("WS send data")
		err := wsConn.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
	wsConn.ws.Close()
}
