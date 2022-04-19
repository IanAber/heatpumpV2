package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	PumpData "heatpump/heatPump"
	PumpController "heatpump/pumpController"
	websocket "heatpump/webSocket"
	"log"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

var CommsPort = flag.String("Port", "/dev/tty.usbserial-1424430", "communication port for the Heat Pump")
var BaudRate = flag.Int("baudrate", 19200, "communication port baud rate for the Heat Pump")
var DataBits = flag.Int("databits", 8, "communication port data bits for the Heat Pump")
var StopBits = flag.Int("stopbits", 2, "communication port stop bits for the Heat Pump")
var Parity = flag.String("parity", "N", "communication port parity for the Heat Pump")
var TimeoutSecs = flag.Int("timeout", 5, "communication port timeout in seconds for the Heat Pump")
var hpSlaveAddress = flag.Int("hpslave", 1, "Modbus slave ID for the Heat Pump")
var pumpSlaveAddress = flag.Int("pumpslave", 10, "Modbus slave ID for the Pump Controller")
var mbus *PumpController.PumpController
var webPort = flag.String("webport", "8085", "port number for the WEB interface")

//var db sql.DB
var stmt *sql.Stmt
var lastAlarmTime time.Time
var lastFailureTime time.Time
var cycleInverterPower bool
var lastFlowAlarm time.Time
var cyclePumps bool
var pool *websocket.Pool

const (
	Coil = iota
	Discrete
	InputRegister
	HoldingRegister
	Blank // Allows blank entries to be placed
)

var lastPumpData *PumpData.PumpData
var lastHeatPumpData *PumpData.PumpData

type ModbusEndPoint struct {
	id         string
	name       string
	address    uint16
	dataType   byte
	multiplier int
	units      string
	writeable  bool
}

const HotPump = 1
const ColdFlow = 2
const ColdPump = 2
const RejectFlow = 3
const RejectPump = 3
const InverterContactor = 4

const InTemp = 1
const OutTemp = 2
const BmsOn = 17
const SetpointCold = 13

const InverterPowerAlarm = 138
const WATERFLOWSWITCHALARM = 53
const MAINWATERPUMP = 7
const WATERFLOWSWITCH = 1
const ALARMRESET = 16
const BMSONOFF = 17

const InverterStatus = 24
const MotorCurrent = 25

