package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/ricorx7/go-serial"
)

// serialPortIO is the  Serial Port struct.
// The portIO and serialPort
// are the same object but use
// different interfaces.
type serialPortIO struct {
	portConf   *SerialConfig      // The serial port configuration
	portIO     io.ReadWriteCloser // Read, Write and Close interface to read and write to the serial port
	serialPort *serial.SerialPort // Serial port connection to manage the
	done       chan bool          // signals the end of this request
	isClosing  bool               // Keep track of whether we're being actively closed just so we don't show scary error messages
}

// SerialConfig is the Serial Port configuration.
type SerialConfig struct {
	Name  string // Port name
	Baud  int    // Baud rate
	RtsOn bool
	DtrOn bool
}

// SpPortList is a list of the serial ports
// with the serial port details.
type SpPortList struct {
	SerialPorts []SpPortItem
}

// SpPortItem describes the serial port
// so it can be listed.
type SpPortItem struct {
	Name                      string
	Friendly                  string
	SerialNumber              string
	DeviceClass               string
	IsOpen                    bool
	IsPrimary                 bool
	RelatedNames              []string
	Baud                      int
	BufferAlgorithm           string
	AvailableBufferAlgorithms []string
	Ver                       float32
	UsbVid                    string
	UsbPid                    string
}

// serialPortHub is the Serial port HUB.
type serialPortHub struct {
	ports      map[*serialPortIO]bool // Opened serial ports.
	write      chan writeRequest      // Write data to serial port
	register   chan *serialPortIO     // Register requests from the connections.
	unregister chan *serialPortIO     // Unregister requests from connections.
}

// SpPortMessage is the Serial Port Message command.
// Wrap the message received from
// the serial port into a JSON struct.
type SpPortMessage struct {
	P string // the port, i.e. com22
	D string // the data, i.e. G0 X0 Y0
}

// writeRequest will send a Write request.
// When constructing the write request,
// set the serial port that the data will
// be sent to.  The serial port can be found
// by the name with the findPortByName().
type writeRequest struct {
	p *serialPortIO // Serial Port
	d string        // Data
}

// serialHub is the Serial Port hub.
// To write to the serial port.
var serialHub = serialPortHub{
	write:      make(chan writeRequest),      // Write to the serial port, the write request will include the port name
	register:   make(chan *serialPortIO),     // Register the serial port connection
	unregister: make(chan *serialPortIO),     // Unregister the serial port connection
	ports:      make(map[*serialPortIO]bool), // Flag if the port is enabled
}

// run will start running the serial port.
func (sh *serialPortHub) run() {

	log.Print("Inside run of serialhub")

	for {
		select {
		// Register a port
		case p := <-sh.register:
			log.Print("Registering a port: ", p.portConf.Name)
			echo.wsBroadcast <- []byte("{\"Cmd\":\"Open\",\"Desc\":\"Got register/open on port.\",\"Port\":\"" + p.portConf.Name + ",\"Baud\":" + strconv.Itoa(p.portConf.Baud) + "}")

			// Register the serial port with the map
			sh.ports[p] = true
			log.Println("Serial Port registered")

			// Unregister a port
		case p := <-sh.unregister:
			log.Print("Unregistering a port: ", p.portConf.Name)
			echo.wsBroadcast <- []byte("{\"Cmd\":\"Close\",\"Desc\":\"Got unregister/close on port.\",\"Port\":\"" + p.portConf.Name + "\",\"Baud\":" + strconv.Itoa(p.portConf.Baud) + "}")

			// Set flag that the serial port is closing
			// so any loops can stops
			p.isClosing = true

			// Close the serial port
			p.serialPort.Close()

			// Delete the serial port from the map
			delete(sh.ports, p)

			// Write to the serial port
		case wr := <-sh.write:
			// if user sent in the commands as one text mode line
			log.Println("SerialPortHub Write")
			write(wr, "")
		}
	}
}

// openPort will open the serial port and initialize it.
// Open the port based off the port name, and baud rate given.
// It returns the serial port struct that contains the IO.ReadWriterCloser
// interface of the serial port, and the serial port hardware and
// port configuration.
func openSerialPort(portname string, baud int) {

	log.Printf("Inside openPort.  Opening serial port %s at %s baud", portname, strconv.Itoa(baud))

	// Set the serial port mode
	mode := &serial.Mode{
		BaudRate: baud, // Baudrate
		Vmin:     0,    // Min
		Vtimeout: 10,   // Timeout
	}

	// Open serial port
	sp, err := serial.OpenPort(portname, mode)
	if err != nil {
		//log.Fatal(err)
		log.Print("Error opening port " + err.Error())
	}

	// Create the serial port configuration
	config := &SerialConfig{Name: portname, Baud: baud, RtsOn: false, DtrOn: false}

	// Create the serial port IO struct
	spio := &serialPortIO{
		portConf:   config, // Port configuration
		portIO:     sp,     // Serial port IO.ReadWriteCloser interface
		serialPort: sp,     // Serial port hardware commands
		isClosing:  false,  // Set flag that the port is not closed
	}

	// Register the serial port
	serialHub.register <- spio

	// Unregister the serial port when shutdown
	defer func() {
		log.Println("Shutting down the serialPortIO")
		serialHub.unregister <- spio
	}()

	log.Println("Serial Port Reader started")

	// Start reading from the serial port
	spio.reader()
}

