package deej

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jacobsa/go-serial/serial"
	"github.com/ncruces/zenity"
	"go.uber.org/zap"

	"github.com/micmonay/keybd_event"
	"github.com/zanyleonic/picodeej/pkg/deej/util"
)

// SerialIO provides a deej-aware abstraction layer to managing serial I/O
type SerialIO struct {
	comPort  string
	baudRate uint

	deej   *Deej
	logger *zap.SugaredLogger

	stopChannel chan bool
	connected   bool
	connOptions serial.OpenOptions
	conn        io.ReadWriteCloser

	messageQueue       chan []byte
	currentUpload      *ImageUploadState
	currentMultiUpload *MultiUploadState
	transferDialog     *zenity.ProgressDialog
	transferInProgress bool

	lastKnownNumSliders        int
	lastKnownNumSwitches       int
	currentSliderPercentValues []float32
	currentSwitchesDelayValues []time.Time

	last      []time.Time
	kbBonding keybd_event.KeyBonding

	sliderMoveConsumers []chan SliderMoveEvent
}

// SliderMoveEvent represents a single slider move captured by deej
type SliderMoveEvent struct {
	SliderID     int
	PercentValue float32
}

var expectedControlInput = regexp.MustCompile(`^\d{1,4}(\|\d{1,4})* \d(\|\d)*\r\n$`)
var transferInput = regexp.MustCompile(`^OK(?: (?:READY|DONE)? ?\d+| \d+)\r\n$`)
var transferMultipleInput = regexp.MustCompile(`^OK\s+FRAMES?\s+(READY|DONE|NEXT)?\s*\d+\r\n$`)
var transferWait = regexp.MustCompile("^WAIT")
var transferFail = regexp.MustCompile(`^FAIL`)

const UploadBlock = 1024

// NewSerialIO creates a SerialIO instance that uses the provided deej
// instance's connection info to establish communications with the arduino chip
func NewSerialIO(deej *Deej, logger *zap.SugaredLogger) (*SerialIO, error) {
	logger = logger.Named("serial")

	kb, err := keybd_event.NewKeyBonding()

	if err != nil {
		logger.Warn("Failed to initialise key bonding!")
		return nil, err
	}

	sio := &SerialIO{
		deej:                deej,
		logger:              logger,
		stopChannel:         make(chan bool),
		messageQueue:        make(chan []byte, 10),
		connected:           false,
		conn:                nil,
		sliderMoveConsumers: []chan SliderMoveEvent{},
		kbBonding:           kb,
	}

	logger.Debug("Created serial i/o instance")

	// respond to config changes
	sio.setupOnConfigReload()

	return sio, nil
}

// Start attempts to connect to our arduino chip
func (sio *SerialIO) Start() error {

	// don't allow multiple concurrent connections
	if sio.connected {
		sio.logger.Warn("Already connected, can't start another without closing first")
		return errors.New("serial: connection already active")
	}

	// set minimum read size according to platform (0 for windows, 1 for linux)
	// this prevents a rare bug on windows where serial reads get congested,
	// resulting in significant lag
	minimumReadSize := 0
	if util.Linux() {
		minimumReadSize = 1
	}

	sio.connOptions = serial.OpenOptions{
		PortName:        sio.deej.config.ConnectionInfo.COMPort,
		BaudRate:        uint(sio.deej.config.ConnectionInfo.BaudRate),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: uint(minimumReadSize),
	}

	sio.logger.Debugw("Attempting serial connection",
		"comPort", sio.connOptions.PortName,
		"baudRate", sio.connOptions.BaudRate,
		"minReadSize", minimumReadSize)

	var err error
	sio.conn, err = serial.Open(sio.connOptions)
	if err != nil {

		// might need a user notification here, TBD
		sio.logger.Warnw("Failed to open serial connection", "error", err)
		return fmt.Errorf("open serial connection: %w", err)
	}

	namedLogger := sio.logger.Named(strings.ToLower(sio.connOptions.PortName))

	namedLogger.Infow("Connected", "conn", sio.conn)
	sio.connected = true

	// read lines or await a stop
	go func() {
		connReader := bufio.NewReader(sio.conn)

		lineChannel := sio.readLine(namedLogger, connReader)
		sio.writeLine(namedLogger)

		for {
			select {
			case <-sio.stopChannel:
				sio.close(namedLogger)
			case line := <-lineChannel:
				sio.handleLine(namedLogger, line)
			}
		}
	}()

	return nil
}