var hpEndPoints = []ModbusEndPoint{
	{"hph13", "Cooling Set Point", SetpointCold, HoldingRegister, 10, "℃", true},
	{"hph1", "Chiller IN Temperature", InTemp, HoldingRegister, 10, "℃", false},
	{"hph2", "Chiller Out Temperature", OutTemp, HoldingRegister, 10, "℃", false},
	{"", "", 0, Blank, 0, "", false},
	{"hph26", "Motor Voltage", 26, HoldingRegister, 1, "V", false},
	{"hph25", "Motor Current", 25, HoldingRegister, 10, "A", false},
	{"hph23", "Motor Rotate Speed", 23, HoldingRegister, 10, "RPS", false},
	{"", "", 0, Blank, 0, "", false},
	{"hpc16", "Alarm Reset", 16, Coil, 0, "", true},
	{"hpc17", "BMS On Off", BmsOn, Coil, 0, "", true},
	{"hpc13", "Unit Start", 13, Coil, 0, "", false},
	{"hpc1", "Water Flow Switch", 1, Coil, 0, "", false},
	{"hpc2", "Remote OnOff", 2, Coil, 0, "", false},
	{"hpc3", "Teminal Switch", 3, Coil, 0, "", false},
	{"hpc4", "High Fan", 4, Coil, 0, "", false},
	{"hpc5", "Low Fan", 5, Coil, 0, "", false},
	{"hpc6", "Four Way Valve", 6, Coil, 0, "", false},
	{"hpc7", "Main Water Pump", 7, Coil, 0, "", false},
	{"hpc8", "Three Port Valve For Water Circuit", 8, Coil, 0, "", false},
	{"hpc9", "Crankshaft Heater", 9, Coil, 0, "", false},
	{"hpc10", "Chassis Heater", 10, Coil, 0, "", false},
	{"hpc11", "End Terminal Water Pump", 11, Coil, 0, "", false},
	{"hpc12", "Electric Heater", 12, Coil, 0, "", false},
	{"hpc14", "Cooling Mode", 14, Coil, 0, "", false},
	{"hpc15", "Heating Mode", 15, Coil, 0, "", false},
	{"hpc18", "Suction Pressure Probe Alarm", 18, Coil, 0, "", false},
	{"hpc19", "Suction Temperature Probe Alarm", 19, Coil, 0, "", false},
	{"hpc20", "Exhaust Pressure Probe Alarm", 20, Coil, 0, "", false},
	{"hpc21", "Exhaust Temperature Probe Alarm", 21, Coil, 0, "", false},
	{"hpc22", "Main Valve Low Super Heat Alarm", 22, Coil, 0, "", false},
	{"hpc23", "Main Valve LOP Alarm", 23, Coil, 0, "", false},
	{"hpc24", "Main Valve MOP Alarm", 24, Coil, 0, "", false},
	{"hpc25", "Main Valve LowSuction Tempertaure", 25, Coil, 0, "", false},
	{"hpc26", "Main Valve Adaptation PID Error", 26, Coil, 0, "", false},
	{"hpc27", "Main Valve Range Error", 27, Coil, 0, "", false},
	{"hpc28", "Main Valve High Condensing Temperature", 28, Coil, 0, "", false},
	{"hpc29", "Main Valve Motor Failure", 29, Coil, 0, "", false},
	{"hpc30", "Main Valve Emergency Shutdown Alarm", 30, Coil, 0, "", false},
	{"hpc31", "Main Valve Sequence Error", 31, Coil, 0, "", false},
	{"hpc32", "Main Valve Position Signal Error", 32, Coil, 0, "", false},
	{"hpc33", "Auxiliary Valve Low SuperheatAlarm", 33, Coil, 0, "", false},
	{"hpc34", "Auxiliary Valve LOP Alarm", 34, Coil, 0, "", false},
	{"hpc35", "Auxiliary Valve MOP Alarm", 35, Coil, 0, "", false},
	{"hpc36", "Auxiliary Valve Range Error", 36, Coil, 0, "", false},
	{"hpc37", "Auxiliary Valve High Condensing Temperature", 37, Coil, 0, "", false},
	{"hpc38", "Auxiliary Valve Low Suction Temperature", 38, Coil, 0, "", false},
	{"hpc39", "Auxiliary Valve Motor Error", 39, Coil, 0, "", false},
	{"hpc40", "Auxiliary Valve Adaptive PIDError", 40, Coil, 0, "", false},
	{"hpc41", "Auxiliary Valve Emergency Shutdown Alarm", 41, Coil, 0, "", false},
	{"hpc42", "Auxiliary Valve Sequence Error", 42, Coil, 0, "", false},
	{"hpc43", "Auxiliary Valve Position Signal Error", 43, Coil, 0, "", false},
	{"hpc44", "Water In temperature Probe Alarm", 44, Coil, 0, "", false},
	{"hpc45", "Water Out Temperature Probe Alarm", 45, Coil, 0, "", false},
	{"hpc46", "Ambient Temperature Probe Alarm", 46, Coil, 0, "", false},
	{"hpc47", "Coil temperature probe alarm", 47, Coil, 0, "", false},
	{"hpc48", "Enthalpy Suction Temperature Probe Alarm", 48, Coil, 0, "", false},
	{"hpc49", "Enthalpy Suction Pressure Probe Alarm", 49, Coil, 0, "", false},
	{"hpc50", "Storage Type Variables Frequently Written", 50, Coil, 0, "", false},
	{"hpc51", "Storage Variable Write Error", 51, Coil, 0, "", false},
	{"hpc52", "PumpActive", 52, Coil, 0, "", false},
	{"hpc53", "WaterFlowSwitchAlarm", 53, Coil, 0, "", false},
	{"hpc54", "HighPressureAlarm", 54, Coil, 0, "", false},
	{"hpc55", "LowPressureAlarm", 55, Coil, 0, "", false},
	{"hpc56", "WaterOutTemperatureTooHighAlarm", 56, Coil, 0, "", false},
	{"hpc57", "WaterOutTemperatureTooLowAlarm", 57, Coil, 0, "", false},
	{"hpc58", "WaterInOutDeltaTemperature", 58, Coil, 0, "", false},
	{"hpc59", "BLDC-Starting pressure difference is too high", 59, Coil, 0, "", false},
	{"hpc60", "BLDC Compressor Off", 60, Coil, 0, "", false},
	{"hpc61", "BLDC Out Of Operation Range", 61, Coil, 0, "", false},
	{"hpc62", "BLDCCompressorStartupFailureRetry", 62, Coil, 0, "", false},
	{"hpc63", "BLDCCompressorStartupFailureLock", 63, Coil, 0, "", false},
	{"hpc64", "BLDCLowPressureDeltaDifference", 64, Coil, 0, "", false},
	{"hpc65", "BLDCExhaustTemperatureTooHigh", 65, Coil, 0, "", false},
	{"hpc66", "EnvelopeHighPressureRatio", 66, Coil, 0, "", false},
	{"hpc67", "EnvelopeExhaustPressureHigh", 67, Coil, 0, "", false},
	{"hpc68", "EnvelopeElectricCurrentHigh", 68, Coil, 0, "", false},
	{"hpc69", "EnvelopeSuctionPressureHigh", 69, Coil, 0, "", false},
	{"hpc70", "EnvelopeLowPressureRatio", 70, Coil, 0, "", false},
	{"hpc71", "Envelope Low Pressure Delta Difference", 71, Coil, 0, "", false},
	{"hpc72", "Envelope Exhaust Pressure Low", 72, Coil, 0, "", false},
	{"hpc73", "Envelope Suction Pressure Low", 73, Coil, 0, "", false},
	{"hpc74", "Envelope Exhaust Temperature High", 74, Coil, 0, "", false},
	{"hpc75", "Over Current", 75, Coil, 0, "", false},
	{"hpc76", "Motor Overload", 76, Coil, 0, "", false},
	{"hpc77", "DC Bus Over Voltage", 77, Coil, 0, "", false},
	{"hpc78", "DC Bus Under Voltage", 78, Coil, 0, "", false},
	{"hpc79", "Inverter Overheating", 79, Coil, 0, "", false},
	{"hpc80", "Inverter Under Temperature", 80, Coil, 0, "", false},
	{"hpc81", "OverCurrent HW", 81, Coil, 0, "", false},
	{"hpc82", "Motor Overheat", 82, Coil, 0, "", false},
	{"hpc83", "IGBT Module Failure", 83, Coil, 0, "", false},
	{"hpc84", "CPU Failure", 84, Coil, 0, "", false},
	{"hpc85", "Parameter Missing", 85, Coil, 0, "", false},
	{"hpc86", "Bus Voltage Fluctuation", 86, Coil, 0, "", false},
	{"hpc87", "DataCommunicationFailure", 87, Coil, 0, "", false},
	{"hpc88", "ThermistorFailure", 88, Coil, 0, "", false},
	{"hpc89", "AutomaticAdjustmentFailure", 89, Coil, 0, "", false},
	{"hpc90", "InverterDisabled", 90, Coil, 0, "", false},
	{"hpc91", "MotorPhaseSequenceFailure", 91, Coil, 0, "", false},
	{"hpc92", "FanFailure", 92, Coil, 0, "", false},
	{"hpc93", "SpeedFailure", 93, Coil, 0, "", false},
	{"hpc94", "PFCModuleFailure", 94, Coil, 0, "", false},
	{"hpc95", "PFCOvervoltage", 95, Coil, 0, "", false},
	{"hpc96", "PFCUndervoltage", 96, Coil, 0, "", false},
	{"hpc97", "STODetectionError_1", 97, Coil, 0, "", false},
	{"hpc98", "STODetectionError_2", 98, Coil, 0, "", false},
	{"hpc99", "GroundFault", 99, Coil, 0, "", false},
	{"hpc100", "CPUSynchronizationError1", 100, Coil, 0, "", false},
	{"hpc101", "CPUSynchronizationError2", 101, Coil, 0, "", false},
	{"hpc102", "InverterOverload", 102, Coil, 0, "", false},
	{"hpc103", "UCsafetyFault", 103, Coil, 0, "", false},
	{"hpc104", "UnexpectedRestart", 104, Coil, 0, "", false},
	{"hpc105", "UnexpectedStop", 105, Coil, 0, "", false},
	{"hpc106", "Current Measurement Fault", 106, Coil, 0, "", false},
	{"hpc107", "Current Unbalanced", 107, Coil, 0, "", false},
	{"hpc108", "Over Current Safety", 108, Coil, 0, "", false},
	{"hpc109", "STO Alarm", 109, Coil, 0, "", false},
	{"hpc110", "STO Hardware Alarm", 110, Coil, 0, "", false},
	{"hpc111", "Power Supply Missing", 111, Coil, 0, "", false},
	{"hpc112", "HW Fault Comand Buffer", 112, Coil, 0, "", false},
	{"hpc113", "HW Fault Heater", 113, Coil, 0, "", false},
	{"hpc114", "Data Communication Fault", 114, Coil, 0, "", false},
	{"hpc115", "Compressor Stall Detect", 115, Coil, 0, "", false},
	{"hpc116", "DC bus Over Current", 116, Coil, 0, "", false},
	{"hpc117", "HWF DC bus Current", 117, Coil, 0, "", false},
	{"hpc118", "DC bus voltage", 118, Coil, 0, "", false},
	{"hpc119", "HWF DC bus voltage", 119, Coil, 0, "", false},
	{"hpc120", "Input Voltage", 120, Coil, 0, "", false},
	{"hpc121", "HWFInputVoltage", 121, Coil, 0, "", false},
	{"hpc122", "DCbusPowerAlarm", 122, Coil, 0, "", false},
	{"hpc123", "HWFPowerMismatch", 123, Coil, 0, "", false},
	{"hpc124", "NTCOverTemperature", 124, Coil, 0, "", false},
	{"hpc125", "NTCUnderTemperature", 125, Coil, 0, "", false},
	{"hpc126", "NTCFault", 126, Coil, 0, "", false},
	{"hpc127", "HWFSyncFault", 127, Coil, 0, "", false},
	{"hpc128", "InvalidParameter", 128, Coil, 0, "", false},
	{"hpc129", "FWFault", 129, Coil, 0, "", false},
	{"hpc130", "HWFault", 130, Coil, 0, "", false},
	{"hpc131", "PowerAndSafetyReserved1", 131, Coil, 0, "", false},
	{"hpc132", "PowerAndSafetyReserved2", 132, Coil, 0, "", false},
	{"hpc133", "PowerAndSafetyReserved3", 133, Coil, 0, "", false},
	{"hpc134", "PowerAndSafetyReserved4", 134, Coil, 0, "", false},
	{"hpc135", "PowerAndSafetyReserved5", 135, Coil, 0, "", false},
	{"hpc136", "PowerAndSafetyReserved6", 136, Coil, 0, "", false},
	{"hpc137", "PowerAndSafetyReserved7", 137, Coil, 0, "", false},
	{"hpc138", "InverterOfflineAlarm", 138, Coil, 0, "", false},
	{"", "", 0, Blank, 0, "", false},
	{"", "", 0, Blank, 0, "", false},
	{"hph3", "Ambient Temperature", 3, HoldingRegister, 10, "℃", false},
	{"hph4", "Suction Temperature EVI", 4, HoldingRegister, 10, "℃", false},
	{"hph5", "Condensor Coil Temperature", 5, HoldingRegister, 10, "℃", false},
	{"hph6", "Suction Pressure EVI", 6, HoldingRegister, 10, "Bar", false},
	{"hph7", "Suction Temperature", 7, HoldingRegister, 10, "℃", false},
	{"hph8", "Suction Pressure", 8, HoldingRegister, 10, "Bar", false},
	{"hph9", "Discharge Temperature", 9, HoldingRegister, 10, "℃", false},
	{"hph10", "Discharge Pressure", 10, HoldingRegister, 10, "Bar", false},
	{"hph11", "Pump Aout", 11, HoldingRegister, 1, "", false},
	{"hph12", "Unit Status", 12, HoldingRegister, 1, "", false},
	{"hph14", "Heating Set Point", 14, HoldingRegister, 10, "℃", true},
	{"hph15", "Cool/Heat Mode Selection", 15, HoldingRegister, 1, "", false},
	{"hph16", "Compressor Demand", 16, HoldingRegister, 10, "%", false},
	{"hph17", "Main Valve Superheat", 17, HoldingRegister, 10, "℃", false},
	{"hph18", "Main Valve Opening Steps", 18, HoldingRegister, 1, "", false},
	{"hph19", "Main Valve Opening Percent", 19, HoldingRegister, 10, "%", false},
	{"hph20", "Auxilliary Valve Superheat", 20, HoldingRegister, 10, "℃", false},
	{"hph21", "Auxilliary Valve Opening Steps", 21, HoldingRegister, 1, "", false},
	{"hph22", "Auxilliary Valve Opening Percent", 22, HoldingRegister, 10, "%", false},
	{"hph24", "Inverter Status", 24, HoldingRegister, 1, "", false},
	{"hph27", "Inverter Temperature", 27, HoldingRegister, 10, "℃", false},
	{"hph28", "Bus Voltage", 28, HoldingRegister, 1, "V", false},
}