// closeSerialPort will close the serial port.
// It will be given the serial port name.  If the
// serial port is open, it will close the port.
func closeSerialPort(portName string) {
	//see if we have this port open
	spio, isFound := findPortByName(portName)

	if !isFound {
		//we couldn't find the port, so send err
		//spErr("We could not find the serial port " + portname + " that you were trying to write to.")
		log.Println("Could not find the serial port " + portName + " that you were trying to write to.")
		return
	}

	serialHub.unregister <- spio
}

// reader is the Serial port Reader function
// Will loop through waiting for data and
// and Read will unblock until data is available.
// It will then send the data to the websocket.
func (spio *serialPortIO) reader() {

	for {
		//var buf bytes.Buffer
		ch := make([]byte, 1024)

		// Read in data
		n, err := spio.portIO.Read(ch)

		//log.Println("Read data from ther serial port: " + string(ch))

		// Detect if the port is closing
		if spio.isClosing {
			log.Println("Closing the port")
			break
		}

		// read can return legitimate bytes as well as an error
		// so process the bytes if n > 0
		if n > 0 {

			// Check for error reading
			if err != nil {
				log.Println("Error reading the port.\n", err)
				break
			}

			//log.Print("Read " + strconv.Itoa(n) + " bytes ch: " + string(ch))
			// Set the incoming data to a string and trim blank
			data := string(ch[:n])

			// Create a JSON message of the data
			m := SpPortMessage{spio.portConf.Name, data}
			b, err := json.Marshal(m)
			if err != nil {
				log.Println(err)
				echo.wsBroadcast <- []byte("Error creating json on " + spio.portConf.Name + " " +
					err.Error() + " The data we were trying to convert is: " + string(ch[:n]))
				break
			}

			// Broadcast the JSON data
			echo.wsBroadcast <- b
		}
	}
}

// write the data to the serial port.
// This will take a writeRequest.  The writeRequest
// will include the serial port pointer.
// It will also check if the command is a BREAK.
func write(wr writeRequest, id string) {
	log.Println("serial write port: " + wr.p.portConf.Name)
	log.Println("serial Write: " + wr.d)

	// Check if the command is a BREAK
	cmdU := strings.ToUpper(wr.d)
	if cmdU == "BREAK" {
		wr.p.serialPort.SendBreak(400)
		return
	}

	// FINALLY, OF ALL THE CODE IN THIS PROJECT
	// WE TRULY/FINALLY GET TO WRITE TO THE SERIAL PORT!
	wr.p.portIO.Write([]byte(wr.d))
}

// spWrite will write data to the serial port.
// This will take a 3 parameters.
// SEND [portName] [cmd]
// SEND is the command to send data to the serial port.
// portName is the serial port name.  eg. COM5
// CMD is the command to accomplish.  eg. CSHOW
// It will then construct the writeRequest to send the data
// to the serial port.
func spWrite(arg string) {
	log.Println("Inside spWrite arg: " + arg)
	// Trim the command
	arg = strings.TrimPrefix(arg, " ")

	// Split the command in to the 3 parameters
	args := strings.SplitN(arg, " ", 3)
	if len(args) != 3 {
		errstr := "Could not parse send command: " + arg
		log.Println(errstr)
		return
	}

	// Get the portname
	portname := strings.Trim(args[1], " ")
	log.Println("The port to write to is:" + portname + "---")
	log.Println("The data is:" + args[2] + "---")

	//see if we have this port open
	spio, isFound := findPortByName(portname)

	if !isFound {
		//we couldn't find the port, so send err
		//spErr("We could not find the serial port " + portname + " that you were trying to write to.")
		log.Println("We could not find the serial port " + portname + " that you were trying to write to.")
		return
	}

	// we found our port
	// create our write request
	var wr writeRequest

	// Set the serial port
	wr.p = spio

	// include newline and trim the end
	wr.d = strings.Trim(args[2], "") + "\r"

	log.Println("spWRite to serial port " + wr.d)

	// send it to the write channel
	serialHub.write <- wr
}

