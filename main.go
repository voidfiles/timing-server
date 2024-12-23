package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
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

type ChannelFormat int

const (
	CHANNEL_FORMAT_RAW ChannelFormat = iota
	CHANNEL_FORMAT_CLOCK
	CHANNEL_FORMAT_LANE
)

const SPACE_ASCII = 0x20

// content holds our static web server content.
var fs = http.FileServer(http.Dir("./static"))

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

type JSONableSlice []uint8

func (u JSONableSlice) MarshalJSON() ([]byte, error) {
	var result string
	if u == nil {
		result = "null"
	} else {
		result = strings.Join(strings.Fields(fmt.Sprintf("%d", u)), ",")
	}
	return []byte(result), nil
}

// Application Types
type Channel struct {
	Number   uint8         `json:"number,omitempty"`
	Data     JSONableSlice `json:"data,omitempty"`
	Format   JSONableSlice `json:"foramt,omitempty"`
	FormatAs ChannelFormat `json:"-"`
}

type ChannelAlias struct {
	Number       uint8         `json:"number,omitempty"`
	Data         JSONableSlice `json:"data,omitempty"`
	Format       JSONableSlice `json:"format,omitempty"`
	Preformatted string        `json:"preformatted"`
}

func (u *Channel) MarshalJSON() ([]byte, error) {
	// Create an alias to avoid recursion
	return json.Marshal(&ChannelAlias{
		u.Number,
		u.Data,
		u.Format,
		u.Formatted(),
	})
}

func (c *Channel) Formatted() string {
	switch c.FormatAs {
	case CHANNEL_FORMAT_RAW:
		return c.RawFormat()
	case CHANNEL_FORMAT_CLOCK:
		return c.ClockFormat()
	case CHANNEL_FORMAT_LANE:
		return c.LaneFormat()
	default:
		return c.RawFormat()
	}
}

func (c *Channel) getChar(segment uint8) string {
	ch := c.Data[segment]
	if ch == SPACE_ASCII || ch == 63 {
		return "_"
	}
	return string(rune(ch))
}

func (c *Channel) getTime() string {
	if c.getChar(5) == "0" && c.getChar(6) == "0" {
		return "--:--.-"
	} else {
		output := c.getChar(2) + c.getChar(3) + ":" + c.getChar(4) + c.getChar(5) + "." + c.getChar(6) + c.getChar(7)
		return strings.ReplaceAll(output, "_", "0")
	}
}

func (c *Channel) LaneFormat() string {
	lane := c.getChar(0)
	place := c.getChar(1)
	time := c.getTime()

	return fmt.Sprintf("%s %s %s", lane, place, time)
}

func (c *Channel) ClockFormat() string {
	return c.getTime()
}

func (c *Channel) RawFormat() string {
	out := ""
	for i := 0; i < len(c.Data); i++ {
		out += c.getChar(uint8(i))
	}

	return out
}

func MustNewChannel(number uint8, format ChannelFormat) *Channel {
	return &Channel{
		Number:   number,
		Data:     make([]uint8, 8),
		Format:   make([]uint8, 8),
		FormatAs: format,
	}
}

type Frame struct {
	Channels map[uint8]*Channel      `json:"channels"`
	Formats  map[uint8]ChannelFormat `json:"-"`
}

func (f *Frame) BlankChannelData(channel uint8) {
	for i := 0; i < 8; i++ {
		f.Channels[channel].Data[i] = 0x20
	}
}

func (f *Frame) SetSegment(channel uint8, segment int, data uint8, channelReadState ChannelReadState) {
	// log.Debug("%d %d %d", channel, segment, data)
	if val, ok := f.Channels[channel]; ok {
		if channelReadState == CHANNEL_READ_STATE_DATA {
			val.Data[segment] = data
		} else {
			val.Format[segment] = data
		}
	} else {
		fmt, ok := f.Formats[channel]
		if !ok {
			fmt = CHANNEL_FORMAT_RAW
		}
		f.Channels[channel] = MustNewChannel(channel, fmt)
		if channelReadState == CHANNEL_READ_STATE_DATA {
			f.Channels[channel].Data[segment] = data
		} else {
			f.Channels[channel].Format[segment] = data
		}

	}
}