// Stop signals us to shut down our serial connection, if one is active
func (sio *SerialIO) Stop() {
	if sio.connected {
		sio.logger.Debug("Shutting down serial connection")
		sio.stopChannel <- true
	} else {
		sio.logger.Debug("Not currently connected, nothing to stop")
	}
}

// SubscribeToSliderMoveEvents returns an unbuffered channel that receives
// a sliderMoveEvent struct every time a slider moves
func (sio *SerialIO) SubscribeToSliderMoveEvents() chan SliderMoveEvent {
	ch := make(chan SliderMoveEvent)
	sio.sliderMoveConsumers = append(sio.sliderMoveConsumers, ch)

	return ch
}

func (sio *SerialIO) setupOnConfigReload() {
	configReloadedChannel := sio.deej.config.SubscribeToChanges()

	const stopDelay = 50 * time.Millisecond

	go func() {
		for {
			select {
			case <-configReloadedChannel:

				// make any config reload unset our slider number to ensure process volumes are being re-set
				// (the next read line will emit SliderMoveEvent instances for all sliders)\
				// this needs to happen after a small delay, because the session map will also re-acquire sessions
				// whenever the config file is reloaded, and we don't want it to receive these move events while the map
				// is still cleared. this is kind of ugly, but shouldn't cause any issues
				go func() {
					<-time.After(stopDelay)
					sio.lastKnownNumSliders = 0
				}()

				// if connection params have changed, attempt to stop and start the connection
				if sio.deej.config.ConnectionInfo.COMPort != sio.connOptions.PortName ||
					uint(sio.deej.config.ConnectionInfo.BaudRate) != sio.connOptions.BaudRate {

					sio.logger.Info("Detected change in connection parameters, attempting to renew connection")
					sio.Stop()

					// let the connection close
					<-time.After(stopDelay)

					if err := sio.Start(); err != nil {
						sio.logger.Warnw("Failed to renew connection after parameter change", "error", err)
					} else {
						sio.logger.Debug("Renewed connection successfully")
					}
				}
			}
		}
	}()
}

func (sio *SerialIO) close(logger *zap.SugaredLogger) {
	if err := sio.conn.Close(); err != nil {
		logger.Warnw("Failed to close serial connection", "error", err)
	} else {
		logger.Debug("Serial connection closed")
	}

	sio.conn = nil
	sio.connected = false
}

func (sio *SerialIO) readLine(logger *zap.SugaredLogger, reader *bufio.Reader) chan string {
	ch := make(chan string)

	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {

				if sio.deej.Verbose() {
					logger.Warnw("Failed to read line from serial", "error", err, "line", line)
				}

				// just ignore the line, the read loop will stop after this
				return
			}

			if sio.deej.Verbose() {
				logger.Debugw("Read new line", "line", line)
			}

			// deliver the line to the channel
			ch <- line
		}
	}()

	return ch
}

func (sio *SerialIO) handleLine(logger *zap.SugaredLogger, line string) {
	// For when uploading an image to the microcontroller
	if transferInput.MatchString(line) {
		err := sio.transferBlock(logger, line)
		if err != nil {
			logger.Errorw("Error when transferring block!", "error", err)
		}
		return
	}

	// For when uploading image frames
	if transferMultipleInput.MatchString(line) {
		err := sio.transferAnimated(logger, line)
		if err != nil {
			logger.Errorw("Error when transfering animated image!", "error", err)
		}
		return
	}

	// Handle I/O errors
	if transferFail.MatchString(line) {
		sio.handleTransferError(logger, line)
		return
	}

	if transferWait.MatchString(line) {
		logger.Infow("Waiting for controller not to be busy", "raw", line)
		return
	}

	// this function receives an unsanitized line which is guaranteed to end with LF,
	// but most lines will end with CRLF. it may also have garbage instead of
	// deej-formatted values, so we must check for that! just ignore bad ones
	if !expectedControlInput.MatchString(line) {
		logger.Debugw("Unexpected output", "output", line)
		return
	}

	// trim the suffix
	line = strings.TrimSuffix(line, "\r\n")

	splitLine := strings.Split(line, " ")

	sio.handleSliders(logger, splitLine[0])
	sio.handleSwitches(logger, splitLine[1])
}