// findPortByName will find the serial port by the name.
// This will check the map for the serial port pointer.
func findPortByName(portname string) (*serialPortIO, bool) {
	portnamel := strings.ToLower(portname)
	for port := range serialHub.ports {
		if strings.ToLower(port.portConf.Name) == portnamel {
			// we found our port
			return port, true
		}
	}
	return nil, false
}

// serialPortList will get the Serial Port list.
// It will then broadcast the serial port list to the
// websocket.
func serialPortList() {

	// call our os specific implementation of getting the serial list
	list, _ := GetList()

	// do a quick loop to see if any of our open ports
	// did not end up in the list port list. this can
	// happen on windows in a fallback scenario where an
	// open port can't be identified because it is locked,
	// so just solve that by manually inserting
	for port := range serialHub.ports {

		isFound := false
		for _, item := range list {
			if strings.ToLower(port.portConf.Name) == strings.ToLower(item.Name) {
				isFound = true
			}
		}

		if !isFound {
			// artificially push to front of port list
			log.Println(fmt.Sprintf("Did not find an open port in the serial port list. We are going to artificially push it onto the list. port:%v", port.portConf.Name))
			var ossp OsSerialPort
			ossp.Name = port.portConf.Name
			ossp.FriendlyName = port.portConf.Name
			list = append([]OsSerialPort{ossp}, list...)
		}
	}

	// we have a full clean list of ports now. iterate thru them
	// to append the open/close state, baud rates, etc to make
	// a super clean nice list to send back to browser
	n := len(list)
	spl := SpPortList{make([]SpPortItem, n, n)}

	// now try to get the meta data for the ports. keep in mind this may fail
	// to give us anything
	metaports, err := GetMetaList()
	log.Printf("Got metadata on ports:%v", metaports)

	ctr := 0
	for _, item := range list {

		/*
			Name                      string
			Friendly                  string
			IsOpen                    bool
			IsPrimary                 bool
			RelatedNames              []string
			Baud                      int
			RtsOn                     bool
			DtrOn                     bool
			BufferAlgorithm           string
			AvailableBufferAlgorithms []string
			Ver                       float32
		*/
		spl.SerialPorts[ctr] = SpPortItem{
			Name:            item.Name,
			Friendly:        item.FriendlyName,
			SerialNumber:    item.SerialNumber,
			DeviceClass:     item.DeviceClass,
			IsOpen:          false,
			IsPrimary:       false,
			RelatedNames:    item.RelatedNames,
			Baud:            0,
			BufferAlgorithm: "",
			//AvailableBufferAlgorithms: availableBufferAlgorithms,
			Ver:    versionFloat,
			UsbPid: item.IdProduct,
			UsbVid: item.IdVendor,
		}

		// if we have meta data for this port, use it
		if len(metaports) > 0 {
			setMetaData(&spl.SerialPorts[ctr], metaports)
		}

		// figure out if port is open
		//spl.SerialPorts[ctr].IsOpen = false
		myport, isFound := findPortByName(item.Name)

		if isFound {
			// we found our port
			spl.SerialPorts[ctr].IsOpen = true
			spl.SerialPorts[ctr].Baud = myport.portConf.Baud
			//spl.SerialPorts[ctr].BufferAlgorithm = myport.BufferType
			//spl.SerialPorts[ctr].IsPrimary = myport.IsPrimary
		}
		//ls += "{ \"name\" : \"" + item.Name + "\", \"friendly\" : \"" + item.FriendlyName + "\" },\n"
		ctr++
	}

	// we are getting a crash here, so thinking it's like a null pointer. do some further
	// debug and set default values
	log.Printf("About to marshal the serial port list. spl:%v", spl)
	ls, err := json.MarshalIndent(spl, "", "\t")
	if err != nil {
		log.Println(err)
		echo.wsBroadcast <- []byte("Error creating json on port list " +
			err.Error())
	} else {
		//log.Print("Printing out json byte data...")
		//log.Print(ls)
		echo.wsBroadcast <- ls
	}
}

///
/// Set the META data.
///
func setMetaData(pi *SpPortItem, metadata []OsSerialPort) {
	// loop thru metadata and see if this port (pi) is in the list
	for _, mi := range metadata {
		if pi.Name == mi.Name {
			// we have a winner
			pi.Friendly = mi.FriendlyName
			pi.DeviceClass = mi.DeviceClass
			pi.SerialNumber = mi.SerialNumber
			pi.RelatedNames = mi.RelatedNames
			pi.UsbPid = mi.IdProduct
			pi.UsbVid = mi.IdVendor
			break
		}
	}
}
