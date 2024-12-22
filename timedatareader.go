package main

import (
	"errors"
	"io"
	"log"
)

type SegmentValueKind int

var DO_NOT_UPDATE uint8 = 254

const (
	SEGMENT_DATA SegmentValueKind = iota
	SEGMENT_FORMAT
)

type TimeDataReaderState int

const (
	TIME_DATA_READER_DONE TimeDataReaderState = iota
	TIME_DATA_READER_MORE
	TIME_DATA_READER_LOOP
)

type ChannelSegments struct {
	Number int
	Data   []byte
	Format []byte
	Blank  bool
}

type TimeDataReader struct {
	input       io.ReadSeekCloser
	upNext      *Channel
	value       *Channel
	frameBuffer []byte
	readBuffer  []byte
	readState   SegmentValueKind
}

func MustNewTimeDataReader(input io.ReadSeekCloser) *TimeDataReader {
	var blankSlice = make([]byte, 8)
	for i := range blankSlice {
		blankSlice[i] = DO_NOT_UPDATE
	}
	upNextData := make([]byte, 8)
	upNextFormat := make([]byte, 8)
	valueData := make([]byte, 8)
	valueFormat := blankSlice

	copy(blankSlice, upNextData)
	copy(blankSlice, upNextFormat)
	copy(blankSlice, valueData)

	return &TimeDataReader{
		input: input,
		upNext: &Channel{
			Number: 0,
			Data:   upNextData,
			Format: upNextFormat,
		},
		value: &Channel{
			Number: 0,
			Data:   valueData,
			Format: valueFormat,
		},
		frameBuffer: make([]byte, 8),
		readBuffer:  make([]byte, 1),
	}
}

var ErrDone = errors.New("time data reader is done")

func (ti *TimeDataReader) Value() *Channel {
	return ti.value
}

func (ti *TimeDataReader) Next() bool {
	if state, err := ti.Iterate(); err != nil {
		if errors.Is(err, ErrDone) {
			return false
		}
		log.Fatalf("Can't iterate %s", err)
		panic(err)
	} else {
		if state == TIME_DATA_READER_MORE {
			return true
		} else {
			return false
		}
	}
}

func (ti *TimeDataReader) Iterate() (TimeDataReaderState, error) {
	var channel uint8
	for {
		read, err := ti.input.Read(ti.readBuffer)
		if err != nil {
			return TIME_DATA_READER_DONE, ErrDone
		}

		if read != 1 {
			return TIME_DATA_READER_DONE, ErrDone
		}

		currentByte := ti.readBuffer[0]

		if currentByte >= 0x7f {
			ti.readState = SegmentValueKind(currentByte & 1)

			channel = ((currentByte >> 1) & 0x1f) ^ 0x1f

			if channel == ti.upNext.Number {
				if currentByte > 190 {
					for i := 0; i < 8; i++ {
						ti.upNext.Data[i] = SPACE_ASCII
					}
				}

				if currentByte > 169 && currentByte < 190 {
					if ti.upNext.Data[0] == SPACE_ASCII {
						for i := 0; i < 8; i++ {
							ti.upNext.Data[i] = SPACE_ASCII
						}
					}
				}
			}

			if channel != ti.upNext.Number {
				ti.upNext.Number = channel
				for i, k := range ti.upNext.Data {
					ti.value.Data[i] = k
					ti.value.Format[i] = ti.upNext.Format[i]
					ti.upNext.Data[i] = 254
					ti.upNext.Format[i] = 254
				}

				return TIME_DATA_READER_MORE, nil
			}

			ti.upNext.Number = channel
		}

		segmentNum := (currentByte & 0xf0) >> 4
		segmentData := (currentByte & 0x0f) ^ 0x0f + 48

		// if (ti.upNext.Number > 0) && (segmentData == 0) {
		// 	// Blank the character
		// 	segmentData = SPACE_ASCII
		// } else {
		// 	segmentData = segmentData ^ 0x0f + 48 // 40 = 0x30 = ASCII '0'
		// }

		if segmentNum <= 7 {
			if ti.readState == SEGMENT_DATA {
				ti.upNext.Data[segmentNum] = segmentData
			} else {
				ti.upNext.Format[segmentNum] = segmentData
			}
		}

	}
}