func (sio *SerialIO) handleSwitches(logger *zap.SugaredLogger, line string) {
	// split on pipe (|), this gives a slice of numerical strings between "0" and "1023"
	splitLine := strings.Split(line, "|")
	numSwitches := len(splitLine)

	// update our switch count, if needed - this will send slider move events for all
	if numSwitches != sio.lastKnownNumSwitches {
		logger.Infow("Detected switches", "amount", numSwitches)
		sio.lastKnownNumSwitches = numSwitches
		sio.currentSwitchesDelayValues = make([]time.Time, numSwitches)

		for idx := range sio.currentSwitchesDelayValues {
			sio.currentSwitchesDelayValues[idx] = time.Now()
		}
	}

	for switchIdx, stringValue := range splitLine {
		cVal := util.Atob(stringValue)

		if !cVal {
			continue
		}

		currentTime := time.Now()
		pressDiff := currentTime.Sub(sio.currentSwitchesDelayValues[switchIdx])

		logger.Debug("id: ", switchIdx, " delay: ", pressDiff, " vs ", sio.deej.config.SwitchesDelayBetweeenPresses, "ms")

		if pressDiff < (time.Duration(sio.deej.config.SwitchesDelayBetweeenPresses) * time.Millisecond) {
			continue
		}

		sAct, _ := sio.deej.config.SwitchesMapping.get(switchIdx)

		sio.HandleMediaKeys(logger, sAct[0])

		sio.currentSwitchesDelayValues[switchIdx] = time.Now()
	}
}

func (sio *SerialIO) handleSliders(logger *zap.SugaredLogger, line string) {
	// split on pipe (|), this gives a slice of numerical strings between "0" and "1023"
	splitLine := strings.Split(line, "|")
	numSliders := len(splitLine)

	// update our slider count, if needed - this will send slider move events for all
	if numSliders != sio.lastKnownNumSliders {
		logger.Infow("Detected sliders", "amount", numSliders)
		sio.lastKnownNumSliders = numSliders
		sio.currentSliderPercentValues = make([]float32, numSliders)

		// reset everything to be an impossible value to force the slider move event later
		for idx := range sio.currentSliderPercentValues {
			sio.currentSliderPercentValues[idx] = -1.0
		}
	}

	// for each slider:
	moveEvents := []SliderMoveEvent{}
	for sliderIdx, stringValue := range splitLine {

		// convert string values to integers ("1023" -> 1023)
		number, _ := strconv.Atoi(stringValue)

		// turns out the first line could come out dirty sometimes (i.e. "4558|925|41|643|220")
		// so let's check the first number for correctness just in case
		if sliderIdx == 0 && number > 1023 {
			sio.logger.Debugw("Got malformed line from serial, ignoring", "line", line)
			return
		}

		// map the value from raw to a "dirty" float between 0 and 1 (e.g. 0.15451...)
		dirtyFloat := float32(number) / 1023.0

		// normalize it to an actual volume scalar between 0.0 and 1.0 with 2 points of precision
		normalizedScalar := util.NormalizeScalar(dirtyFloat)

		// if sliders are inverted, take the complement of 1.0
		if sio.deej.config.InvertSliders {
			normalizedScalar = 1 - normalizedScalar
		}

		// check if it changes the desired state (could just be a jumpy raw slider value)
		if util.SignificantlyDifferent(sio.currentSliderPercentValues[sliderIdx], normalizedScalar, sio.deej.config.NoiseReductionLevel) {

			// if it does, update the saved value and create a move event
			sio.currentSliderPercentValues[sliderIdx] = normalizedScalar

			moveEvents = append(moveEvents, SliderMoveEvent{
				SliderID:     sliderIdx,
				PercentValue: normalizedScalar,
			})

			if sio.deej.Verbose() {
				logger.Debugw("Slider moved", "event", moveEvents[len(moveEvents)-1])
			}
		}
	}

	// deliver move events if there are any, towards all potential consumers
	if len(moveEvents) > 0 {
		for _, consumer := range sio.sliderMoveConsumers {
			for _, moveEvent := range moveEvents {
				consumer <- moveEvent
			}
		}
	}
}

func (sio *SerialIO) writeLine(logger *zap.SugaredLogger) {
	go func() {
		for {
			select {
			case msg, more := <-sio.messageQueue:
				b, err := sio.conn.Write(msg)
				if err != nil {
					logger.Errorw("Error when writing bytes to buffer serial device", "error", err)
				}

				logger.Debugw("Wrote to serial device", "bytes", b)

				if !more || sio.messageQueue == nil {
					return
				}
			}
		}
	}()
}
