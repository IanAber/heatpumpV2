package PumpData

import (
	"log"
)

// 6 Coils, 9 Discretes, 16 Inputs, 4 Holding
// 129 coils, 28 Holding
type PumpData struct {
	Type          string   `json:"type"`
	Coil          []bool   `json:"coil"`
	Discrete      []bool   `json:"discrete"`
	Input         []uint16 `json:"input"`
	Holding       []uint16 `json:"holding"`
	coilStart     uint16
	discreteStart uint16
	inputStart    uint16
	holdingStart  uint16
	slaveAddress  uint8
}

func New(Type string, Coils uint16, CoilStart uint16, Discretes uint16, DiscreteStart uint16, Inputs uint16, InputsStart uint16, HoldingRegisters uint16, HoldingStart uint16, slave uint8) *PumpData {
	this := new(PumpData)
	this.Type = Type
	this.Coil = make([]bool, Coils)
	this.Discrete = make([]bool, Discretes)
	this.Input = make([]uint16, Inputs)
	this.Holding = make([]uint16, HoldingRegisters)
	this.coilStart = CoilStart
	this.discreteStart = DiscreteStart
	this.inputStart = InputsStart
	this.holdingStart = HoldingStart
	this.slaveAddress = slave
	return this
}

func (pumpData *PumpData) SlaveAddress() uint8 {
	return pumpData.slaveAddress
}

func (pumpData *PumpData) CoilStart() uint16 {
	return pumpData.coilStart
}

func (pumpData *PumpData) DiscreteStart() uint16 {
	return pumpData.discreteStart
}

func (pumpData *PumpData) InputStart() uint16 {
	return pumpData.inputStart
}

func (pumpData *PumpData) HoldingStart() uint16 {
	return pumpData.holdingStart
}

func (pumpData *PumpData) GetSpecs() (string, uint16, uint16, uint16, uint16, uint16, uint16, uint16, uint16, uint8) {
	return pumpData.Type, uint16(len(pumpData.Coil)), pumpData.coilStart, uint16(len(pumpData.Discrete)), pumpData.discreteStart, uint16(len(pumpData.Input)), pumpData.inputStart, uint16(len(pumpData.Holding)), pumpData.holdingStart, pumpData.SlaveAddress()
}

func (pumpData *PumpData) Compare(p1 *PumpData) bool {
	if pumpData.Type != p1.Type {
		return false
	}
	for n := range pumpData.Coil {
		if pumpData.Coil[n] != p1.Coil[n] {
			//			fmt.Println("Coil ", n, " changed from ", p1.Coil[n], " to ", pumpData.Coil[n])
			return false
		}
	}
	for n := range pumpData.Discrete {
		if pumpData.Discrete[n] != p1.Discrete[n] {
			//			fmt.Println("Discrete Input ", n, " changed from ", p1.Discrete[n], " to ", pumpData.Discrete[n])
			return false
		}
	}
	for n := range pumpData.Input {
		if pumpData.Input[n] != p1.Input[n] {
			//			fmt.Println("Input ", n, " changed from ", p1.Input[n], " to ", pumpData.Input[n])
			return false
		}
	}
	for n := range pumpData.Holding {
		if pumpData.Holding[n] != p1.Holding[n] {
			//			fmt.Println("Holding Register ", n, " changed from ", p1.Holding[n], " to ", pumpData.Holding[n])
			return false
		}
	}
	return true
}

func (pumpData *PumpData) Update(newData *PumpData) {
	if pumpData.Type != newData.Type {
		log.Panic("Trying to update pump data with data from the wrong type!")
	}
	copy(pumpData.Coil[:], newData.Coil[0:])
	copy(pumpData.Holding[:], newData.Holding[0:])
	copy(pumpData.Input[:], newData.Input[0:])
	copy(pumpData.Discrete[:], newData.Discrete[0:])
	pumpData.slaveAddress = newData.slaveAddress
}
