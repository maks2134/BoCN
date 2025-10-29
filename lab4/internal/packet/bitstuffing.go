package packet

import (
	"fmt"
	"strings"
)

func groupBinary(bits string) string {
	if len(bits) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(bits); i += 8 {
		end := i + 8
		if end > len(bits) {
			end = len(bits)
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(bits[i:end])
	}
	return b.String()
}

type BitStuffer struct{}

func NewBitStuffer() *BitStuffer {
	return &BitStuffer{}
}

func (bs *BitStuffer) Stuff(binaryData string) string {
	var result strings.Builder
	i := 0
	for i < len(binaryData) {
		if i+7 <= len(binaryData) && binaryData[i:i+7] == "0000111" {
			result.WriteString("0000111")
			result.WriteByte('1')
			i += 7
			continue
		}
		result.WriteByte(binaryData[i])
		i++
	}
	return result.String()
}

func (bs *BitStuffer) Destuff(stuffedData string) string {
	var result strings.Builder
	i := 0
	for i < len(stuffedData) {
		if i+8 <= len(stuffedData) && stuffedData[i:i+8] == "00001111" {
			result.WriteString("0000111")
			i += 8
			continue
		}
		result.WriteByte(stuffedData[i])
		i++
	}
	return result.String()
}

func (bs *BitStuffer) StuffPacket(p *Packet) string {
	stuffedAddress := bs.Stuff(fmt.Sprintf("%08b", p.Address))
	stuffedControl := bs.Stuff(fmt.Sprintf("%08b", p.Control))
	stuffedData := bs.Stuff(BytesToBinaryString(p.Data))
	stuffedFCS := bs.Stuff(fmt.Sprintf("%08b", p.FCS))

	stuffedFrame := stuffedAddress + stuffedControl + stuffedData + stuffedFCS
	pad := (8 - (len(stuffedFrame) % 8)) % 8
	if pad > 0 {
		stuffedFrame += strings.Repeat("0", pad)
	}

	flagBinary := fmt.Sprintf("%08b", p.Flag)
	finalBinary := flagBinary + stuffedFrame + flagBinary

	return BinaryStringToBytes(finalBinary)
}

func (bs *BitStuffer) DestuffPacket(receivedData string) *Packet {
	binaryData := BytesToBinaryString(receivedData)

	if len(binaryData) < 16 {
		return nil
	}

	startFlag := binaryData[0:8]
	endFlag := binaryData[len(binaryData)-8:]

	if startFlag != "00001110" || endFlag != "00001110" {
		return nil
	}

	stuffedFrame := binaryData[8 : len(binaryData)-8]
	destuffedFrame := bs.Destuff(stuffedFrame)
	if rem := len(destuffedFrame) % 8; rem != 0 {
		trim := rem
		if trim > len(destuffedFrame) {
			trim = len(destuffedFrame)
		}
		for trim > 0 && destuffedFrame[len(destuffedFrame)-1] == '0' {
			destuffedFrame = destuffedFrame[:len(destuffedFrame)-1]
			trim--
		}
		if len(destuffedFrame)%8 != 0 {
			padNeeded := (8 - (len(destuffedFrame) % 8)) % 8
			if padNeeded > 0 {
				destuffedFrame += strings.Repeat("0", padNeeded)
			}
		}
	}
	frameDataBytes := BinaryStringToBytes(destuffedFrame)
	fullFrame := string(byte(0x0E)) + string(frameDataBytes) + string(byte(0x0E))

	return ParseFrame(fullFrame)
}

func (bs *BitStuffer) GetStuffedFrameInfo(p *Packet) string {
	addr := p.Address
	ctrl := p.Control
	fcs := p.FCS

	addressBinary := fmt.Sprintf("%08b", p.Address)
	controlBinary := fmt.Sprintf("%08b", p.Control)
	dataBinary := BytesToBinaryString(p.Data)
	fcsBinary := fmt.Sprintf("%08b", p.FCS)

	frameBinary := addressBinary + controlBinary + dataBinary + fcsBinary

	stuffedAddress := bs.Stuff(addressBinary)
	stuffedControl := bs.Stuff(controlBinary)
	stuffedData := bs.Stuff(dataBinary)
	stuffedFCS := bs.Stuff(fcsBinary)

	stuffedFrame := stuffedAddress + stuffedControl + stuffedData + stuffedFCS
	pad := (8 - (len(stuffedFrame) % 8)) % 8
	if pad > 0 {
		stuffedFrame += strings.Repeat("0", pad)
	}

	flagBinary := fmt.Sprintf("%08b", p.Flag)
	stuffedFull := flagBinary + stuffedFrame + flagBinary

	stuffedBytes := BinaryStringToBytes(stuffedFull)
	finalBinary := BytesToBinaryString(stuffedBytes)

	var origGroups []string
	originalWithFlags := flagBinary + frameBinary + flagBinary
	for i := 0; i < len(originalWithFlags); i += 8 {
		if i+8 <= len(originalWithFlags) {
			origGroups = append(origGroups, originalWithFlags[i:i+8])
		}
	}

	var stuffedGroups []string
	for i := 0; i < len(finalBinary); i += 8 {
		if i+8 <= len(finalBinary) {
			stuffedGroups = append(stuffedGroups, finalBinary[i:i+8])
		}
	}

	var md strings.Builder

	md.WriteString(fmt.Sprintf("**Flag:** `0x%02X` (%08b)\n\n", p.Flag, p.Flag))
	md.WriteString(fmt.Sprintf("**Sender's address:** %d (%08b)\n\n", addr, addr))
	md.WriteString(fmt.Sprintf("**Control:** %d (%08b)\n\n", ctrl, ctrl))
	md.WriteString(fmt.Sprintf("**Data:** %s\n\n", p.Data))
	md.WriteString(fmt.Sprintf("**FCS (Cyclic Code):** 0x%02X (%08b) - 8-bit CRC\n\n", fcs, fcs))

	md.WriteString("**Original frame:**\n\n```text\n")
	bytesPerLine := 16
	for i := 0; i < len(origGroups); i += bytesPerLine {
		end := i + bytesPerLine
		if end > len(origGroups) {
			end = len(origGroups)
		}
		md.WriteString(strings.Join(origGroups[i:end], " "))
		md.WriteString("\n")
	}
	md.WriteString("```\n\n")

	md.WriteString("**Packet after bit-stuffing:**\n\n```text\n")
	for i := 0; i < len(stuffedGroups); i += bytesPerLine {
		end := i + bytesPerLine
		if end > len(stuffedGroups) {
			end = len(stuffedGroups)
		}
		md.WriteString(strings.Join(stuffedGroups[i:end], " "))
		md.WriteString("\n")
	}
	md.WriteString("```\n\n")
	return md.String()
}

func (bs *BitStuffer) GetTransmissionInfo(original *Packet, corrupted *Packet) string {
	addr := original.Address
	ctrl := original.Control
	fcs := original.FCS

	originalDataBinary := BytesToBinaryString(original.Data)
	corruptedDataBinary := BytesToBinaryString(corrupted.Data)

	addressBinary := fmt.Sprintf("%08b", original.Address)
	controlBinary := fmt.Sprintf("%08b", original.Control)
	fcsBinary := fmt.Sprintf("%08b", original.FCS)

	stuffedAddress := bs.Stuff(addressBinary)
	stuffedControl := bs.Stuff(controlBinary)
	stuffedDataCorrupted := bs.Stuff(corruptedDataBinary)
	stuffedFCS := bs.Stuff(fcsBinary)

	stuffedFrame := stuffedAddress + stuffedControl + stuffedDataCorrupted + stuffedFCS
	pad := (8 - (len(stuffedFrame) % 8)) % 8
	if pad > 0 {
		stuffedFrame += strings.Repeat("0", pad)
	}

	flagBinary := fmt.Sprintf("%08b", original.Flag)
	stuffedFull := flagBinary + stuffedFrame + flagBinary

	stuffedBytes := BinaryStringToBytes(stuffedFull)
	finalBinary := BytesToBinaryString(stuffedBytes)

	var stuffedGroups []string
	for i := 0; i < len(finalBinary); i += 8 {
		if i+8 <= len(finalBinary) {
			stuffedGroups = append(stuffedGroups, finalBinary[i:i+8])
		}
	}

	var md strings.Builder

	md.WriteString(fmt.Sprintf("**Flag:** `0x%02X` (%08b)\n\n", original.Flag, original.Flag))
	md.WriteString(fmt.Sprintf("**Sender's address:** %d (%08b)\n\n", addr, addr))
	md.WriteString(fmt.Sprintf("**Control:** %d (%08b)\n\n", ctrl, ctrl))
	md.WriteString(fmt.Sprintf("**FCS:** 0x%02X (%08b) \n\n", fcs, fcs))

	//md.WriteString("**Original data (string):**\n\n")
	//md.WriteString("```text\n")
	//md.WriteString(original.Data + "\n")
	//md.WriteString("```\n\n")

	md.WriteString("**Original data:**\n\n")
	md.WriteString("```text\n")
	md.WriteString(groupBinary(originalDataBinary) + "\n")
	md.WriteString("```\n\n")

	//md.WriteString("**Corrupted data (string):**\n\n")
	//md.WriteString("```text\n")
	//md.WriteString(corrupted.Data + "\n")
	//md.WriteString("```\n\n")

	//md.WriteString("**Corrupted data (binary):**\n\n")
	//md.WriteString("```text\n")
	//md.WriteString(groupBinary(corruptedDataBinary) + "\n")
	//md.WriteString("```\n\n")

	preStuff := flagBinary + addressBinary + controlBinary + corruptedDataBinary + fcsBinary + flagBinary
	var preGroups []string
	for i := 0; i < len(preStuff); i += 8 {
		if i+8 <= len(preStuff) {
			preGroups = append(preGroups, preStuff[i:i+8])
		}
	}
	md.WriteString("**Original frame:**\n\n```text\n")
	bytesPerLine := 16
	for i := 0; i < len(preGroups); i += bytesPerLine {
		end := i + bytesPerLine
		if end > len(preGroups) {
			end = len(preGroups)
		}
		md.WriteString(strings.Join(preGroups[i:end], " "))
		md.WriteString("\n")
	}
	md.WriteString("```\n\n")

	md.WriteString("**Frame after bit-stuffing:**\n\n```text\n")
	for i := 0; i < len(stuffedGroups); i += bytesPerLine {
		end := i + bytesPerLine
		if end > len(stuffedGroups) {
			end = len(stuffedGroups)
		}
		md.WriteString(strings.Join(stuffedGroups[i:end], " "))
		md.WriteString("\n")
	}
	md.WriteString("```\n\n")

	return md.String()
}
