// Package ffmpeg captures video from RTSP streams, like IP cameras.
//
// Provides a simple interface to set FFMPEG options and capture video from an RTSP source.
package ffmpeg

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Default, Maximum and Minimum Values for encoder configuration. Change these if your needs differ.
//
//nolint:gochecknoglobals
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
	DefaultCaptureSize = int64(2500000)   //nolint:gomnd,nolintlint // 2.5MB default (roughly 5-10 seconds)
	MaximumCaptureSize = int64(104857600) //nolint:gomnd,nolintlint // 100MB max.
	DefaultFFmpegPath  = "/usr/local/bin/ffmpeg"
	DefaultProfile     = "main"
	DefaultLevel       = "3.0"
)

// Custom errors that this library outputs. The library also outputs errors created elsewhere.
var (
	ErrInvalidOutput = fmt.Errorf("output path is not valid")
	ErrInvalidInput  = fmt.Errorf("input path is not valid")
)

// Config defines how ffmpeg shall transcode a stream.
// If Copy is true, these options are ignored: profile, level, width, height, crf and frame rate.
type Config struct {
	Copy   bool   // Copy original stream, rather than transcode.
	Audio  bool   // include audio?
	Width  int    // 1920
	Height int    // 1080
	CRF    int    // 24
	Time   int    // 15 (seconds)
	Rate   int    // framerate (5-20)
	Size   int64  // max file size (always goes over). use 2000000 for 2.5MB
	FFMPEG string // "/usr/local/bin/ffmpeg"
	Level  string // 3.0, 3.1 ..
	Prof   string // main, high, baseline
}

// Encoder is the struct returned by this library.
// Contains all the bound methods.
type Encoder struct {
	config *Config
}

// Get an encoder interface.
func Get(config *Config) *Encoder {
	encode := &Encoder{config: config}
	if encode.config.FFMPEG == "" {
		encode.config.FFMPEG = DefaultFFmpegPath
	}

	encode.SetLevel(encode.config.Level)
	encode.SetProfile(encode.config.Prof)
	encode.fixValues()

	return encode
}

// Config returns the current values in the encoder.
func (e *Encoder) Config() Config {
	return *e.config
}

// SetAudio turns audio on or off based on a string value.
// This can also be passed into Get() as a boolean.
func (e *Encoder) SetAudio(audio string) bool {
	e.config.Audio, _ = strconv.ParseBool(audio)

	return e.config.Audio
}

// SetLevel sets the h264 transcode level.
// This can also be passed into Get().
func (e *Encoder) SetLevel(level string) string {
	if e.config.Level = level; level != "3.0" && level != "3.1" && level != "4.0" && level != "4.1" && level != "4.2" {
		e.config.Level = DefaultLevel
	}

	return e.config.Level
}

// SetProfile sets the h264 transcode profile.
// This can also be passed into Get().
func (e *Encoder) SetProfile(profile string) string {
	if e.config.Prof = profile; e.config.Prof != "main" && e.config.Prof != "baseline" && e.config.Prof != "high" {
		e.config.Prof = DefaultProfile
	}

	return e.config.Prof
}

// SetWidth sets the transcode frame width from a string.
// This can also be passed into Get() as an int.
func (e *Encoder) SetWidth(width string) int {
	e.config.Width, _ = strconv.Atoi(width)

	e.fixValues()

	return e.config.Width
}

// SetHeight sets the transcode frame width from a string.
// This can also be passed into Get() as an int.
func (e *Encoder) SetHeight(height string) int {
	e.config.Height, _ = strconv.Atoi(height)

	e.fixValues()

	return e.config.Height
}

// SetCRF sets the h264 transcode CRF value from a string.
// This can also be passed into Get() as an int.
func (e *Encoder) SetCRF(crf string) int {
	e.config.CRF, _ = strconv.Atoi(crf)

	e.fixValues()

	return e.config.CRF
}

// SetTime sets the maximum transcode duration from a string representing seconds.
// This can also be passed into Get() as an int.
func (e *Encoder) SetTime(seconds string) int {
	e.config.Time, _ = strconv.Atoi(seconds)

	e.fixValues()

	return e.config.Time
}

