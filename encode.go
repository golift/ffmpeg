package ffmpeg

/* Encode videos from RTSP URLs using FFMPEG */

import (
	"bytes"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Default, Maximum and Minimum Values
var (
	DefaultFrameRate   = 5
	MinimumFrameRate   = 1
	MaximumFrameRate   = 60
	DefaultFrameHeight = 720
	DefaultFrameWidth  = 1280
	MinimumFrameSize   = 100
	MaximumFrameSize   = 5000
	DefaultEncodeCRF   = 21
	MinimumEncodeCRF   = 16
	MaximumEncodeCRF   = 30
	DefaultCaptureTime = 15
	MaximumCaptureTime = 1200             // 10 minute max.
	DefaultCaptureSize = int64(2500000)   // 2.5MB default (roughly 5-10 seconds)
	MaximumCaptureSize = int64(104857600) // 100MB max.
	DefaultFFmpegPath  = "/usr/local/bin/ffmpeg"
	DefaultProfile     = "main"
	DefaultLevel       = "3.0"
	ErrorInvalidOutput = errors.New("output path is not valid")
	ErrorInvalidInput  = errors.New("input path is not valid")
)

// Config defines how to ffmpeg shall transcode a stream.
type Config struct {
	FFMPEG string // "/usr/local/bin/ffmpeg"
	Level  string // 3.0, 3.1 ..
	Width  int    // 1920
	Height int    // 1080
	CRF    int    // 24
	Time   int    // 15 (seconds)
	Audio  bool   // include audio?
	Rate   int    // framerate (5-20)
	Size   int64  // max file size (always goes over). use 2000000 for 2.5MB
	Prof   string // main, high, baseline
	Copy   bool   // Copy original stream, rather than transcode.
}

// Encoder is the struct returned by this library.
// Contains all the bound methods.
type Encoder struct {
	config *Config
}

// Get an encoder interface.
func Get(config *Config) *Encoder {
	e := &Encoder{config: config}
	if e.config == nil {
		e.config = &Config{FFMPEG: DefaultFFmpegPath}
	} else if e.config.FFMPEG == "" {
		e.config.FFMPEG = DefaultFFmpegPath
	}
	e.SetLevel(e.config.Level)
	e.SetProfile(e.config.Prof)
	e.fixValues()
	return e
}

// Config returns the current values in the encoder.
func (e *Encoder) Config() Config {
	return *e.config
}

// SetAudio turns audio on or off based on a string value.
func (e *Encoder) SetAudio(audio string) bool {
	e.config.Audio, _ = strconv.ParseBool(audio)
	return e.config.Audio
}

// SetLevel sets the h264 transcode level.
func (e *Encoder) SetLevel(level string) string {
	if e.config.Level = level; level != "3.0" && level != "3.1" && level != "4.0" && level != "4.1" && level != "4.2" {
		e.config.Level = DefaultLevel
	}
	return e.config.Level
}

// SetProfile sets the h264 transcode profile.
func (e *Encoder) SetProfile(profile string) string {
	if e.config.Prof = profile; e.config.Prof != "main" && e.config.Prof != "baseline" && e.config.Prof != "high" {
		e.config.Prof = DefaultProfile
	}
	return e.config.Prof
}

// SetWidth sets the transcode frame width.
func (e *Encoder) SetWidth(width string) int {
	e.config.Width, _ = strconv.Atoi(width)
	e.fixValues()
	return e.config.Width
}

// SetHeight sets the transcode frame width.
func (e *Encoder) SetHeight(height string) int {
	e.config.Height, _ = strconv.Atoi(height)
	e.fixValues()
	return e.config.Height
}

// SetCRF sets the h264 transcode CRF value.
func (e *Encoder) SetCRF(crf string) int {
	e.config.CRF, _ = strconv.Atoi(crf)
	e.fixValues()
	return e.config.CRF
}

// SetTime sets the maximum transcode duration.
func (e *Encoder) SetTime(seconds string) int {
	e.config.Time, _ = strconv.Atoi(seconds)
	e.fixValues()
	return e.config.Time
}

// SetRate sets the transcode framerate.
func (e *Encoder) SetRate(rate string) int {
	e.config.Rate, _ = strconv.Atoi(rate)
	e.fixValues()
	return e.config.Rate
}

// SetSize sets the maximum transcode file size.
func (e *Encoder) SetSize(size string) int64 {
	e.config.Size, _ = strconv.ParseInt(size, 10, 64)
	e.fixValues()
	return e.config.Size
}

func (e *Encoder) getVideoHandle(input, output, title string) (string, *exec.Cmd) {
	if title == "" {
		title = filepath.Base(output)
	}
	arg := []string{
		e.config.FFMPEG,
		"-v", "16", // log level
		"-rtsp_transport", "tcp",
		"-i", input,
		"-f", "mov",
		"-metadata", `title="` + title + `"`,
		"-y", "-map", "0",
	}
	if e.config.Size > 0 {
		arg = append(arg, "-fs", strconv.FormatInt(e.config.Size, 10))
	}
	if e.config.Time > 0 {
		arg = append(arg, "-t", strconv.Itoa(e.config.Time))
	}
	if !e.config.Copy {
		arg = append(arg, "-vcodec", "libx264",
			"-profile:v", e.config.Prof,
			"-level", e.config.Level,
			"-pix_fmt", "yuv420p",
			"-movflags", "faststart",
			"-s", strconv.Itoa(e.config.Width)+"x"+strconv.Itoa(e.config.Height),
			"-preset", "superfast",
			"-crf", strconv.Itoa(e.config.CRF),
			"-r", strconv.Itoa(e.config.Rate),
		)
	} else {
		arg = append(arg, "-c", "copy")
	}
	if !e.config.Audio {
		arg = append(arg, "-an")
	} else {
		arg = append(arg, "-c:a", "copy")
	}
	arg = append(arg, output)
	cmd := exec.Command(arg[0], arg[1:]...)

	return strings.Join(arg, " "), cmd
}

// GetVideo retreives video from an input and returns a Reader to consume the output.
// The Reader contains output messages if output is a filepath.
// The Reader contains the video if the output is "-"
func (e *Encoder) GetVideo(input, title string) (string, io.ReadCloser, error) {
	if input == "" {
		return "", nil, ErrorInvalidInput
	}
	cmdStr, cmd := e.getVideoHandle(input, "-", title)
	stdoutpipe, err := cmd.StdoutPipe()
	if err != nil {
		return cmdStr, nil, err
	}
	return cmdStr, stdoutpipe, cmd.Run()
}

// SaveVideo saves a video snippet to a file.
func (e *Encoder) SaveVideo(input, output, title string) (string, string, error) {
	if input == "" {
		return "", "", ErrorInvalidInput
	} else if output == "" || output == "-" {
		return "", "", ErrorInvalidOutput
	}
	cmdStr, cmd := e.getVideoHandle(input, output, title)
	// log.Println(cmdStr) // DEBUG
	var out bytes.Buffer
	cmd.Stderr = &out
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return cmdStr, "", err
	}
	err := cmd.Wait()
	return cmdStr, strings.TrimSpace(out.String()), err
}