var pumpEndPoints = []ModbusEndPoint{
	{"pd1", "Digital Input 1", 1, Discrete, 0, "", false},
	{"pd2", "Cold Flow", ColdFlow, Discrete, 0, "", false},
	{"pd3", "Reject Flow", RejectFlow, Discrete, 0, "", false},
	{"pd4", "Digital Input 4", 4, Discrete, 0, "", false},

	{"pc1", "Hot Pump", HotPump, Coil, 0, "", true},
	{"pc2", "Cold Pump", ColdPump, Coil, 0, "", true},
	{"pc3", "Reject Pump", RejectPump, Coil, 0, "", true},
	{"pc4", "Inverter Contactor", InverterContactor, Coil, 0, "", true},

	{"pc5", "Relay 5", 5, Coil, 0, "", true},
	{"pc6", "Relay 6", 6, Coil, 0, "", true},
	{"pc7", "Output 7", 7, Coil, 0, "", true},
	{"pc8", "Output 8", 8, Coil, 0, "", true},

	{"pi1", "Chilli Hot Pump", 1, InputRegister, 1, "", false},
	{"pi2", "Chillii Cold Pump", 2, InputRegister, 1, "", false},
	{"pi3", "Chillii Reject Pump", 3, InputRegister, 1, "", false},
	{"pi4", "Analog Input 4", 4, InputRegister, 1, "", false},

	{"pi5", "Analog Input 5", 5, InputRegister, 1, "", false},
	{"pi6", "Analog Input 6", 6, InputRegister, 1, "", false},
	{"pi7", "Analog Input 7", 7, InputRegister, 1, "", false},
	{"pi8", "Analog Input 8", 8, InputRegister, 1, "", false},

	{"pi12", "Reject Temp. In", 12, InputRegister, 10, "C", false},
	{"pi13", "Reject Temp. Out", 13, InputRegister, 10, "C", false},
	{"pi14", "Temperature 3", 14, InputRegister, 10, "C", false},
	{"pi15", "Temperature 4", 15, InputRegister, 10, "C", false},

	{"pi16", "Temperature 5", 16, InputRegister, 10, "C", false},
	{"", "", 0, Blank, 0, "", false},
	{"", "", 0, Blank, 0, "", false},
	{"", "", 0, Blank, 0, "", false},

	{"pi9", "Actual Hot Pump", 9, InputRegister, 1, "", false},
	{"pi10", "Actual Cold Pump", 10, InputRegister, 1, "", false},
	{"pi11", "Actual Reject Pump", 11, InputRegister, 1, "", false},
	{"", "", 0, Blank, 0, "", false},

	{"ph2", "Hot Pump Setting", 2, HoldingRegister, 1, "", true},
	{"ph3", "Cold Pump Setting", 3, HoldingRegister, 1, "", true},
	{"ph4", "Reject Pump Setting", 4, HoldingRegister, 1, "", true},
	{"", "", 0, Blank, 0, "", false},

	{"ph1", "Analog Out 0", 1, HoldingRegister, 1, "", true},
	{"ph5", "Slave ID", 5, HoldingRegister, 1, "", true},
	{"ph6", "Baud Rate", 6, HoldingRegister, 1, "", true},
	{"", "", 0, Blank, 0, "", false},
}

func webControlHeatPump(w http.ResponseWriter, r *http.Request) {

	var payload struct {
		HeatPump bool `json:"heatpump"`
	}
	err := json.NewDecoder(r.Body).Decode(&payload)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if payload.HeatPump {
		webStartHeatPump(w, r)
	} else {
		webStopHeatPump(w, r)
	}
}

func webToggleCoil(_ http.ResponseWriter, r *http.Request) {
	address := r.FormValue("coil")

	fmt.Println("Writing to ", address)
	if strings.HasPrefix(address, "hpc") {
		// heat pump coil
		n, _ := strconv.ParseUint(address[3:], 10, 16)

		nIndex := uint16(n)
		nIndex = nIndex - lastHeatPumpData.CoilStart() // nIndex is now 0 based
		if lastHeatPumpData.Coil[nIndex] {
			log.Println("Setting coil ", nIndex, " to false")
		} else {
			log.Println("Setting coil ", nIndex, " to true")
		}
		err := mbus.WriteCoil(nIndex+lastHeatPumpData.CoilStart(), !lastHeatPumpData.Coil[nIndex], lastHeatPumpData.SlaveAddress())
		if err != nil {
			log.Println(err)
		}
		//		getValues(true, lastHeatPumpData, mbus)
	} else if strings.HasPrefix(address, "pc") {
		// pump controller coil
		nIndex, _ := strconv.ParseUint(address[2:], 10, 16)
		fmt.Println("Write ", !lastPumpData.Coil[nIndex-1], " to ", lastPumpData.SlaveAddress(), ":", nIndex)
		err := mbus.WriteCoil(uint16(nIndex), !lastPumpData.Coil[nIndex-1], lastPumpData.SlaveAddress())
		if err != nil {
			log.Println(err)
		}
		//		getValues(true, lastPumpData, mbus)
	}
}

func getHeatPumpDisposition() (rejectPumpOn bool, coldPumpOn bool, heatPumpOn bool, err error) {
	rejectPump, err := mbus.ReadHoldingRegister(4, lastPumpData.SlaveAddress())
	if err != nil {
		return
	}
	rejectPumpOn = rejectPump > 99
	coldPump, err := mbus.ReadHoldingRegister(3, lastPumpData.SlaveAddress())
	if err != nil {
		return
	}
	coldPumpOn = coldPump > 99
	heatPumpOn, err = mbus.ReadCoil(17, lastHeatPumpData.SlaveAddress())
	if err != nil {
		return
	}
	return
}

/**
Attempt to turn off the cold and reject pumps by setting their override values to 0.
If either pump is on we set them both to zero then wait 5 seconds and check the status again.
If the status of either pump is still ON then try again and keep trying until both go off.
*/
func killPumps(delay time.Duration) {
	time.Sleep(delay)

	for pumpsActive := true; pumpsActive; {
		if !lastPumpData.Coil[1] && !lastPumpData.Coil[2] {
			pumpsActive = false
			log.Println("Pumps are not active.")
		} else {
			// Check the heat pump isn't still running
			if !lastHeatPumpData.Coil[MAINWATERPUMP] {
				log.Println("Stopping pumps")
				err := mbus.WriteHoldingRegister(4, 0, lastPumpData.SlaveAddress())
				if err != nil {
					log.Print(err)
				}
				err = mbus.WriteHoldingRegister(3, 0, lastPumpData.SlaveAddress())
				if err != nil {
					log.Print(err)
				}
			}
			// Wait 5 seconds then loop until the pumps are all stopped.
			time.Sleep(time.Second * 5)
		}
	}
}

