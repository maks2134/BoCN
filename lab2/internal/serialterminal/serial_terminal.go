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

func (st *SerialTerminal) SendPacket(address, control byte, data string) error {
	if st.port == nil {
		return fmt.Errorf("port is not open")
	}

	p := packet.NewPacket(address, control, data)

	stuffedData := st.bitStuffer.StuffPacket(p)

	packetInfo := st.bitStuffer.GetStuffedFrameInfo(p)
	st.packetChan <- packetInfo

	_, err := st.port.Write([]byte(stuffedData))
	if err != nil {
		return st.formatError("write to", err)
	}

	log.Printf("Packet sent to %s: Address=0x%02X, Control=0x%02X, Data=%s",
		st.portName, address, control, data)
	st.messageChan <- "TX:" + data

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
	var receivedData string // Изменяем на строку байтов, а не битов

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

				// Ищем флаги в сырых данных
				flag := byte(0x0E)
				for {
					startIdx := strings.IndexByte(receivedData, flag)
					if startIdx == -1 {
						if len(receivedData) > 1024 {
							receivedData = ""
						}
						break
					}

					// Ищем следующий флаг после стартового
					endIdx := strings.IndexByte(receivedData[startIdx+1:], flag)
					if endIdx == -1 {
						break
					}
					endIdx += startIdx + 1

					// Извлекаем полный фрейм включая флаги
					frameData := receivedData[startIdx : endIdx+1]

					// Обрабатываем через DestuffPacket
					packetObj := st.bitStuffer.DestuffPacket(frameData)

					// Убираем обработанные данные из буфера
					receivedData = receivedData[endIdx+1:]

					if packetObj != nil && packetObj.VerifyFCS() {
						log.Printf("Packet received from %s: Address=0x%02X, Control=0x%02X, Data=%s",
							st.portName, packetObj.Address, packetObj.Control, packetObj.Data)
						st.messageChan <- "RX:" + packetObj.Data
					} else if packetObj != nil {
						log.Printf("Invalid packet received from %s: FCS error; frame hex=%x",
							st.portName, []byte(frameData))
					}
				}
			}

			time.Sleep(time.Millisecond * 10)
		}
	}
}