// SetRate sets the transcode framerate from a string.
// This can also be passed into Get() as an int.
func (e *Encoder) SetRate(rate string) int {
	e.config.Rate, _ = strconv.Atoi(rate)

	e.fixValues()

	return e.config.Rate
}

// SetSize sets the maximum transcode file size as a string.
// This can also be passed into Get() as an int64.
func (e *Encoder) SetSize(size string) int64 {
	e.config.Size, _ = strconv.ParseInt(size, 10, 64) //nolint:gomnd,nolintlint

	e.fixValues()

	return e.config.Size
}

// getVideoHandle is a helper function that creates and returns an ffmpeg command.
// This is used by higher level function to cobble together an input stream.
func (e *Encoder) getVideoHandle(input, output, title string) (string, *exec.Cmd) {
	if title == "" {
		title = filepath.Base(output)
	}

	// the order of these values is important.
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
		arg = append(arg, "-fs", strconv.FormatInt(e.config.Size, 10)) //nolint:gomnd,nolintlint
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

	arg = append(arg, output) // save file path goes last.

	return strings.Join(arg, " "), exec.Command(arg[0], arg[1:]...) //nolint:Gosec
}

// GetVideo retreives video from an input and returns an io.ReadCloser to consume the output.
// Input must be an RTSP URL. Title is encoded into the video as the "movie title."
// Returns command used, io.ReadCloser and error or nil.
func (e *Encoder) GetVideo(input, title string) (string, io.ReadCloser, error) {
	if input == "" {
		return "", nil, ErrInvalidInput
	}

	cmdStr, cmd := e.getVideoHandle(input, "-", title)

	stdoutpipe, err := cmd.StdoutPipe()
	if err != nil {
		return cmdStr, nil, fmt.Errorf("subcommand failed: %w", err)
	}

	if err := cmd.Run(); err != nil {
		return cmdStr, stdoutpipe, fmt.Errorf("run failed: %w", err)
	}

	return cmdStr, stdoutpipe, nil
}

// SaveVideo saves a video snippet to a file.
// Input must be an RTSP URL and output must be a file path. It will be overwritten.
// Returns command used, command output and error or nil.
func (e *Encoder) SaveVideo(input, output, title string) (string, string, error) {
	if input == "" {
		return "", "", ErrInvalidInput
	} else if output == "" || output == "-" {
		return "", "", ErrInvalidOutput
	}

	cmdStr, cmd := e.getVideoHandle(input, output, title)
	// log.Println(cmdStr) // DEBUG

	var out bytes.Buffer
	cmd.Stderr, cmd.Stdout = &out, &out

	if err := cmd.Start(); err != nil {
		return cmdStr, strings.TrimSpace(out.String()), fmt.Errorf("subcommand failed: %w", err)
	} else if err := cmd.Wait(); err != nil {
		return cmdStr, strings.TrimSpace(out.String()), fmt.Errorf("subcommand failed: %w", err)
	}

	return cmdStr, strings.TrimSpace(out.String()), nil
}

// fixValues makes sure video request values are sane.
func (e *Encoder) fixValues() { //nolint:cyclop
	switch {
	case e.config.Height == 0:
		e.config.Height = DefaultFrameHeight
	case e.config.Height > MaximumFrameSize:
		e.config.Height = MaximumFrameSize
	case e.config.Height < MinimumFrameSize:
		e.config.Height = MinimumFrameSize
	}

	switch {
	case e.config.Width == 0:
		e.config.Width = DefaultFrameWidth
	case e.config.Width > MaximumFrameSize:
		e.config.Width = MaximumFrameSize
	case e.config.Width < MinimumFrameSize:
		e.config.Width = MinimumFrameSize
	}

	switch {
	case e.config.CRF == 0:
		e.config.CRF = DefaultEncodeCRF
	case e.config.CRF < MinimumEncodeCRF:
		e.config.CRF = MinimumEncodeCRF
	case e.config.CRF > MaximumEncodeCRF:
		e.config.CRF = MaximumEncodeCRF
	}

	switch {
	case e.config.Rate == 0:
		e.config.Rate = DefaultFrameRate
	case e.config.Rate < MinimumFrameRate:
		e.config.Rate = MinimumFrameRate
	case e.config.Rate > MaximumFrameRate:
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