func stopHeatPump() error {
	rejectPump, coldPump, heatPump, err := getHeatPumpDisposition()
	if err != nil {
		log.Println("Error getting heatpump disposition : ", err)
		return err
	}

	if heatPump {
		err = mbus.WriteCoil(17, false, lastHeatPumpData.SlaveAddress())
		if err != nil {
			log.Print("Error turning the heat pump off", err)
			return err
		}
	}
	if rejectPump || coldPump {
		// Wait 15 seconds before stopping the other pumps
		killPumps(15 * time.Second)
	}
	return nil
}

func webStopHeatPump(w http.ResponseWriter, _ *http.Request) {

	rejectPump, coldPump, heatPump, err := getHeatPumpDisposition()
	if err != nil {
		_, err = fmt.Fprintf(w, `{"request":"GetHeatPumpDisposition","status":"ERROR","error":"%s"}`, err)
		if err != nil {
			log.Print(err)
		}
		return
	}

	if heatPump {
		err = mbus.WriteCoil(17, false, lastHeatPumpData.SlaveAddress())
		if err != nil {
			_, err = fmt.Fprintf(w, `{"request":"TurnOffHeatPump","status":"ERROR","error":"%s"}`, err)
			if err != nil {
				log.Print(err)
			}
			return
		}
	}
	if rejectPump || coldPump {
		// Wait 15 seconds before stopping the other pumps
		go killPumps(15 * time.Second)
	}
	_, err = fmt.Fprintf(w, `{"status":"OK","description":"HeatPump Stopped"}`)
	if err != nil {
		log.Print(err)
	}
}

func webChangeHeatPumpSetpoint(w http.ResponseWriter, r *http.Request) {
	newValue, err := strconv.ParseFloat(r.FormValue("setpoint"), 32)
	if err != nil {
		_, err = fmt.Fprintf(w, `{"request":"SetCoolingSetpoint","status":"ERROR","error":"%s"}`, err)
		if err != nil {
			log.Print(err)
		}
		return
	}

	newValue = newValue * 10
	err = mbus.WriteHoldingRegister(13, uint16(newValue), lastHeatPumpData.SlaveAddress())
	if err != nil {
		_, err = fmt.Fprintf(w, `{"request":"SetCoolingSetpoint","status":"ERROR","error":"%s"}`, err)
		if err != nil {
			log.Print(err)
		}
		return
	} else {
		_, err = fmt.Fprint(w, `{"status":"OK"}`)
		if err != nil {
			log.Print(err)
		}
	}
}

func startHeatPump() {
	for loops := 0; loops < 10; loops++ {
		if lastPumpData.Discrete[ColdFlow-1] || lastPumpData.Discrete[RejectFlow-1] {
			time.Sleep(time.Second * 15)
		} else {
			err := mbus.WriteCoil(BMSONOFF, true, lastHeatPumpData.SlaveAddress())
			if err != nil {
				log.Printf(`{"request":"TurnOnHeatPump","status":"ERROR","error":"%s"}`, err)
			} else {
				return
			}
		}
	}
	log.Println("Timed out waiting for the pumps to start up. Heat Pump was not started.")
}

func webStartHeatPump(w http.ResponseWriter, _ *http.Request) {

	rejectPump, coldPump, heatPump, err := getHeatPumpDisposition()
	if err != nil {
		_, err = fmt.Fprintf(w, `{"request":"GetHeatPumpDisposition","status":"ERROR","error":"%s"}`, err)
		if err != nil {
			log.Print(err)
		}
		return
	}

	if heatPump {
		_, err = fmt.Fprint(w, `{"status":"OK", "description":"HeatPump is already started"}`)
		if err != nil {
			log.Print(err)
		}
		return
	}

	if !rejectPump {
		err = mbus.WriteHoldingRegister(4, 100, lastPumpData.SlaveAddress())
		if err != nil {
			_, err = fmt.Fprintf(w, `{"request":"TurnOnRejectPump","status":"ERROR","error":"%s"}`, err)
			if err != nil {
				log.Print(err)
			}
			return
		}
		time.Sleep(time.Second)
	}

	if !coldPump {
		err = mbus.WriteHoldingRegister(3, 100, lastPumpData.SlaveAddress())
		if err != nil {
			_, err = fmt.Fprintf(w, `{"request":"TurnOnColdPump","status":"ERROR","error":"%s"}`, err)
			if err != nil {
				log.Print(err)
			}
			return
		}
		time.Sleep(time.Second)
	}
	go startHeatPump()
	_, err = fmt.Fprintf(w, `{"status":"OK", "description":"HeatPump Starting"}`)
	if err != nil {
		log.Print(err)
	}
}

/**
Report the current status to Home Assistant
*/
func webGetStatus(w http.ResponseWriter, _ *http.Request) {
	var data struct {
		On         bool    `json:"on"`
		Setpoint   float32 `json:"setpoint"`
		InTemp     float32 `json:"in_temperature"`
		OutTemp    float32 `json:"out_temperature"`
		Speed      uint16  `json:"speed"`
		Current    float32 `json:"current"`
		Voltage    float32 `json:"voltage"`
		ColdPump   bool    `json:"cold_pump"`
		ColdFlow   bool    `json:"cold_flow"`
		RejectPump bool    `json:"reject_pump"`
		RejectFlow bool    `json:"reject_flow"`
		Alarm      bool    `json:"alarm"`
	}
	data.On = lastHeatPumpData.Coil[16]
	data.Setpoint = float32(lastHeatPumpData.Holding[12]) / 10.0
	data.InTemp = float32(lastHeatPumpData.Holding[0]) / 10.0
	data.OutTemp = float32(lastHeatPumpData.Holding[1]) / 10.0
	data.Speed = lastHeatPumpData.Holding[22]
	data.Current = float32(lastHeatPumpData.Holding[24]) / 10.0
	data.Voltage = float32(lastHeatPumpData.Holding[25])
	data.ColdPump = lastPumpData.Coil[1]
	data.ColdFlow = !lastPumpData.Discrete[1]
	data.RejectPump = lastPumpData.Coil[2]
	data.RejectFlow = !lastPumpData.Discrete[2]
	s, err := json.Marshal(data)
	if err != nil {
		_, err = fmt.Fprint(w, err)
		if err != nil {
			log.Print(err)
		}
	} else {
		_, err = fmt.Fprint(w, string(s))
		if err != nil {
			log.Print(err)
		}
	}
}

