package packet

import (
	"math/rand"
	"time"
)

type CyclicCode struct {
	generator uint16
	fcsLength int
}

func NewCyclicCode() *CyclicCode {
	return &CyclicCode{
		generator: 0x107,
		fcsLength: 8,
	}
}

func (cc *CyclicCode) CalculateFCS(data string) uint8 {
	if len(data) == 0 {
		return 0
	}

	dataBinary := BytesToBinaryString(data)
	remainder := uint64(0)

	for i := 0; i < len(dataBinary); i++ {
		remainder = (remainder << 1) | uint64(dataBinary[i]-'0')

		if remainder >= (1 << 9) {
			remainder ^= uint64(cc.generator)
		}
	}

	return uint8(remainder)
}

func (cc *CyclicCode) VerifyFCS(data string, receivedFCS uint8) bool {
	calculatedFCS := cc.CalculateFCS(data)
	return calculatedFCS == receivedFCS
}

func (cc *CyclicCode) polynomialDivision(dividend uint64, dividendLength int) uint64 {
	generatorAligned := uint64(cc.generator) << (dividendLength - 9)
	remainder := dividend

	for i := 0; i < dividendLength-8; i++ {
		if remainder&(1<<(dividendLength-1-i)) != 0 {
			remainder ^= generatorAligned >> i
		}
	}

	return remainder & 0xFF
}

func (cc *CyclicCode) DetectErrors(data string, receivedFCS uint8) (bool, int, string) {
	calculatedFCS := cc.CalculateFCS(data)

	if calculatedFCS == receivedFCS {
		return false, 0, data
	}

	syndrome := calculatedFCS ^ receivedFCS

	correctedData, correctionSuccess := cc.correctSingleError(data, syndrome)
	if correctionSuccess {
		return true, 1, correctedData
	}

	return true, 2, data
}

func (cc *CyclicCode) correctSingleError(data string, syndrome uint8) (string, bool) {
	dataBinary := BytesToBinaryString(data)

	for i := 0; i < len(dataBinary); i++ {
		flippedData := []byte(dataBinary)
		if flippedData[i] == '0' {
			flippedData[i] = '1'
		} else {
			flippedData[i] = '0'
		}

		correctedDataStr := BinaryStringToBytes(string(flippedData))
		correctedFCS := cc.CalculateFCS(correctedDataStr)

		if correctedFCS == syndrome {
			return correctedDataStr, true
		}
	}

	return data, false
}

func (cc *CyclicCode) SimulateBitCorruption(data string) string {
	rand.Seed(time.Now().UnixNano())

	errorType := rand.Float32()
	dataBinary := []byte(BytesToBinaryString(data))

	if errorType < 0.25 {
		bitPos := rand.Intn(len(dataBinary))
		if dataBinary[bitPos] == '0' {
			dataBinary[bitPos] = '1'
		} else {
			dataBinary[bitPos] = '0'
		}
	} else {
		bitPos1 := rand.Intn(len(dataBinary))
		bitPos2 := rand.Intn(len(dataBinary))

		for bitPos2 == bitPos1 {
			bitPos2 = rand.Intn(len(dataBinary))
		}

		if dataBinary[bitPos1] == '0' {
			dataBinary[bitPos1] = '1'
		} else {
			dataBinary[bitPos1] = '0'
		}

		if dataBinary[bitPos2] == '0' {
			dataBinary[bitPos2] = '1'
		} else {
			dataBinary[bitPos2] = '0'
		}
	}

	return BinaryStringToBytes(string(dataBinary))
}

func (cc *CyclicCode) GetFCSLength() int {
	return cc.fcsLength
}