// fixValues makes sure video request values are sane.
func (e *Encoder) fixValues() {
	if e.config.Height == 0 {
		e.config.Height = DefaultFrameHeight
	} else if e.config.Height > MaximumFrameSize {
		e.config.Height = MaximumFrameSize
	} else if e.config.Height < MinimumFrameSize {
		e.config.Height = MinimumFrameSize
	}

	if e.config.Width == 0 {
		e.config.Width = DefaultFrameWidth
	} else if e.config.Width > MaximumFrameSize {
		e.config.Width = MaximumFrameSize
	} else if e.config.Width < MinimumFrameSize {
		e.config.Width = MinimumFrameSize
	}

	if e.config.CRF == 0 {
		e.config.CRF = DefaultEncodeCRF
	} else if e.config.CRF < MinimumEncodeCRF {
		e.config.CRF = MinimumEncodeCRF
	} else if e.config.CRF > MaximumEncodeCRF {
		e.config.CRF = MaximumEncodeCRF
	}

	if e.config.Rate == 0 {
		e.config.Rate = DefaultFrameRate
	} else if e.config.Rate < MinimumFrameRate {
		e.config.Rate = MinimumFrameRate
	} else if e.config.Rate > MaximumFrameRate {
		e.config.Rate = MaximumFrameRate
	}

	// No minimums.
	if e.config.Time == 0 {
		e.config.Time = DefaultCaptureTime
	} else if e.config.Time > MaximumCaptureTime {
		e.config.Time = MaximumCaptureTime
	}

	if e.config.Size == 0 {
		e.config.Size = DefaultCaptureSize
	} else if e.config.Size > MaximumCaptureSize {
		e.config.Size = MaximumCaptureSize
	}
}