func webProcessHoldingRegistersForm(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()

	if err != nil {
		_, err = fmt.Fprint(w, `<html><head><title>Error</title></head><body><h1>`, err, `</h1></body></html>`)
		if err != nil {
			log.Println(err)
		}
	}
	for sKey, sValue := range r.Form {
		fmt.Println("Key:", sKey, " value:", sValue)
		nValue, _ := strconv.ParseUint(sValue[0], 10, 16)
		if strings.HasPrefix(sKey, "hph") {
			// Heat pump update
			nIndex, _ := strconv.ParseUint(sKey[3:], 10, 16)
			_, err = fmt.Println("Writing ", nValue, " to heat pump register ", nIndex)
			if err != nil {
				log.Print(err)
			}
			err = mbus.WriteHoldingRegister(uint16(nIndex), uint16(nValue), uint8(*hpSlaveAddress))
			if err != nil {
				log.Println("Error writing to heat pump holding register - ", err)
			}
			//			getValues(true, lastHeatPumpData, mbus)
		} else if strings.HasPrefix(sKey, "ph") {
			// Pump controler update
			nIndex, _ := strconv.ParseUint(sKey[2:], 10, 16)
			_, err = fmt.Println("Writing ", nValue, " to pump controller register ", nIndex)
			if err != nil {
				log.Print(err)
			}
			err = mbus.WriteHoldingRegister(uint16(nIndex), uint16(nValue), uint8(*pumpSlaveAddress))
			if err != nil {
				log.Println("Error writing to pump controller holding register ", nIndex, " value ", nValue, " - ", err)
			}
			// We need to space out the writes so the Arduino has time to update its EEPROM if required between individual holding registers.
			// One alternative would be to send them as a bulk write but there are other potential issues there too. This 100mS delay seems to be sufficient.
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func startDataWebSocket(pool *websocket.Pool, w http.ResponseWriter, r *http.Request) {
	fmt.Println("WebSocket Endpoint Hit")
	conn, err := websocket.Upgrade(w, r)
	if err != nil {
		_, err = fmt.Fprintf(w, "%+v\n", err)
		if err != nil {
			log.Println(err)
		}
	}

	client := &websocket.Client{
		Conn: conn,
		Pool: pool,
	}

	pool.Register <- client
	fmt.Println("New client so force an update.")
	getValues(true, lastPumpData, mbus)
	getValues(true, lastHeatPumpData, mbus)
}

func getValues(bRefresh bool, lastValues *PumpData.PumpData, p *PumpController.PumpController) {
	if bRefresh {
		fmt.Println("Forced client refresh.")
	}
	var source string
	if lastValues.Type == "hp" {
		source = "heat pump"
	} else {
		source = "pump controller"
	}
	newValues := PumpData.New(lastValues.GetSpecs())
	if len(newValues.Discrete) > 0 {
		mbData, err := p.ReadMultipleDiscreteRegisters(newValues.DiscreteStart(), uint16(len(newValues.Discrete)), newValues.SlaveAddress())
		if err != nil {
			log.Println("Error getting ", source, " discrete inputs from slave ID ", lastValues.SlaveAddress(), " - ", err)
			return
		}
		copy(newValues.Discrete[:], mbData)
	}
	if len(newValues.Coil) > 0 {
		mbData, err := p.ReadMultipleCoils(newValues.CoilStart(), uint16(len(newValues.Coil)), newValues.SlaveAddress())
		if err != nil {
			log.Println("Error getting ", source, " coils - ", err)
			return
		}
		copy(newValues.Coil[:], mbData)
	}
	if len(newValues.Holding) > 0 {
		//		fmt.Println("Holding registers starting at ", newValues.HoldingStart(), " for ", uint16(len(newValues.Holding)), " values")
		mbUintData, err := p.ReadMultipleHoldingRegisters(newValues.HoldingStart(), uint16(len(newValues.Holding)), newValues.SlaveAddress())
		if err != nil {
			log.Println("Error getting ", source, " holding registers - ", err)
			return
		}
		copy(newValues.Holding[:], mbUintData)
	}
	if len(newValues.Input) > 0 {
		mbUintData, err := p.ReadMultipleInputRegisters(newValues.InputStart(), uint16(len(newValues.Input)), newValues.SlaveAddress())
		if err != nil {
			log.Println("Error getting ", source, " input registers - ", err)
			return
		}
		copy(newValues.Input[:], mbUintData)
	}
	if bRefresh || !newValues.Compare(lastValues) {
		lastValues.Update(newValues)
		bytes, err := json.Marshal(lastValues)
		if err != nil {
			log.Print("Error marshalling the data - ", err)
		} else {
			select {
			case pool.Broadcast <- bytes:
			default:
				fmt.Println("Channel would block!")
			}
		}
	}
}

func getAlarms(values *PumpData.PumpData) string {
	var Alarms = ""
	for _, ep := range hpEndPoints {
		if ep.dataType == Coil {
			if ep.address > 17 {
				if values.Coil[ep.address-1] {
					if Alarms != "" {
						Alarms += " : "
					}
					Alarms += ep.name
				}
			}
		}
	}
	return Alarms
}

func logValues(values *PumpData.PumpData, pumpValues *PumpData.PumpData) {

	// Only log data if the heat pump is switched on.
	//	if !values.Coil[16] {
	//		return
	//	}
	//var Alarms = ""
	//
	//for _, ep := range hpEndPoints {
	//	if ep.dataType == Coil {
	//		if ep.address > 17 {
	//			if values.Coil[ep.address-1] {
	//				if Alarms != "" {
	//					Alarms += " : "
	//				}
	//				Alarms += ep.name
	//			}
	//		}
	//	}
	//}
	_, err := stmt.Exec(values.Holding[0], values.Holding[1], values.Holding[2], values.Holding[3], values.Holding[4],
		values.Holding[5], values.Holding[6], values.Holding[7], values.Holding[8], values.Holding[9],
		values.Holding[10], values.Holding[11], values.Holding[12], values.Holding[13], values.Holding[14],
		values.Holding[15], values.Holding[16], values.Holding[17], values.Holding[18], values.Holding[19],
		values.Holding[20], values.Holding[21], values.Holding[22], values.Holding[23], values.Holding[24],
		values.Holding[25], values.Holding[26], values.Holding[27], pumpValues.Input[11], pumpValues.Input[12],
		values.Coil[0], values.Coil[1], values.Coil[2], values.Coil[3], values.Coil[4], values.Coil[5],
		values.Coil[6], values.Coil[7], values.Coil[8], values.Coil[9], values.Coil[10], values.Coil[11],
		values.Coil[12], getAlarms(values), pumpValues.Input[3], pumpValues.Input[4], pumpValues.Input[7])
	if err != nil {
		log.Println(err)
	}
}

// CyclePowerToInverter
/**
Cycle the power to the inverter by switching on the first relay on the pump controller. This switches on the power control
Relay on the heat pump and powers down the inverter module since it is connected through the N/C contacts. Wait 90 seconds then
power it back up. It should come back on line withing 60 seconds which will clear the alarm
*/
func CyclePowerToInverter() {
	err := mbus.WriteCoil(4, true, lastPumpData.SlaveAddress())
	if err != nil {
		log.Println(err)
	}
	if err = stopHeatPump(); err != nil {
		log.Println("Failed to stop the heat pump : ", err)
		return
	}
	time.Sleep(120 * time.Second)
	err = mbus.WriteCoil(4, false, lastPumpData.SlaveAddress())
	if err != nil {
		log.Println(err)
	}
	time.Sleep(60 * time.Second)
	// The alarm should be clear now. Email me if it isn't
	if lastHeatPumpData.Coil[InverterPowerAlarm-1] {
		err := smtp.SendMail("mail.cedartechnology.com:587",
			smtp.PlainAuth("", "pi@cedartechnology.com", "7444561", "mail.cedartechnology.com"),
			"pi@cedartechnology.com", []string{"ian.abercrombie@cedartechnology.com"}, []byte(`From: HeatPump
To: Ian.Abercrombie@CedarTechnology.com
Subject: Heat Pump Failure
The heat pump inverter has gone offline and attempting recover by cycling the power to it did not bring it back!`))
		if err != nil {
			log.Println("Failed to send email about heatpump inverter offline recovery failure. - ", err)
		}
	} else {
		err := smtp.SendMail("mail.cedartechnology.com:587",
			smtp.PlainAuth("", "pi@cedartechnology.com", "7444561", "mail.cedartechnology.com"),
			"pi@cedartechnology.com", []string{"ian.abercrombie@cedartechnology.com"}, []byte(`From: HeatPump
To: Ian.Abercrombie@CedarTechnology.com
Subject: Heat Pump Inverter Power Cycld
The heat pump inverter has gone offline and I am cycling the power to it to try to bring it back!`))
		if err != nil {
			log.Println("Failed to send email about heatpump inverter offline recovery failure. - ", err)
		}
	}
	startHeatPump()
	cycleInverterPower = false
}

func CyclePumps() {
	err := mbus.WriteCoil(2, false, lastPumpData.SlaveAddress())
	if err != nil {
		log.Println(err)
	}
	err = mbus.WriteCoil(3, false, lastPumpData.SlaveAddress())
	if err != nil {
		log.Println(err)
	}
	time.Sleep(time.Second)
	err = mbus.WriteCoil(2, true, lastPumpData.SlaveAddress())
	if err != nil {
		log.Println(err)
	}
	err = mbus.WriteCoil(3, true, lastPumpData.SlaveAddress())
	if err != nil {
		log.Println(err)
	}
	// Give the pumps 30 seconds to get running again then reset the alarm code on the heat pump
	time.AfterFunc(time.Second*30, func() {
		err := mbus.WriteCoil(ALARMRESET, true, lastHeatPumpData.SlaveAddress())
		if err != nil {
			log.Print(err)
		}
	})
}

func reportValues() {
	var ticks int8 = 0
	ticker := time.NewTicker(time.Second)
	quit := make(chan struct{})
	log.Print("starting the data logger")

	for {
		select {
		case <-ticker.C:
			//			log.Println("Getting pump data - ticks = ", ticks)
			getValues(false, lastPumpData, mbus)
			//			log.Println("Getting heatpump data")
			getValues(false, lastHeatPumpData, mbus)
			if lastPumpData.Discrete[ColdFlow-1] || lastPumpData.Discrete[RejectFlow-1] {
				_, _, heatPump, err := getHeatPumpDisposition()
				if err != nil {
					log.Println("Error getting heatpump disposition : ", err)
					err = mbus.WriteCoil(17, false, lastHeatPumpData.SlaveAddress())
					if err != nil {
						log.Print("Error turning the heat pump off", err)
					}
				} else {
					if heatPump {
						log.Println("--ERROR!-- Heat pump is on but circulators are not showing adequate flow. Stopping the heat pump")
						err = mbus.WriteCoil(17, false, lastHeatPumpData.SlaveAddress())
						if err != nil {
							log.Print("Error turning the heat pump off", err)
						}
					}
				}
			}
			ticks++
			if ticks > 4 {
				//				log.Println("Logging heatpump values.")
				logValues(lastHeatPumpData, lastPumpData)
				ticks = 0
			}
			if !lastHeatPumpData.Coil[InverterPowerAlarm-1] {
				// No power alarm so clear the flag and reset the time
				lastAlarmTime = time.Unix(0, 0)
				//				cycleInverterPower = false
			} else {
				// If the time is '0' then record the current time as when the alrm was first seen
				if lastAlarmTime == time.Unix(0, 0) {
					lastAlarmTime = time.Now()
				}
				// If the alarm was first seen more than 1 minute ago then cycle the power to the inverter.
				if (time.Since(lastAlarmTime) > (1 * time.Minute)) && !cycleInverterPower {
					// Power down the inverter for 90 seconds
					cycleInverterPower = true
					go CyclePowerToInverter()
				}
			}
			if ((lastHeatPumpData.Holding[InverterStatus-1] == 1) && (lastHeatPumpData.Holding[MotorCurrent-1] > 0)) || lastHeatPumpData.Holding[InverterStatus-1] == 0 {
				lastFailureTime = time.Unix(0, 0)
				cycleInverterPower = false
			} else {
				if lastFailureTime == time.Unix(0, 0) {
					lastFailureTime = time.Now()
				}
				if (time.Since(lastFailureTime) > (3 * time.Minute)) && !cycleInverterPower {
					cycleInverterPower = true
					go CyclePowerToInverter()
				}
			}
			if !lastHeatPumpData.Coil[WATERFLOWSWITCHALARM-1] { // Water flow switch alarm
				lastFlowAlarm = time.Unix(0, 0)
				cyclePumps = false
			} else {
				log.Print("flow switch error")
				if lastFlowAlarm == time.Unix(0, 0) {
					lastFlowAlarm = time.Now()
				}
				if time.Since(lastFlowAlarm) > (1 * time.Minute) {
					if !cyclePumps {
						log.Print("cycle pumps")
						cyclePumps = true
						go CyclePumps()
					} else {
						if lastHeatPumpData.Coil[WATERFLOWSWITCH-1] {
							log.Print("reset alarm")
							err := mbus.WriteCoil(ALARMRESET, true, lastHeatPumpData.SlaveAddress())
							if err != nil {
								log.Println(err)
							}
						}
					}
				}
			}
		case <-quit:
			ticker.Stop()
			return
		}
	}
}

// HeatPumpData
/**
Returns a JSON object with key values in for use by WEB Service consumers
*/
type HeatPumpData struct {
	ColdPump   bool    `json:"cold_pump"`
	ColdFlow   bool    `json:"cold_flow"`
	RejectPump bool    `json:"reject_pump"`
	RejectFlow bool    `json:"reject_flow"`
	HeatPumpOn bool    `json:"heatpump_on"`
	SetPoint   float64 `json:"setpoint"`
	InTemp     float64 `json:"in_temp"`
	OutTemp    float64 `json:"out_temp"`
}

func webGetData(w http.ResponseWriter, _ *http.Request) {
	var data HeatPumpData

	data.ColdPump = lastPumpData.Coil[ColdPump-1]
	data.RejectPump = lastPumpData.Coil[RejectPump-1]
	data.ColdFlow = lastPumpData.Discrete[ColdFlow-1]
	data.RejectFlow = lastPumpData.Discrete[RejectFlow-1]
	data.HeatPumpOn = lastHeatPumpData.Coil[BmsOn-1]
	data.InTemp = float64(lastHeatPumpData.Holding[InTemp-1]) / 10
	data.OutTemp = float64(lastHeatPumpData.Holding[OutTemp-1]) / 10
	data.SetPoint = float64(lastHeatPumpData.Holding[SetpointCold-1]) / 10

	strData, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		_, err = fmt.Fprint(w, `{"error":"`, err, `"}`)
		if err != nil {
			log.Print(err)
		}
	} else {
		_, err = fmt.Fprint(w, string(strData))
		if err != nil {
			log.Print(err)
		}
	}
}

/**
Returns the main WEB page used to control the system.
*/

func webGetValues(w http.ResponseWriter, _ *http.Request) {

	_, err := fmt.Fprint(w, `<html>
  <head>
    <link rel="shortcut icon" href="">
    <title>Heat Pump V2</title>
    <style>
      table{border-width:2px;border-style:solid}
      td.coilOff{border-width:1px;border-style:solid;text-align:center;background-color:darkgreen;color:white}
      td.coilOn{border-width:1px;border-style:solid;text-align:center;background-color:red;color:white}
      td.discreteOff{border-width:1px;border-style:solid;text-align:center;border:1px solid darkgreen}
      td.discreteOn{border-width:1px;border-style:solid;text-align:center;border:1px solid red}
      span.coilOff{color:white}
	  span.coilOn{color:white}
      span.discreteOn{color:red;font-weight:bold}
	  span.discreteOff{color:darkgreen;font-weight:bold}
      td.holdingRegister{border-width:1px;border-style:solid;text-align:right}
      td.inputRegister{border-width:1px;border-style:solid;text-align:right}
      input.holdingRegister{padding: 6px 1px;margin: 3px 0;width: 50px; box-sizing: border-box;border: 1px solid blue;background-color: #5CADEC;color: white;}
      input.inputRegister{padding: 6px 1px;margin: 3px 0;width: 50px; box-sizing: border-box;border: none;background-color: gainsboro;color: black;}
      label{padding: 6px; margin: 3px}
      label.readWrite{background-color:#68A068; color:white}
      button {
        font-family: Arial, Helvetica, sans-serif;
        font-size: 23px;
        color: #edfcfa;
        padding: 10px 20px;
        background: -moz-linear-gradient( top, #ffffff 0%, #6e5fd9 50%, #3673cf 50%, #000794);
        background: -webkit-gradient( linear, left top, left bottom, from(#ffffff), color-stop(0.50, #6e5fd9), color-stop(0.50, #3673cf), to(#000794));
        -moz-border-radius: 14px;
        -webkit-border-radius: 14px;
        border-radius: 14px;
        border: 1px solid #004d80;
        -moz-box-shadow:
        0px 1px 3px rgba(000, 000, 000, 0.5), inset 0px 0px 2px rgba(255, 255, 255, 1);
        -webkit-box-shadow:
        0px 1px 3px rgba(000, 000, 000, 0.5), inset 0px 0px 2px rgba(255, 255, 255, 1);
        box-shadow:
        0px 1px 3px rgba(000, 000, 000, 0.5), inset 0px 0px 2px rgba(255, 255, 255, 1);
        text-shadow:
        0px -1px 0px rgba(000, 000, 000, 0.2), 0px 1px 0px rgba(255, 255, 255, 0.4);
      }


    </style>
	<script type="text/javascript">
		var stopFunctionTimeout = 0;
		(function() {
			var url = "ws://" + window.location.host + "/ws";
			var conn = new WebSocket(url);
			conn.onclose = function(evt) {
				alert('Connection closed');
			}
			conn.onmessage = function(evt) {
				data = JSON.parse(evt.data);
				if(data.type == "p") {
					data.discrete.forEach(setPumpsDiscrete);
					data.coil.forEach(setPumpsCoil);
					data.holding.forEach(setPumpsHoldingReg);
					data.input.forEach(setPumpsInputReg);
				} else if(data.type == "hp") {
					data.discrete.forEach(setHPDiscrete);
					data.coil.forEach(setHPCoil);
					data.holding.forEach(setHPHoldingReg);
					data.input.forEach(setHPInputReg);
					if (document.getElementById("hpc13").className == "coilOn") {
						document.getElementById("hpOnOff").innerText = "Stop Heat Pump";
					} else {
						document.getElementById("hpOnOff").innerText = "Start Heat Pump";
					}
				}
			}
		})();
		function setPumpsCoil(item, index) {
			var control = document.getElementById("pc" + (index + 1));
			if (item) {
				control.className = "coilOn";
				control.parentElement.className = "coilOn";
			} else {
				control.className = "coilOff";
				control.parentElement.className = "coilOff";
			}
		}

		function setPumpsDiscrete(item, index) {
			var control = document.getElementById("pd" + (index + 1));
			if (item) {
				control.className = "discreteOn";
				control.parentElement.className = "discreteOn";
			} else {
				control.className = "discreteOff";
				control.parentElement.className = "discreteOff";
			}
		}

		function setPumpsHoldingReg(item, index) {
			control = document.getElementById("ph" + (index + 1));
			if((document.activeElement.id == null) || (document.activeElement.id != control.id)) {
				control.value = item / control.attributes["multiplier"].value;
			}
		}

		function setPumpsInputReg(item, index) {
			control = document.getElementById("pi" + (index + 1));
			control.value = item / control.attributes["multiplier"].value;
		}

		function setHPCoil(item, index) {
			var control = document.getElementById("hpc" + (index + 1));
			if (item) {
				control.className = "coilOn";
				control.parentElement.className = "coilOn";
			} else {
				control.className = "coilOff";
				control.parentElement.className = "coilOff";
			}
		}

		function setHPDiscrete(item, index) {
			var control = document.getElementById("hpd" + (index + 1));
			if (item) {
				control.className = "discreteOn";
				control.parentElement.className = "discreteOn";
			} else {
				control.className = "discreteOff";
				control.parentElement.className = "discreteOff";
			}
		}

		function setHPHoldingReg(item, index) {
			control = document.getElementById("hph" + (index + 1));
			if((document.activeElement.id == null) || (document.activeElement.id != control.id)) {
				control.value = item / control.attributes["multiplier"].value;
			}
		}

		function setHPInputReg(item, index) {
			control = document.getElementById("hpi" + (index + 1));
			control.value = item / control.attributes["multiplier"].value;
		}

		function clickCoil(id) {
			var xhr = new XMLHttpRequest();
			xhr.open('PATCH','toggleCoil?coil=' + id);
			xhr.send();
		}

		function getElementVal(control) {
			var v = control.value;
			var m = control.attributes["multiplier"].value;
			if(isNaN(m)) {
				return v;
			} else {
				return v * m;
			}
		}

		function startPumps() {
			document.getElementById("ph3").value = 100;
			document.getElementById("ph4").value = 100;
			sendFormData('pumpForm', 'setHoldingRegisters');
		}

		function stopPumps() {
			// Don't allow the pumps to stop if the heat pump is still running.'
			if (document.getElementById("hpc13").className == "coilOn") {
				alert("Stop the heat pump first!");
			} else {
				document.getElementById("ph3").value = 0;
				document.getElementById("ph4").value = 0;
				sendFormData('pumpForm', 'setHoldingRegisters');
			}
		}

		function startHeatPump() {
			var on = document.getElementById("hpc13").className == "coilOn";
			if (stopFunctionTimeout != 0) {
				clearTimeout(stopFunctionTimeout);	// Clear any previous timeout to stop the pumps
			}
			if(!on && ((document.getElementById("ph3").value != 100) || (document.getElementById("ph4").value != 100))) {
				startPumps();	// If the heat pump is not on and either pump is not running then start the pumps
			}
			if(on) {
				// Set a timeout to stop the pumps in 15 seconds which should be enough for the heat pump to stop.
				stopFunctionTimeout = setTimeout(stopPumps, 15000);
			}
			// Toggle the heat pump switch
			clickCoil('hpc17');
		}

		function sendFormData(form, url) {
			var urlEncode = function(data, rfc3986) {
				if (typeof rfc3986 === 'undefined') {
					rfc3986 = true;
				}
				// Encode value
				data = encodeURIComponent(data);
				data = data.replace(/%20/g, '+');
				// RFC 3986 compatibility
				if (rfc3986) {
					data = data.replace(/[!'()*]/g, function(c) {
						return '%' + c.charCodeAt(0).toString(16);
					});
				}
				return data;
			}
			form = document.getElementById(form);

			var frmDta = "";
			for (var i=0; i < form.elements.length; ++i) {
				if (form.elements[i].name != ''){
					if (frmDta.length != 0) {
						frmDta = frmDta + "&";
					}
					frmDta = frmDta + urlEncode(form.elements[i].name) + '=' + urlEncode(getElementVal(form.elements[i]));
				}
			}
			var xhr = new XMLHttpRequest();
			xhr.open('POST', url, true);
			xhr.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");
			xhr.send(frmDta);
		}
		function resetAlarm() {
			clickCoil('hpc16');
		}
	</script>
  </head>
  <body>
	<h1>Pumps Controller</h1>
    <h2>Connected on `, *CommsPort, ` at `, *BaudRate, ` baud at slave address `, *pumpSlaveAddress, `</h2>
    <div id="pumps">
      <form onsubmit="return false;" id="pumpForm">
        <table class="pumps"><tr><td colspan=2 style="text-align:center">---Key---</td><td class="coilOn">===ON===</td><td class="coilOff">===OFF===</td></tr>'`)
	if err != nil {
		log.Println(err)
	}
	var bClosed bool
	var onClick string
	var labelClass string
	var readOnly string
	var name string
	nIndex := 0
	for _, ep := range pumpEndPoints {
		if (nIndex % 4) == 0 {
			_, err := fmt.Fprint(w, `
          <tr>`)
			if err != nil {
				log.Println(err)
			}
			bClosed = false
		}
		if ep.writeable {
			onClick = `onclick="clickCoil('` + ep.id + `')"`
			labelClass = `class="readWrite"`
			readOnly = ``
			name = ` name="` + ep.id + `"`
		} else {
			onClick = ""
			labelClass = ""
			readOnly = `readonly`
			name = ``

		}
		switch ep.dataType {
		case Coil:
			_, err = fmt.Fprintf(w, `<td class="coil" %s><span class="coilOff" id="%s">%s</span></td>`, onClick, ep.id, ep.name)
		case Discrete:
			_, err = fmt.Fprintf(w, `<td class="discrete"><span class="discreteOff" id="%s">%s</span></td>`, ep.id, ep.name)
		case HoldingRegister:
			_, err = fmt.Fprint(w, `<td class="holdingRegister"><label for="`, ep.id, `" `, labelClass, `>`, ep.name, `</label><input class="holdingRegister" type="text"`, name, ` id="`, ep.id, `" multiplier="`, ep.multiplier, `" value="" `, readOnly, `></td>`)
		case InputRegister:
			_, err = fmt.Fprint(w, `<td class="inputRegister"><label for="`, ep.id, `">`, ep.name, `</label `, labelClass, `><input class="inputRegister" type="text" id="`, ep.id, `" multiplier="`, ep.multiplier, `" value="" readonly></td>`)
		case Blank:
			_, err = fmt.Fprint(w, `<td>&nbsp;</td>`)
		}
		if err != nil {
			log.Println(err)
		}
		nIndex++
		if (nIndex % 4) == 0 {
			_, err = fmt.Fprint(w, `</tr>`)
			if err != nil {
				log.Println(err)
			}
			bClosed = true
		}
	}
	if !bClosed {
		_, err = fmt.Fprint(w, "</tr>")
		if err != nil {
			log.Println(err)
		}
	}
	_, err = fmt.Fprint(w, `
        </table>
        <br />
			<button class="frmSubmit" type="button" onclick="sendFormData('pumpForm', 'setHoldingRegisters')">Submit Changes</button>&nbsp;&nbsp;
			<button class="frmSubmit" type="button" onclick="startPumps()">Start Pumps</button>&nbsp;&nbsp;
			<button class="frmSubmit" type="button" onclick="stopPumps()">Stop Pumps</button>
		</form><br />
	</div>
	<h1>HeatPump</h1>
    <h2>Connected on `, *CommsPort, ` at `, *BaudRate, ` baud at slave address `, *hpSlaveAddress, `</h2>
    <div id="hpcoils">
      <form onsubmit="return false;" id="heatPumpForm">
        <button class="frmSubmit" type="button" onclick="sendFormData('heatPumpForm', 'setHoldingRegisters')">Submit Changes</button>&nbsp;&nbsp;
		<button class="frmSubmit" type="button" id="hpOnOff" onclick="startHeatPump()">Start Heat Pump</button>&nbsp;&nbsp;
		<button class="alarmResset" type="button" id="alarmReset" onclick="resetAlarm()">Reset Alarm</button><br />
        <table class="heatpump">`)
	if err != nil {
		log.Println(err)
	}
	nIndex = 0
	for _, ep := range hpEndPoints {
		if (nIndex % 4) == 0 {
			_, err = fmt.Fprint(w, `
          <tr>`)
			if err != nil {
				log.Println(err)
			}
			bClosed = false
		}
		if ep.writeable {
			onClick = `onclick="clickCoil('` + ep.id + `')"`
			labelClass = `class="readWrite"`
			readOnly = ""
			name = ` name="` + ep.id + `"`
		} else {
			onClick = ""
			labelClass = ""
			readOnly = "readonly"
			name = ``
		}
		switch ep.dataType {
		case Coil:
			_, err = fmt.Fprintf(w, `<td class="coil" %s><span class="coilOff" id="%s">%s</span></td>`, onClick, ep.id, ep.name)
			//			fmt.Fprintf(w, `<td class="coil"><span class="coilOff" id="%s" %s>%s</span></td>`, ep.id, onClick, ep.name)
		case Discrete:
			_, err = fmt.Fprintf(w, `<td class="discrete"><span class="discreteOff" id="%s" %s>%s</span></td>`, ep.id, onClick, ep.name)
		case HoldingRegister:
			_, err = fmt.Fprint(w, `<td class="holdingRegister"><label for="`, ep.id, `" `, labelClass, `>`, ep.name, `</label><input class="holdingRegister" type="text"`, name, ` id="`, ep.id, `" multiplier="`, ep.multiplier, `" value="" `, readOnly, `></td>`)
		case InputRegister:
			_, err = fmt.Fprint(w, `<td class="inputRegister"><label for="`, ep.id, `">`, ep.name, `</label><input class="inputRegister" type="text" id="`, ep.id, `" multiplier="`, ep.multiplier, `" value="" readonly></td>`)
		case Blank:
			_, err = fmt.Fprint(w, `<td>&nbsp;</td>`)
		}
		if err != nil {
			log.Println(err)
		}
		nIndex++
		if (nIndex % 4) == 0 {
			_, err = fmt.Fprint(w, `</tr>`)
			if err != nil {
				log.Println(err)
			}
			bClosed = true
		}
	}
	if !bClosed {
		_, err = fmt.Fprint(w, "</tr>")
		if err != nil {
			log.Println(err)
		}
	}
	_, err = fmt.Fprint(w, `
        </table>
      </form>
    </div>
  </body>
</html>`)
	if err != nil {
		log.Println(err)
	}
}

func setUpWebSite() {
	pool = websocket.NewPool()
	go pool.Start()

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", webGetValues).Methods("GET")
	router.HandleFunc("/toggleCoil", webToggleCoil).Methods("PATCH")
	router.HandleFunc("/setHoldingRegisters", webProcessHoldingRegistersForm).Methods("POST")
	router.HandleFunc("/startHeatPump", webStartHeatPump).Methods("PATCH")
	router.HandleFunc("/stopHeatPump", webStopHeatPump).Methods("PATCH")
	router.HandleFunc("/setHeatPump", webChangeHeatPumpSetpoint).Methods("PATCH")
	router.HandleFunc("/status", webGetStatus).Methods("GET")
	router.HandleFunc("/controlHeatPump", webControlHeatPump).Methods("POST")
	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		startDataWebSocket(pool, w, r)
	}).Methods("GET")
	router.HandleFunc("/getData", webGetData).Methods("GET")
	var sWebAddress = ":" + *webPort
	log.Fatal(http.ListenAndServe(sWebAddress, router))
}

