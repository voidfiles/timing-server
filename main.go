package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jacobsa/go-serial/serial"
	"github.com/olahol/melody"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// CONSTANTS

type ChannelReadState int

const (
	CHANNEL_READ_STATE_DATA ChannelReadState = iota
	CHANNEL_READ_STATE_FORMAT
)

const SPACE_ASCII = 0x20

// CONFIGURATION

var FileOpt *string = flag.String("file", "", "use input file instead of serial port")
var ConfigFile *string = flag.String("config", "", "Pointer to a config file")
var PortName *string = flag.String("port", "", "")
var help = *flag.Bool("help", false, "print this output")
var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

// start listening for updates and render

var channelReadState = CHANNEL_READ_STATE_DATA

func config() {
	viper.SetDefault("port", "/dev/tty.usbserial-A8008HlV")
	viper.SetDefault("baud_rate", 9600)
	viper.SetDefault("data_bits", 8)
	viper.SetDefault("stop_bits", 1)
	viper.SetDefault("parity_mode", serial.PARITY_EVEN)

	flag.Parse()
	if *ConfigFile != "" {
		viper.SetConfigName(*ConfigFile) // name of config file (without extension)
		viper.AddConfigPath(".")         // optionally look for config in the working directory
		err := viper.ReadInConfig()      // Find and read the config file
		if err != nil {                  // Handle errors reading the config file
			panic(fmt.Errorf("fatal error config file: %w", err))
		}
	}

	viper.BindPFlags(flag.CommandLine)

	if help {
		flag.PrintDefaults()
		panic("no args")
	}
}

// Application Types
type Channel struct {
	Number int   `json:"number,omitempty"`
	Data   []int `json:"data,omitempty"`
	Format []int `json:"format,omitempty"`
}

func (c *Channel) BlankData() {
	for i := range c.Data {
		c.Data[i] = 0x20
	}
}

func MustNewChannel(number int) *Channel {
	return &Channel{
		Number: number,
		Data:   make([]int, 8),
		Format: make([]int, 8),
	}
}

type Frame struct {
	Channels map[int]*Channel `json:"channels"`
	sync     sync.Mutex       `json:"-"`
}

func (f *Frame) BlankChannelData(channel int) {
	f.sync.Lock()
	defer f.sync.Unlock()
	i := 7
	for i >= 0 {
		f.Channels[channel].Data[i] = 0x20
		i--
	}
}

func (f *Frame) SetSegment(channel int, segment int, data int, channelReadState ChannelReadState) {
	f.sync.Lock()
	defer f.sync.Unlock()
	// log.Debug("%d %d %d", channel, segment, data)
	if val, ok := f.Channels[channel]; ok {
		if channelReadState == CHANNEL_READ_STATE_DATA {
			val.Data[segment] = data
		} else {
			val.Format[segment] = data
		}
	} else {
		f.Channels[channel] = MustNewChannel(channel)
		if channelReadState == CHANNEL_READ_STATE_DATA {
			f.Channels[channel].Data[segment] = data
		} else {
			f.Channels[channel].Format[segment] = data
		}

	}
}

func (f *Frame) getChar(channel, segment int) string {
	return string(rune(f.Channels[channel].Data[segment]))
}

func (f *Frame) getTime(channel int) string {
	if f.getChar(channel, 5) == "0" && f.getChar(channel, 6) == "0" {
		return "--:--.-"
	} else {
		return f.getChar(channel, 2) + f.getChar(channel, 3) + ":" + f.getChar(channel, 4) + f.getChar(channel, 5) + "." + f.getChar(channel, 6)
	}
}

func (f *Frame) LaneFormat(channel int) string {
	lane := f.getChar(channel, 0)
	place := f.getChar(channel, 1)
	time := f.getTime(channel)

	return fmt.Sprintf("%s %s %s", lane, place, time)
}

func (f *Frame) AsJSON() ([]byte, error) {
	f.sync.Lock()
	defer f.sync.Unlock()
	return json.Marshal(f)
}

func MustNewFrame() *Frame {
	channels := make(map[int]*Channel)

	return &Frame{
		Channels: channels,
		sync:     sync.Mutex{},
	}
}

// Application Code

func MustGetSerial(options serial.OpenOptions) io.ReadWriteCloser {
	port, err := serial.Open(options)
	if err != nil {
		log.Fatalf("serial.Open: %v", err)
	}

	return port
}

type file = *os.File

type RateLimitFileReader struct {
	file
}

func (r *RateLimitFileReader) Read(b []byte) (n int, err error) {
	time.Sleep(time.Second / 1000)
	return r.file.Read(b)
}

func MustGetFile(file string) io.ReadSeekCloser {
	fd, err := os.Open(file)
	if err != nil {
		log.Fatalf("os.Open: %v", err)
	}

	return &RateLimitFileReader{fd}
}

type TIMING_ITERATOR_STATE int

const (
	TIMING_ITERATOR_STATE_DONE TIMING_ITERATOR_STATE = iota
	TIMING_ITERATOR_STATE_MORE
)

type TimingIterator struct {
	input        io.ReadSeekCloser
	outputFrame  *Frame
	channel      int
	replay       bool
	frameSync    sync.Mutex
	laneUp       bool
	landeAddress bool
}

func MustNewTimingIterator(input io.ReadSeekCloser, replay bool) *TimingIterator {
	ti := &TimingIterator{
		input:        input,
		outputFrame:  MustNewFrame(),
		channel:      0,
		replay:       replay,
		frameSync:    sync.Mutex{},
		laneUp:       false,
		landeAddress: false,
	}

	ti.frameSync.Lock()

	return ti
}

