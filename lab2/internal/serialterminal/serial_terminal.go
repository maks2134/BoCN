package serialterminal

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/tarm/serial"
)

type SerialTerminal struct {
	port        io.ReadWriteCloser
	portName    string
	dataBits    int
	stopReading chan bool
	messageChan chan string
	OnMessage   func(string)
	OnStatus    func(string)
}

func New(name string) *SerialTerminal {
	return &SerialTerminal{
		portName:    name,
		dataBits:    8,
		stopReading: make(chan bool, 1),
		messageChan: make(chan string, 100),
		OnMessage:   func(string) {},
		OnStatus:    func(string) {},
	}
}

func (st *SerialTerminal) SetPortName(name string) {
	st.portName = name
}

func (st *SerialTerminal) SetDataBits(dataBits int) {
	oldDataBits := st.dataBits
	st.dataBits = dataBits

	if st.port != nil && oldDataBits != dataBits {
		log.Printf("Data bits changed from %d to %d, reconnecting...", oldDataBits, dataBits)
		st.Disconnect()
		time.Sleep(time.Millisecond * 100)
		st.Connect()
	}
}

func (st *SerialTerminal) GetDataBits() int {
	return st.dataBits
}

func (st *SerialTerminal) GetPortName() string {
	return st.portName
}

func (st *SerialTerminal) IsConnected() bool {
	return st.port != nil
}

func (st *SerialTerminal) Connect() error {
	c := &serial.Config{
		Name:        st.portName,
		Baud:        9600,
		ReadTimeout: time.Second * 1,
		Size:        byte(st.dataBits),
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
	}

	s, err := serial.OpenPort(c)
	if err != nil {
		return st.formatError("open", err)
	}

	st.port = s
	if st.OnStatus != nil {
		st.OnStatus(fmt.Sprintf("Port %s open", st.portName))
	}
	log.Printf("Port %s opened successfully", st.portName)

	go st.readPort()
	go st.messageHandler()

	return nil
}

func (st *SerialTerminal) Disconnect() error {
	if st.port != nil {
		select {
		case st.stopReading <- true:
		default:
		}

		err := st.port.Close()
		if err != nil {
			return st.formatError("close", err)
		}

		st.port = nil
		if st.OnStatus != nil {
			st.OnStatus("Port closed")
		}
		log.Printf("Port %s closed", st.portName)
	}
	return nil
}

func (st *SerialTerminal) formatError(operation string, err error) error {
	if os.IsNotExist(err) {
		return fmt.Errorf("serial port %s does not exist", st.portName)
	}
	if os.IsPermission(err) {
		return fmt.Errorf("permission denied for port %s", st.portName)
	}
	return fmt.Errorf("failed to %s port %s: %v", operation, st.portName, err)
}

func (st *SerialTerminal) applyDataBitMask(msg string) string {
	if st.dataBits >= 8 {
		return msg
	}

	mask := byte((1 << st.dataBits) - 1)
	result := make([]byte, len(msg))
	for i, char := range []byte(msg) {
		result[i] = char & mask
	}

	return string(result)
}

func (st *SerialTerminal) SendMessage(msg string) error {
	if st.port == nil {
		return fmt.Errorf("port is not open")
	}

	if msg == "" {
		return nil
	}

	maskedMsg := st.applyDataBitMask(msg)
	message := maskedMsg + "\r\n"

	_, err := st.port.Write([]byte(message))
	if err != nil {
		return st.formatError("write to", err)
	}

	log.Printf("Data sent to %s: %s (original: %s, data bits: %d)", st.portName, maskedMsg, msg, st.dataBits)
	st.messageChan <- "TX:" + maskedMsg

	return nil
}

func (st *SerialTerminal) messageHandler() {
	for {
		select {
		case msg := <-st.messageChan:
			if st.OnMessage != nil {
				st.OnMessage(msg)
			}
		case <-st.stopReading:
			return
		}
	}
}

func (st *SerialTerminal) readPort() {
	buf := make([]byte, 256)

	for {
		select {
		case <-st.stopReading:
			log.Printf("Reading stopped for port %s", st.portName)
			return
		default:
			if st.port == nil {
				return
			}

			n, err := st.port.Read(buf)
			if err != nil {
				if err == io.EOF {
					time.Sleep(time.Millisecond * 100)
					continue
				}
				log.Printf("Error reading from port %s: %v", st.portName, err)
				time.Sleep(time.Second * 1)
				continue
			}

			if n > 0 {
				maskedData := make([]byte, n)
				mask := byte((1 << st.dataBits) - 1)
				if st.dataBits < 8 {
					for i := 0; i < n; i++ {
						maskedData[i] = buf[i] & mask
					}
				} else {
					copy(maskedData, buf[:n])
				}

				receivedMessage := string(maskedData)
				log.Printf("Data received from %s: %q (%d bytes, data bits: %d)", st.portName, receivedMessage, n, st.dataBits)
				st.messageChan <- "RX:" + receivedMessage
			}

			time.Sleep(time.Millisecond * 10)
		}
	}
}
