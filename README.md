A GO application that will allow you to use the serial port over the internet using a websocket connection.


## BUILD
set GOPATH=PATH\To\GO
go build

## Run Application
./go-serial-websocket --port COM5 --baud 115200

This will open a serial port connection to COM5 with a baudrate of 115200.  
Open up the web browser and go to the file path:
localhost:8989/serial

Communicate with the serial port using the web browser.

Most of this code was derived from http://github.com/johnlauer/serial-port-json-server