func (ti *TimingIterator) Iterate() TIMING_ITERATOR_STATE {
	readLen := 1
	frame := make([]byte, readLen)

	for {
		read, err := ti.input.Read(frame)

		if err != nil {
			if errors.Is(err, io.EOF) {
				logger.Debug("Reached end of file")
				if ti.replay {
					logger.Debug("Attempting to restart")
					if _, err := ti.input.Seek(0, io.SeekStart); err != nil {
						log.Fatalf("failed to replay data %s", err)
						return TIMING_ITERATOR_STATE_DONE
					} else {
						logger.Debug("Restarting file from start")
						continue
					}
				}
				return TIMING_ITERATOR_STATE_DONE
			}

			log.Fatalf("failed to read from input %s", err)
		}

		if read != readLen {
			log.Printf("finished reading input")
			return TIMING_ITERATOR_STATE_DONE
		}

		currentByte := frame[0]

		if currentByte >= 0x7f {
			ti.frameSync.Unlock()
			defer ti.frameSync.Lock()
			channelReadState = ChannelReadState((currentByte & 1))

			if ti.channel > 1 && ti.channel < 7 {
				logger.Debug("Parsing control byte new_frame:true", "channel", ti.channel, "data", ti.outputFrame.LaneFormat(ti.channel))
			}

			ti.channel = int(((currentByte >> 1) & 0x1f) ^ 0x1f)

			logger.Debug("Parsing control byte new_frame:true", "channelReadState", channelReadState, "channel", ti.channel)

			if int(currentByte) > 190 {
				ti.outputFrame.BlankChannelData(ti.channel)
			} else {
				ti.laneUp = false
			}

			if int(currentByte) > 169 && int(currentByte) < 190 {
				ti.landeAddress = true
			} else {
				ti.landeAddress = false
			}

			return TIMING_ITERATOR_STATE_MORE
		} else {
			logger.Debug("On channel", "channelReadState", channelReadState, "channel", ti.channel)
			if channelReadState == CHANNEL_READ_STATE_DATA {
				segmentNum := (currentByte & 0xf0) >> 4
				if segmentNum >= 8 {
					log.Printf("While parsing the data for a channel found a segment greater then 8 %d", segmentNum)
					continue
				}

				segmentData := ((currentByte << 4) & 0xf0) >> 4
				if (ti.channel > 0) && (segmentData == 0) {
					// Blank the character
					segmentData = SPACE_ASCII
				} else {
					segmentData = segmentData ^ 0x0f + 48 // 40 = 0x30 = ASCII '0'
				}
				ti.outputFrame.SetSegment(ti.channel, int(segmentNum), int(segmentData), channelReadState)
			} else {
				segmentNum := int((currentByte & 0xf0) >> 4)
				if segmentNum >= 8 {
					log.Printf("While parsing the data for a channel found a segment greater then 8 %d", segmentNum)
					continue
				}
				segmentData := (currentByte & 0x0f)
				if (ti.channel > 0) && (segmentData == 0) {
					// Blank the character
					segmentData = SPACE_ASCII
				} else {
					// data = data ^ 0x0f + 48;
					segmentData = segmentData ^ 0x0f + 48 // 40 = 0x30 = ASCII '0'
				}
				ti.outputFrame.SetSegment(ti.channel, segmentNum, int(segmentData), channelReadState)
			}
		}
	}
}

func (ti *TimingIterator) Next() bool {
	if ti.Iterate() == TIMING_ITERATOR_STATE_MORE {
		return true
	} else {
		return false
	}
}

func (ti *TimingIterator) Value() *Frame {
	ti.frameSync.Lock()
	defer ti.frameSync.Unlock()

	return ti.outputFrame
}

type SeekerError struct {
	io.ReadCloser
}

func (s SeekerError) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("this will always fail")
}

func GetInput() io.ReadSeekCloser {
	if viper.GetString("file") != "" {
		return MustGetFile(viper.GetString("file"))
	} else {
		portName := viper.GetString("port")
		if portName == "" {
			panic("serial.Open: no port name")
		}
		p := MustGetSerial(serial.OpenOptions{
			PortName:   portName,
			BaudRate:   uint(viper.GetInt("baud_rate")),
			DataBits:   uint(viper.GetInt("data_bits")),
			StopBits:   uint(viper.GetInt("stop_bits")),
			ParityMode: serial.ParityMode(viper.GetInt("parity_mode")),
		})

		return SeekerError{p}
	}
}

func HTTPServer(m *melody.Melody) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("got a ws request")
		m.HandleRequest(w, r)
	})

	return &http.Server{
		Addr:         "127.0.0.1:8000",
		Handler:      mux,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
}

func Integrator(m *melody.Melody, timing_iterator *TimingIterator) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for timing_iterator.Next() {
			// timing_iterator.Value()

		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			frame := timing_iterator.Value()
			j, err := frame.AsJSON()
			if err != nil {
				panic(fmt.Errorf("Integrator %s", err))
			}

			if err := m.Broadcast([]byte(j)); err != nil {
				panic(err)
			}

			logger.Debug("msg", "msg", string(j))
			time.Sleep(time.Second / 10)
		}
	}()

	wg.Wait()
}

func main() {

	config()

	in := GetInput()
	defer in.Close()

	m := melody.New()
	timing_iterator := MustNewTimingIterator(in, true)

	server := HTTPServer(m)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		server.ListenAndServe()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		Integrator(m, timing_iterator)
	}()

	wg.Wait()
}
