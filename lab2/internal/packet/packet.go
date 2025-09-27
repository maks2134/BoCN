package packet

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

type Packet struct {
	Flag    byte
	Address byte
	Control byte
	Data    string
	FCS     byte
}

func NewPacket(address, control byte, data string) *Packet {
	p := &Packet{
		Flag:    0x0E,
		Address: address,
		Control: control,
		Data:    data,
	}
	p.FCS = p.CalculateFCS()
	return p
}

func (p *Packet) CalculateFCS() byte {
	fcs := p.Address ^ p.Control
	for i := 0; i < len(p.Data); i++ {
		fcs ^= p.Data[i]
	}
	return fcs
}

func (p *Packet) VerifyFCS() bool {
	return p.FCS == p.CalculateFCS()
}

func (p *Packet) ToString() string {
	return fmt.Sprintf(
		"Packet Structure (Pre-Stuffing):\n"+
			"Field Name | Value (Hex) | Value (Binary) | Value (String/Length)\n"+
			"---|---|---|---\n"+
			"Flag | 0x%02X | %08b | \n"+
			"Address | 0x%02X | %08b | \n"+
			"Control | 0x%02X | %08b | \n"+
			"Data | %s | %s | %s (Length: %d)\n"+
			"FCS | 0x%02X | %08b | \n",
		p.Flag, p.Flag,
		p.Address, p.Address,
		p.Control, p.Control,
		hex.EncodeToString([]byte(p.Data)), BytesToBinaryString(p.Data), p.Data, len(p.Data),
		p.FCS, p.FCS,
	)
}

func BytesToBinaryString(data string) string {
	var result strings.Builder
	for _, b := range []byte(data) {
		result.WriteString(fmt.Sprintf("%08b", b))
	}
	return result.String()
}

func BinaryStringToBytes(binStr string) string {
	if len(binStr)%8 != 0 {
		padding := 8 - (len(binStr) % 8)
		binStr += strings.Repeat("0", padding)
	}

	var result []byte
	for i := 0; i < len(binStr); i += 8 {
		end := i + 8
		if end > len(binStr) {
			end = len(binStr)
		}
		byteStr := binStr[i:end]
		if val, err := strconv.ParseUint(byteStr, 2, 8); err == nil {
			result = append(result, byte(val))
		}
	}
	return string(result)
}

func (p *Packet) GetFrameData() string {
	return string([]byte{p.Address, p.Control}) + p.Data + string(p.FCS)
}

func (p *Packet) CreateFrame() string {
	return string(p.Flag) + p.GetFrameData() + string(p.Flag)
}

func ParseFrame(frameData string) *Packet {
	if len(frameData) < 4 {
		return nil
	}

	if frameData[0] != 0x0E || frameData[len(frameData)-1] != 0x0E {
		return nil
	}

	packetData := frameData[1 : len(frameData)-1]

	if len(packetData) < 3 {
		return nil
	}

	packet := &Packet{
		Flag:    0x0E,
		Address: packetData[0],
		Control: packetData[1],
	}

	if len(packetData) > 3 {
		packet.Data = packetData[2 : len(packetData)-1]
		packet.FCS = packetData[len(packetData)-1]
	} else {
		packet.FCS = packetData[2]
	}

	return packet
}
