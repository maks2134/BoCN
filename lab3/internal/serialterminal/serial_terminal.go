package serialterminal

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"oks/internal/packet"

	"fyne.io/fyne/v2"
	"github.com/tarm/serial"
)

type SerialTerminal struct {
	port        io.ReadWriteCloser
	portName    string
	dataBits    int
	stopReading chan bool
	messageChan chan string
	packetChan  chan string
	bitStuffer  *packet.BitStuffer
	OnMessage   func(string)
	OnStatus    func(string)
	OnPacket    func(string)
}

func New(name string) *SerialTerminal {
	return &SerialTerminal{
		portName:    name,
		dataBits:    8,
		stopReading: make(chan bool, 1),
		messageChan: make(chan string, 100),
		packetChan:  make(chan string, 50),
		bitStuffer:  packet.NewBitStuffer(),
		OnMessage:   func(string) {},
		OnStatus:    func(string) {},
		OnPacket:    func(string) {},
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
		err := st.Disconnect()
		if err != nil {
			return
		}
		time.Sleep(time.Millisecond * 100)
		err = st.Connect()
		if err != nil {
			return
		}
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
		ReadTimeout: time.Millisecond * 50,
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

func (st *SerialTerminal) SendPacket(address, control byte, data string) error {
	if st.port == nil {
		return fmt.Errorf("port is not open")
	}

	original := packet.NewPacket(address, control, data)
	corrupted := packet.NewPacket(address, control, data)
	corrupted.SimulateCorruption()
	log.Printf("Bit corruption simulated for packet: Address=0x%02X, Control=0x%02X, Data=%s, FCS=0x%02X",
		address, control, corrupted.Data, corrupted.FCS)

	stuffedData := st.bitStuffer.StuffPacket(corrupted)

	packetInfo := st.bitStuffer.GetTransmissionInfo(original, corrupted)
	st.packetChan <- packetInfo

	_, err := st.port.Write([]byte(stuffedData))
	if err != nil {
		return st.formatError("write to", err)
	}

	log.Printf("Packet sent to %s: Address=0x%02X, Control=0x%02X, Data=%s, FCS=0x%02X",
		st.portName, address, control, original.Data, original.FCS)
	st.messageChan <- "TX:" + original.Data

	return nil
}

func (st *SerialTerminal) SendMessage(msg string) error {
	address := byte(0x01)
	if strings.Contains(st.portName, "ttys003") {
		address = 0x02
	}

	return st.SendPacket(address, 0x00, msg)
}

func (st *SerialTerminal) messageHandler() {
	for {
		select {
		case msg := <-st.messageChan:
			if st.OnMessage != nil {
				fyne.Do(func() { st.OnMessage(msg) })
			}
		case packetInfo := <-st.packetChan:
			if st.OnPacket != nil {
				fyne.Do(func() { st.OnPacket(packetInfo) })
			}
		case <-st.stopReading:
			return
		}
	}
}

func (st *SerialTerminal) readPort() {
	buf := make([]byte, 512)
	var receivedData string

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
				receivedData += string(buf[:n])

				flag := byte(0x0E)
				for {
					startIdx := strings.IndexByte(receivedData, flag)
					if startIdx == -1 {
						if len(receivedData) > 1024 {
							receivedData = ""
						}
						break
					}

					endIdx := strings.IndexByte(receivedData[startIdx+1:], flag)
					if endIdx == -1 {
						break
					}
					endIdx += startIdx + 1

					frameData := receivedData[startIdx : endIdx+1]
					packetObj := st.bitStuffer.DestuffPacket(frameData)

					if packetObj == nil {
						// false flag, drop one byte after start and search again
						receivedData = receivedData[startIdx+1:]
						continue
					}

					hasErrors, errorCount, correctedData := packetObj.DetectAndCorrectErrors()

					// consume only when we have a frame to act on
					receivedData = receivedData[endIdx+1:]

					if !hasErrors {
						log.Printf("Packet received from %s: Address=0x%02X, Control=0x%02X, Data=%s, FCS=0x%02X",
							st.portName, packetObj.Address, packetObj.Control, packetObj.Data, packetObj.FCS)
						st.messageChan <- "RX:" + packetObj.Data
					} else if errorCount == 1 {
						log.Printf("Single error detected and corrected from %s: Original=%s, Corrected=%s, FCS=0x%02X",
							st.portName, packetObj.Data, correctedData, packetObj.FCS)
						st.messageChan <- "RX:" + correctedData
					} else if errorCount == 2 {
						log.Printf("Double error detected from %s: Data=%s, FCS=0x%02X (cannot correct)",
							st.portName, packetObj.Data, packetObj.FCS)
						// Suppress showing any message on RX for errored frames
					} else {
						// Suppress showing any message on RX for unknown/uncorrectable errors
					}
				}
			}

		}
	}
}