func (f *Frame) UpdateFromChannel(channel *Channel) {
	val, ok := f.Channels[channel.Number]
	if !ok {
		fmt, ok := f.Formats[channel.Number]
		if !ok {
			fmt = CHANNEL_FORMAT_RAW
		}
		val = MustNewChannel(channel.Number, fmt)
		f.Channels[channel.Number] = val
	}

	for i := 0; i < len(channel.Data); i++ {
		val.Data[i] = channel.Data[i]
		val.Format[i] = channel.Format[i]
	}
}

func (f *Frame) AsJSON() ([]byte, error) {
	return json.Marshal(f)
}

func MustNewFrame(formats map[uint8]ChannelFormat) *Frame {
	channels := make(map[uint8]*Channel)

	return &Frame{
		Channels: channels,
		Formats:  formats,
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

	return DevBinReader{input: &RateLimitFileReader{fd}}
}

type TIMING_ITERATOR_STATE int

const (
	TIMING_ITERATOR_STATE_DONE TIMING_ITERATOR_STATE = iota
	TIMING_ITERATOR_STATE_MORE
)

type TimingIterator struct {
	input        io.ReadSeekCloser
	outputFrame  *Frame
	lastChannel  uint8
	channel      uint8
	replay       bool
	frameSync    sync.Mutex
	laneUp       bool
	landeAddress bool
}

func MustNewTimingIterator(input io.ReadSeekCloser, replay bool) *TimingIterator {
	ti := &TimingIterator{
		input:        input,
		outputFrame:  MustNewFrame(map[uint8]ChannelFormat{}),
		lastChannel:  0,
		channel:      0,
		replay:       replay,
		frameSync:    sync.Mutex{},
		laneUp:       false,
		landeAddress: false,
	}

	return ti
}

func (ti *TimingIterator) Iterate() TIMING_ITERATOR_STATE {
	readLen := 1
	frame := make([]byte, readLen)

	for {
		read, err := ti.input.Read(frame)

		if err != nil {
			log.Fatalf("failed to read from input %s", err)
		}

		if read != readLen {
			log.Printf("finished reading input")
			return TIMING_ITERATOR_STATE_DONE
		}

		currentByte := frame[0]

		if ti.lastChannel != ti.channel {
			ti.lastChannel = ti.channel
		}

		if currentByte >= 0x7f {
			channelReadState = ChannelReadState((currentByte & 1))

			if ti.channel > 1 && ti.channel < 7 {
				logger.Debug("Parsing control byte new_frame:true", "channel", ti.channel, "data", ti.channel)
			}

			ti.channel = ((currentByte >> 1) & 0x1f) ^ 0x1f
			logger.Debug("Parsing control byte new_frame:true", "channelReadState", channelReadState, "channel", ti.channel)

			if currentByte > 190 {
				ti.outputFrame.BlankChannelData(ti.channel)
			} else {
				ti.laneUp = false
			}

			if currentByte > 169 && currentByte < 190 {
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
				ti.outputFrame.SetSegment(ti.channel, int(segmentNum), segmentData, channelReadState)
			} else {
				segmentNum := (currentByte & 0xf0) >> 4
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
				ti.outputFrame.SetSegment(ti.channel, int(segmentNum), segmentData, channelReadState)
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

func (ti *TimingIterator) Value() uint8 {
	return ti.lastChannel
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

	mux.HandleFunc("/", fs.ServeHTTP)
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

func Integrator(m *melody.Melody, timing_iterator *TimingIterator, frame Frame) {
	for {
		if !timing_iterator.Next() {
			break
		}
		channelNumber := timing_iterator.Value()
		frame.UpdateFromChannel(timing_iterator.outputFrame.Channels[channelNumber])
		j, err := frame.AsJSON()
		if err != nil {
			panic(fmt.Errorf("Integrator %s", err))
		}
		if err := m.Broadcast([]byte(j)); err != nil {
			panic(err)
		}

		logger.Debug("msg", "msg", string(j))
	}
}

func main() {

	config()

	in := GetInput()
	defer in.Close()

	m := melody.New()
	f := MustNewFrame(map[uint8]ChannelFormat{
		0: CHANNEL_FORMAT_CLOCK,
		1: CHANNEL_FORMAT_LANE,
		2: CHANNEL_FORMAT_LANE,
		3: CHANNEL_FORMAT_LANE,
		4: CHANNEL_FORMAT_LANE,
		5: CHANNEL_FORMAT_LANE,
		6: CHANNEL_FORMAT_LANE,
	})
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
		Integrator(m, timing_iterator, *f)
	}()

	wg.Wait()
}
