package packet

import (
	"fmt"
	"strings"
)

type BitStuffer struct{}

func NewBitStuffer() *BitStuffer {
	return &BitStuffer{}
}

func (bs *BitStuffer) Stuff(binaryData string) string {
	var result strings.Builder
	consecutiveOnes := 0

	for _, bit := range binaryData {
		if bit == '1' {
			consecutiveOnes++
			result.WriteRune(bit)

			if consecutiveOnes == 5 {
				result.WriteByte('0')
				consecutiveOnes = 0
			}
		} else {
			consecutiveOnes = 0
			result.WriteRune(bit)
		}
	}

	return result.String()
}

func (bs *BitStuffer) Destuff(stuffedData string) string {
	var result strings.Builder
	consecutiveOnes := 0
	i := 0

	for i < len(stuffedData) {
		bit := stuffedData[i]

		if bit == '1' {
			consecutiveOnes++
			result.WriteByte('1')
			i++
			continue
		}

		if consecutiveOnes == 5 {
			consecutiveOnes = 0
			i++
			continue
		}

		result.WriteByte('0')
		consecutiveOnes = 0
		i++
	}

	return result.String()
}

func (bs *BitStuffer) StuffPacket(p *Packet) string {
	frameData := p.GetFrameData()
	binaryData := BytesToBinaryString(frameData)

	stuffedBinary := bs.Stuff(binaryData)

	pad := (8 - (len(stuffedBinary) % 8)) % 8
	if pad > 0 {
		stuffedBinary += strings.Repeat("0", pad)
	}

	flagBinary := fmt.Sprintf("%08b", p.Flag)
	finalBinary := flagBinary + stuffedBinary + flagBinary

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

	frameData := BinaryStringToBytes(destuffedFrame)

	fullFrame := string(byte(0x0E)) + frameData + string(byte(0x0E))

	return ParseFrame(fullFrame)
}

func (bs *BitStuffer) GetStuffedFrameInfo(p *Packet) string {
	addr := p.Address
	ctrl := p.Control
	data := p.Data
	fcs := p.FCS

	frameData := p.GetFrameData()
	binaryData := BytesToBinaryString(frameData)
	stuffedBits := bs.Stuff(binaryData)

	var origGroups []string
	origPad := (8 - (len(binaryData) % 8)) % 8
	displayOrig := binaryData
	if origPad > 0 {
		displayOrig = binaryData + strings.Repeat("0", origPad)
	}
	for i := 0; i < len(displayOrig); i += 8 {
		origGroups = append(origGroups, displayOrig[i:i+8])
	}

	var stuffedGroups []string
	stuffedPad := (8 - (len(stuffedBits) % 8)) % 8
	displayStuffed := stuffedBits
	if stuffedPad > 0 {
		displayStuffed = stuffedBits + strings.Repeat("0", stuffedPad)
	}
	for i := 0; i < len(displayStuffed); i += 8 {
		stuffedGroups = append(stuffedGroups, displayStuffed[i:i+8])
	}

	var md strings.Builder
	md.WriteString("## Frame Analysis\n\n")

	md.WriteString(fmt.Sprintf("**Flag:** `0x%02X` (%08b)\n\n", p.Flag, p.Flag))
	md.WriteString(fmt.Sprintf("**Address:** %d (%08b)\n\n", addr, addr))
	md.WriteString(fmt.Sprintf("**Control:** %d (%08b)\n\n", ctrl, ctrl))
	md.WriteString(fmt.Sprintf("**Data (printable):** %s\n\n", data))
	md.WriteString(fmt.Sprintf("**FCS:** %d (%08b)\n\n", fcs, fcs))

	md.WriteString("**Original:**\n\n```text\n")
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

	md.WriteString("**After Bit-Stuffing:**\n\n```text\n")
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