func init() {
	flag.Parse()
	lastPumpData = PumpData.New("p", 8, 1, 4, 1, 16, 1, 6, 1, uint8(*pumpSlaveAddress))
	lastHeatPumpData = PumpData.New("hp", 138, 1, 0, 1, 0, 1, 28, 1, uint8(*hpSlaveAddress))

}

func closedb(db *sql.DB) {
	closeErr := db.Close()
	if closeErr != nil {
		log.Print(closeErr)
	}
}
func main() {

	db, err := sql.Open("mysql", "pi:7444561@tcp(localhost:3306)/logging")
	// defer the close till after the main function has finished executing
	if err != nil {
		log.Println(err)
	} else {
		defer closedb(db)
	}

	// if there is an error opening the connection, handle it
	if err != nil {
		log.Println(err.Error())
	}

	// Open doesn't open a connection. Validate DSN data:
	err = db.Ping()
	if err != nil {
		log.Println(err.Error()) // proper error handling instead of panic in your app
	}

	stmt, err = db.Prepare("insert into heatpump(`logged`," +
		"`ColdWaterIN`,`ColdWaterOut`,`AmbientTemperature`,`SuctionTemperatureEVI`,`CondensorCoilTemperature`," +
		"`SuctionPressureEVI`,`SuctionTemperature`,`SuctionPressure`,`DischargeTemperature`,`DischargePressure`," +
		"`PumpAout`,`UnitStatus`,`CoolingSetPoint`,`HeatingSetPoint`,`CoolHeatModeSelection`," +
		"`CompressorDemand`,`MainValveSuperheat`,`MainValveOpeningSteps`,`MainValveOpeningPercent`,`AuxilliaryValveSuperheat`," +
		"`AuxilliaryValveOpeningSteps`,`AuxilliaryValveOpeningPercent`,`CompressorRotateSpeed`,`InverterStatus`,`MotorCurrent`," +
		"`MotorVoltage`,`InverterTemperature`,`BusVoltage`,`GroundLoopInTemp`,`GroundLoopOutTemp`," +
		"`WaterFlowSwitch`,`EmergencySwitch`,`EndTeminalSignalSwitch`,`HighFan`,`LowFan`," +
		"`FourWayValve`,`MainWaterPump`,`ThreePortValveForWaterCircuit`,`CrankshaftHeater`,`ChassisHeater`," +
		"`EndTerminalWaterPump`,`ElectricHeater`,`UnitStart`,`Alarms`,`RejectInTemp`,`RejectOutTemp`,`Insolation`)" +
		" values (now(),?,?,?,?,?,?,?,?,?,?," +
		"?,?,?,?,?,?,?,?,?,?," +
		"?,?,?,?,?,?,?,?,?,?," +
		"?,?,?,?,?,?,?,?,?,?," +
		"?,?,?,?,?,?,?)")
	if err != nil {
		log.Println(err.Error()) // proper error handling instead of panic in your app

	}

	mbus = PumpController.New(*CommsPort, *BaudRate, *DataBits, *StopBits, *Parity, time.Second*time.Duration(*TimeoutSecs), uint8(*pumpSlaveAddress))
	if mbus != nil {
		defer mbus.Close()

		err := mbus.Connect()
		if err != nil {
			fmt.Println("Port = ", *CommsPort, " baudrate = ", *BaudRate)
			panic(err)
		}
		fmt.Println("Connected to heat pump on ", *CommsPort, " at ", *BaudRate, "baud ", *DataBits, " data ", *StopBits, " Stop ", *Parity, "Parity at slave address ", *hpSlaveAddress)
	}
	// Start the reporting loop
	go reportValues()
	setUpWebSite()
}
