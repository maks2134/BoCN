// packet/bitstuffer.go
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

// StuffPacket применяет бит-стафинг ко всем полям кроме флага
func (bs *BitStuffer) StuffPacket(p *Packet) string {
	// Применяем бит-стафинг к каждому полю отдельно (кроме флага)
	stuffedAddress := bs.Stuff(fmt.Sprintf("%08b", p.Address))
	stuffedControl := bs.Stuff(fmt.Sprintf("%08b", p.Control))
	stuffedData := bs.Stuff(BytesToBinaryString(p.Data))
	stuffedFCS := bs.Stuff(fmt.Sprintf("%08b", p.FCS))

	// Собираем все поля вместе (без флагов)
	stuffedFrame := stuffedAddress + stuffedControl + stuffedData + stuffedFCS

	// Выравниваем stuffedFrame до границы байта, затем добавляем флаги (не стафимые)
	pad := (8 - (len(stuffedFrame) % 8)) % 8
	if pad > 0 {
		stuffedFrame += strings.Repeat("0", pad)
	}

	flagBinary := fmt.Sprintf("%08b", p.Flag)
	finalBinary := flagBinary + stuffedFrame + flagBinary

	return BinaryStringToBytes(finalBinary)
}

// DestuffPacket снимает бит-стафинг со всех полей кроме флага
// DestuffPacket снимает бит-стафинг со всех полей кроме флага
func (bs *BitStuffer) DestuffPacket(receivedData string) *Packet {
	binaryData := BytesToBinaryString(receivedData)

	if len(binaryData) < 16 {
		return nil
	}

	// Извлекаем флаги (ожидаем, что они выровнены по байтам)
	startFlag := binaryData[0:8]
	endFlag := binaryData[len(binaryData)-8:]

	if startFlag != "00001110" || endFlag != "00001110" {
		return nil
	}

	// Получаем данные между флагами
	stuffedFrame := binaryData[8 : len(binaryData)-8]

	// Снимаем бит-стафинг только с полезных данных
	destuffedFrame := bs.Destuff(stuffedFrame)

	// Преобразуем обратно в байты
	frameDataBytes := BinaryStringToBytes(destuffedFrame)

	// Собираем полный фрейм с оригинальными флагами
	fullFrame := string(byte(0x0E)) + string(frameDataBytes) + string(byte(0x0E))

	return ParseFrame(fullFrame)
}

func (bs *BitStuffer) GetStuffedFrameInfo(p *Packet) string {
	addr := p.Address
	ctrl := p.Control
	fcs := p.FCS

	// Оригинальные данные каждого поля
	addressBinary := fmt.Sprintf("%08b", p.Address)
	controlBinary := fmt.Sprintf("%08b", p.Control)
	dataBinary := BytesToBinaryString(p.Data)
	fcsBinary := fmt.Sprintf("%08b", p.FCS)

	// Оригинальный полный пакет (без флагов)
	frameBinary := addressBinary + controlBinary + dataBinary + fcsBinary

	// Данные после стафинга для каждого поля
	stuffedAddress := bs.Stuff(addressBinary)
	stuffedControl := bs.Stuff(controlBinary)
	stuffedData := bs.Stuff(dataBinary)
	stuffedFCS := bs.Stuff(fcsBinary)

	// Полный пакет после стафинга (в правильном порядке)
	stuffedFrame := stuffedAddress + stuffedControl + stuffedData + stuffedFCS

	// Выравниваем stuffedFrame до границы байта, затем добавляем флаги (не стафимые)
	pad := (8 - (len(stuffedFrame) % 8)) % 8
	if pad > 0 {
		stuffedFrame += strings.Repeat("0", pad)
	}

	// Полный пакет с флагами
	flagBinary := fmt.Sprintf("%08b", p.Flag)
	stuffedFull := flagBinary + stuffedFrame + flagBinary

	// Конвертируем в байты и обратно для корректной группировки по байтам
	stuffedBytes := BinaryStringToBytes(stuffedFull)
	finalBinary := BytesToBinaryString(stuffedBytes)

	// Форматируем оригинальные данные для отображения
	var origGroups []string
	originalWithFlags := flagBinary + frameBinary + flagBinary
	for i := 0; i < len(originalWithFlags); i += 8 {
		if i+8 <= len(originalWithFlags) {
			origGroups = append(origGroups, originalWithFlags[i:i+8])
		}
	}

	// Форматируем данные после стафинга для отображения
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
	md.WriteString(fmt.Sprintf("**FCS:** %d (%08b)\n\n", fcs, fcs))

	md.WriteString("**Original packet:**\n\n```text\n")
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
