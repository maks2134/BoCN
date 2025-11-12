package serialterminal

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"oks/internal/csmacd"
	"oks/internal/packet"

	"fyne.io/fyne/v2"
	"github.com/tarm/serial"
)

type SerialTerminal struct {
	port           io.ReadWriteCloser
	portName       string
	dataBits       int
	stopReading    chan bool
	messageChan    chan string
	packetChan     chan string
	bitStuffer     *packet.BitStuffer
	csmaCD         *csmacd.CSMACD
	OnMessage      func(string)
	OnStatus       func(string)
	OnPacket       func(string)
	OnCollision    func()
	OnChannelBusy  func()
	OnChannelState func(string)
}

func New(name string) *SerialTerminal {
	csma := csmacd.NewCSMACD()
	terminal := &SerialTerminal{
		portName:       name,
		dataBits:       8,
		stopReading:    make(chan bool, 1),
		messageChan:    make(chan string, 100),
		packetChan:     make(chan string, 50),
		bitStuffer:     packet.NewBitStuffer(),
		csmaCD:         csma,
		OnMessage:      func(string) {},
		OnStatus:       func(string) {},
		OnPacket:       func(string) {},
		OnCollision:    func() {},
		OnChannelBusy:  func() {},
		OnChannelState: func(string) {},
	}

	csma.SetCallbacks(
		func(state csmacd.ChannelState) {
			log.Printf("CSMA/CD: Channel state changed to %s", csma.GetStateString())
			fyne.Do(func() { terminal.OnChannelState(csma.GetStateString()) })
		},
		func() {
			log.Printf("CSMA/CD: ⚠️ COLLISION DETECTED - Sending jam signal")
			fyne.Do(func() { terminal.OnCollision() })
		},
		func() {
			log.Printf("CSMA/CD: 🔴 Channel busy - waiting...")
			fyne.Do(func() { terminal.OnChannelBusy() })
		},
	)

	return terminal
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

func (st *SerialTerminal) GetCSMAStatistics() (collisions, busy, totalAttempts int) {
	return st.csmaCD.GetStatistics()
}

func (st *SerialTerminal) GetCSMAStatisticsString() string {
	return st.csmaCD.GetStatisticsString()
}

func (st *SerialTerminal) GetChannelState() string {
	return st.csmaCD.GetStateString()
}

func (st *SerialTerminal) SetCSMAEmulation(enabled bool) {
	st.csmaCD.SetEmulationEnabled(enabled)
}

func (st *SerialTerminal) SetCSMAProbabilities(busyProb, collisionProb float64) {
	st.csmaCD.SetProbabilities(busyProb, collisionProb)
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

	maxRetries := 16
	for attempt := 0; attempt < maxRetries; attempt++ {
		log.Printf("CSMA/CD: Attempt %d - Listening to channel...", attempt+1)
		if !st.csmaCD.ListenToChannel() {
			log.Printf("CSMA/CD: Channel busy, waiting... (attempt %d)", attempt+1)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		log.Printf("CSMA/CD: Channel idle, starting transmission...")
		if !st.csmaCD.StartTransmission() {
			log.Printf("CSMA/CD: Failed to start transmission, channel not idle (attempt %d)", attempt+1)
			continue
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
			st.csmaCD.EndTransmission()
			return st.formatError("write to", err)
		}

		log.Printf("CSMA/CD: Checking for collision during transmission...")
		if st.csmaCD.DetectCollision() {
			log.Printf("CSMA/CD: Collision detected during transmission (attempt %d)", attempt+1)
			st.csmaCD.SendJamSignal()
			st.csmaCD.EndTransmission()

			backoffDelay := st.csmaCD.CalculateBackoffDelay()
			log.Printf("CSMA/CD: Backing off for %v", backoffDelay)
			time.Sleep(backoffDelay)
			continue
		}

		log.Printf("CSMA/CD: Transmission successful!")
		log.Printf("Packet sent to %s: Address=0x%02X, Control=0x%02X, Data=%s, FCS=0x%02X",
			st.portName, address, control, original.Data, original.FCS)
		st.messageChan <- "TX:" + original.Data
		st.csmaCD.EndTransmission()
		st.csmaCD.ResetBackoff()
		return nil
	}

	return fmt.Errorf("maximum retry attempts (%d) exceeded", maxRetries)
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
						receivedData = receivedData[startIdx+1:]
						continue
					}

					hasErrors, errorCount, correctedData := packetObj.DetectAndCorrectErrors()

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
					} else {
					}
				}
			}

		}
	}
}
