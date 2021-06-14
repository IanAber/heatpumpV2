package PumpController

import (
	"encoding/binary"
	"fmt"
	"github.com/goburrow/modbus"
	"log"
	"sync"
	"time"
)

type PumpController struct {
	rtuClient    *modbus.RTUClientHandler
	modbusClient modbus.Client
	mu           sync.Mutex
}

/**
*  Set up a new ModBus Client using the parameters given. No attempt is made to connect at this time.
 */
func New(rtuAddress string, baudRate int, dataBits int, stopBits int, parity string, timeout time.Duration, slaveID byte) *PumpController {
	this := new(PumpController)
	this.rtuClient = modbus.NewRTUClientHandler(rtuAddress)
	this.rtuClient.BaudRate = baudRate
	this.rtuClient.DataBits = dataBits
	this.rtuClient.Timeout = timeout
	this.rtuClient.Parity = parity
	this.rtuClient.StopBits = stopBits
	this.rtuClient.SlaveId = slaveID

	return this
}

func (pumpController *PumpController) Close() {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	if pumpController.rtuClient != nil {
		err := pumpController.rtuClient.Close()
		if err != nil {
			log.Println(err)
		}
	}
}

func (pumpController *PumpController) Connect() error {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	err := pumpController.rtuClient.Connect()
	if err != nil {
		return err
	}
	pumpController.modbusClient = modbus.NewClient(pumpController.rtuClient)
	return nil
}

func (pumpController *PumpController) readCoil(coil uint16, slaveId uint8) (bool, error) {
	pumpController.rtuClient.SlaveId = slaveId
	data, err := pumpController.modbusClient.ReadCoils(coil, 1)
	if err != nil {
		return false, err
	} else {
		if len(data) != 1 {
			return false, fmt.Errorf("read coil %d returned %d bytes when 1 was expected", coil, len(data))
		} else {
			return data[0] != 0, nil
		}
	}
}

func (pumpController *PumpController) ReadCoil(coil uint16, slaveId uint8) (bool, error) {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	return pumpController.readCoil(coil, slaveId)
}

func (pumpController *PumpController) WriteCoil(coil uint16, value bool, slaveId uint8) error {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	pumpController.rtuClient.SlaveId = slaveId
	var err error
	if value {
		_, err = pumpController.modbusClient.WriteSingleCoil(coil, 0xFF00)
	} else {
		_, err = pumpController.modbusClient.WriteSingleCoil(coil, 0x0000)
	}
	return err
}

func (pumpController *PumpController) readHoldingRegister(register uint16, slaveId uint8) (uint16, error) {
	pumpController.rtuClient.SlaveId = slaveId
	data, err := pumpController.modbusClient.ReadHoldingRegisters(register, 1)
	if err != nil {
		return 0, err
	} else {
		if len(data) != 2 {
			return 0, fmt.Errorf("read holding register %d returned %d bytes when 2 were expected", register, len(data))
		} else {
			return binary.BigEndian.Uint16(data), nil
		}
	}
}

func (pumpController *PumpController) ReadHoldingRegister(holdingRegister uint16, slaveId uint8) (uint16, error) {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	return pumpController.readHoldingRegister(holdingRegister, slaveId)
}

func (pumpController *PumpController) readHoldingRegisterDiv10(register uint16, slaveId uint8) (float32, error) {
	pumpController.rtuClient.SlaveId = slaveId
	data, err := pumpController.modbusClient.ReadHoldingRegisters(register, 1)
	if err != nil {
		return 0, err
	} else {
		if len(data) != 2 {
			return 0, fmt.Errorf("read holding register %d returned %d bytes when 2 were expected", register, len(data))
		} else {
			return float32(binary.BigEndian.Uint16(data)) / 10, nil
		}
	}
}

func (pumpController *PumpController) WriteHoldingRegister(register uint16, value uint16, slaveId uint8) error {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	pumpController.rtuClient.SlaveId = slaveId
	_, err := pumpController.modbusClient.WriteSingleRegister(register, value)
	return err
}

func (pumpController *PumpController) writeHoldingRegisterFloat(register uint16, value float32, slaveId uint8) error {
	pumpController.rtuClient.SlaveId = slaveId
	_, err := pumpController.modbusClient.WriteSingleRegister(register, uint16(value*10))
	return err
}

func (pumpController *PumpController) readInputRegister(register uint16, slaveId uint8) (uint16, error) {
	pumpController.rtuClient.SlaveId = slaveId
	data, err := pumpController.modbusClient.ReadInputRegisters(register, 1)
	if err != nil {
		fmt.Println("Register = ", register, " Error = ", err)
		return 0, err
	} else {
		if len(data) != 2 {
			return 0, fmt.Errorf("read input register %d returned %d bytes when 2 were expected", register, len(data))
		} else {
			return binary.BigEndian.Uint16(data), nil
		}
	}
}

func (pumpController *PumpController) ReadInputRegister(holdingRegister uint16, slaveId uint8) (uint16, error) {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	return pumpController.readInputRegister(holdingRegister, slaveId)
}

func (pumpController *PumpController) readDiscreteInput(input uint16, slaveId uint8) (bool, error) {
	pumpController.rtuClient.SlaveId = slaveId
	data, err := pumpController.modbusClient.ReadDiscreteInputs(input, 1)
	if err != nil {
		return false, err
	} else {
		if len(data) != 1 {
			return false, fmt.Errorf("read discrete input %d returned %d bytes when 1 was expected", input, len(data))
		} else {
			return data[0] != 0, nil
		}
	}
}

func convertBitsToBools(byteData []byte, length uint16) []bool {
	boolData := make([]bool, length)
	for i, b := range byteData {
		for bit := 0; bit < 8; bit++ {
			boolIndex := uint16((i * 8) + bit)
			if boolIndex < length {
				boolData[(i*8)+bit] = (b & 1) != 0
			}
			b >>= 1
		}
	}
	return boolData
}

func (pumpController *PumpController) ReadMultipleDiscreteRegisters(start uint16, count uint16, slaveId uint8) ([]bool, error) {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	pumpController.rtuClient.SlaveId = slaveId
	mbData, err := pumpController.modbusClient.ReadDiscreteInputs(start, count)
	if err != nil {
		return make([]bool, count), err
	} else {
		return convertBitsToBools(mbData, count), err
	}
}

func (pumpController *PumpController) ReadMultipleCoils(start uint16, count uint16, slaveId uint8) ([]bool, error) {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	pumpController.rtuClient.SlaveId = slaveId
	mbData, err := pumpController.modbusClient.ReadCoils(start, count)
	if err != nil {
		return make([]bool, count), err
	} else {
		return convertBitsToBools(mbData, count), err
	}
}

func convertBytesToWords(byteData []byte) []uint16 {
	wordData := make([]uint16, len(byteData)/2)
	for i := range wordData {
		wordData[i] = binary.BigEndian.Uint16(byteData[i*2:])
	}
	return wordData
}

func (pumpController *PumpController) ReadMultipleInputRegisters(start uint16, count uint16, slaveId uint8) ([]uint16, error) {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	pumpController.rtuClient.SlaveId = slaveId
	mbData, err := pumpController.modbusClient.ReadInputRegisters(start, count)
	return convertBytesToWords(mbData), err
}

func (pumpController *PumpController) ReadMultipleHoldingRegisters(start uint16, count uint16, slaveId uint8) ([]uint16, error) {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	pumpController.rtuClient.SlaveId = slaveId
	mbData, err := pumpController.modbusClient.ReadHoldingRegisters(start, count)
	return convertBytesToWords(mbData), err
}

func (pumpController *PumpController) ReadDiscreteInput(input uint16, slaveId uint8) (bool, error) {
	pumpController.mu.Lock()
	defer pumpController.mu.Unlock()
	return pumpController.readCoil(input, slaveId)
}
