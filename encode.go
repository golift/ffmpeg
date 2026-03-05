// Package ffmpeg captures video from RTSP streams, like IP cameras.
//
// Provides a simple interface to set FFMPEG options and capture video from an RTSP source.
package ffmpeg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Default, Maximum and Minimum Values for encoder configuration. Change these if your needs differ.
//
//nolint:gochecknoglobals,mnd // these are constants, not variables, but configurable by a consumer.
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
)

// Custom errors that this library outputs. The library also outputs errors created elsewhere.
var (
	ErrInvalidOutput = errors.New("output path is not valid")
	ErrInvalidInput  = errors.New("input path is not valid")
)

const (
	bits64 = 64
	base10 = 10
	// Keep the last bytes of ffmpeg stderr to improve diagnostics.
	defaultStderrTail = 8192
	// Account for slow live streams: processing may take longer than clip length.
	minCommandTimeout = 30 * time.Second
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
	cfg := &Config{}
	if config != nil {
		*cfg = *config
	}

	encode := &Encoder{config: cfg}
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
	e.config.Size, _ = strconv.ParseInt(size, base10, bits64)

	e.fixValues()

	return e.config.Size
}

// GetVideo retreives video from an input and returns an io.ReadCloser to consume the output.
// Input must be an RTSP URL. Title is encoded into the video as the "movie title."
// Returns command used for diagnostics, io.ReadCloser and error or nil.
// This will automatically create a context timeout based on the requested capture length.
// For full timeout control, use GetVideoContext().
// If you want to control the context, use GetVideoContext().
func (e *Encoder) GetVideo(input, title string) (string, io.ReadCloser, error) {
	ctx := context.Background()

	var cancel context.CancelFunc

	if e.config.Time > 0 {
		ctx, cancel = context.WithTimeout(ctx, captureTimeout(e.config.Time))
	}

	cmdStr, stream, err := e.GetVideoContext(ctx, input, title)
	if err != nil {
		if cancel != nil {
			cancel()
		}

		return cmdStr, nil, err
	}

	if cancel == nil {
		return cmdStr, stream, nil
	}

	return cmdStr, &cancelReadCloser{ReadCloser: stream, cancel: cancel}, nil
}

// GetVideoContext retreives video from an input and returns an io.ReadCloser to consume the output.
// Input must be an RTSP URL. Title is encoded into the video as the "movie title."
// Returns command used for diagnostics, io.ReadCloser and error or nil.
// Use the context to add a timeout value (max run duration) to the ffmpeg command.
//
//nolint:contextcheck // caller-provided context is accepted and used for command execution.
func (e *Encoder) GetVideoContext(ctx context.Context, input, title string) (string, io.ReadCloser, error) {
	if input == "" {
		return "", nil, ErrInvalidInput
	}

	if ctx == nil {
		ctx = context.Background()
	}

	cmdCtx, cmdCancel := context.WithCancel(ctx)
	cmdStr, cmd := e.getVideoHandle(cmdCtx, input, "-", title)
	stderr := newTailBuffer(defaultStderrTail)
	cmd.Stderr = stderr

	stdoutpipe, err := cmd.StdoutPipe()
	if err != nil {
		cmdCancel()

		return cmdStr, nil, fmt.Errorf("subcommand failed: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		_ = stdoutpipe.Close()

		cmdCancel()

		return cmdStr, nil, withStderr("run failed", err, stderr.String())
	}

	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	return cmdStr, &streamResult{
		out:       stdoutpipe,
		done:      done,
		cmdCancel: cmdCancel,
		stderr:    stderr,
	}, nil
}

// SaveVideo saves a video snippet to a file.
// Input must be an RTSP URL and output must be a file path. It will be overwritten.
// Returns command used for diagnostics, command output and error or nil.
// This will automatically create a context timeout based on the requested capture length.
// For full timeout control, use SaveVideoContext().
// If you want to control the context, use SaveVideoContext().
//
//nolint:nonamedreturns // the names help readability.
func (e *Encoder) SaveVideo(input, output, title string) (cmdStr, outputStr string, err error) {
	ctx := context.Background()

	if e.config.Time > 0 {
		var cancel func()

		ctx, cancel = context.WithTimeout(ctx, captureTimeout(e.config.Time))
		defer cancel()
	}

	return e.SaveVideoContext(ctx, input, output, title)
}

// SaveVideoContext saves a video snippet to a file using a provided context.
// Input must be an RTSP URL and output must be a file path. It will be overwritten.
// Returns command used for diagnostics, command output and error or nil.
// Use the context to add a timeout value (max run duration) to the ffmpeg command.
//
//nolint:nonamedreturns // the names help readability.
func (e *Encoder) SaveVideoContext(
	ctx context.Context, input, output, title string,
) (cmdStr, outputStr string, err error) {
	if input == "" {
		return "", "", ErrInvalidInput
	}

	if output == "" || output == "-" {
		return "", "", ErrInvalidOutput
	}

	cmdStr, cmd := e.getVideoHandle(ctx, input, output, title)
	stderr := newTailBuffer(defaultStderrTail)

	var stdout bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	if err != nil {
		return cmdStr, stdout.String(), runError(ctx, "subcommand failed", err, stderr.String())
	}

	return cmdStr, stdout.String(), nil
}

// fixValues makes sure video request values are sane.
func (e *Encoder) fixValues() { //nolint:cyclop // it's a simple switch statement.
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

// getVideoHandle is a helper function that creates and returns an ffmpeg command.
// This is used by higher level function to cobble together an input stream.
func (e *Encoder) getVideoHandle(ctx context.Context, input, output, title string) (string, *exec.Cmd) {
	if title == "" {
		title = filepath.Base(output)
	}

	// the order of these values is important.
	arg := []string{
		e.config.FFMPEG,
		"-v", "16", // log level
		"-i", input,
		"-metadata", "title=" + title,
		"-y", "-map", "0",
	}

	if isRTSP(input) {
		arg = append(arg[:3], append([]string{"-rtsp_transport", "tcp"}, arg[3:]...)...)
	}

	if output == "-" {
		arg = append(arg, "-f", "mp4", "-movflags", "frag_keyframe+empty_moov")
	} else {
		arg = append(arg, "-f", "mov")
	}

	if e.config.Size > 0 {
		arg = append(arg, "-fs", strconv.FormatInt(e.config.Size, base10))
	}

	if e.config.Time > 0 {
		arg = append(arg, "-t", strconv.Itoa(e.config.Time))
	}

	if !e.config.Copy {
		arg = append(arg, "-vcodec", "libx264",
			"-profile:v", e.config.Prof,
			"-level", e.config.Level,
			"-pix_fmt", "yuv420p",
			"-s", strconv.Itoa(e.config.Width)+"x"+strconv.Itoa(e.config.Height),
			"-preset", "superfast",
			"-crf", strconv.Itoa(e.config.CRF),
			"-r", strconv.Itoa(e.config.Rate),
		)

		if output != "-" {
			arg = append(arg, "-movflags", "faststart")
		}
	} else {
		arg = append(arg, "-c", "copy")
	}

	if !e.config.Audio {
		arg = append(arg, "-an")
	} else {
		arg = append(arg, "-c:a", "copy")
	}

	arg = append(arg, output) // save file path goes last.

	// This command string is for diagnostics only; it is not shell-escaped.
	//nolint:gosec // it's ok, but maybe it's not.
	return strings.Join(arg, " "), exec.CommandContext(ctx, arg[0], arg[1:]...)
}

// streamResult is our custom io.ReadCloser that also cleans up the command and context.
type streamResult struct {
	out       io.ReadCloser
	done      <-chan error
	cmdCancel context.CancelFunc
	stderr    *tailBuffer
	closeOnce sync.Once
	closeErr  error
}

func (s *streamResult) Read(data []byte) (int, error) {
	bytesRead, err := s.out.Read(data)
	if err == nil {
		return bytesRead, nil
	}

	if errors.Is(err, io.EOF) {
		return bytesRead, io.EOF
	}

	if err != nil {
		return bytesRead, fmt.Errorf("read stream: %w", err)
	}

	return bytesRead, nil
}

func (s *streamResult) Close() error {
	s.closeOnce.Do(func() {
		s.cmdCancel()

		_ = s.out.Close()

		waitErr := <-s.done
		if waitErr != nil && !errors.Is(waitErr, context.Canceled) {
			s.closeErr = withStderr("run failed", waitErr, s.stderr.String())
		}
	})

	return s.closeErr
}

type tailBuffer struct {
	buf []byte
	max int
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{max: limit}
}

func (t *tailBuffer) Write(data []byte) (int, error) {
	if t.max <= 0 {
		return len(data), nil
	}

	if len(data) >= t.max {
		t.buf = append(t.buf[:0], data[len(data)-t.max:]...)

		return len(data), nil
	}

	need := len(t.buf) + len(data) - t.max
	if need > 0 {
		t.buf = append(t.buf[:0], t.buf[need:]...)
	}

	t.buf = append(t.buf, data...)

	return len(data), nil
}

func (t *tailBuffer) String() string {
	return strings.TrimSpace(string(t.buf))
}

func withStderr(prefix string, err error, stderr string) error {
	if stderr == "" {
		return fmt.Errorf("%s: %w", prefix, err)
	}

	return fmt.Errorf("%s: %w: %s", prefix, err, stderr)
}

func runError(ctx context.Context, prefix string, err error, stderr string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return withStderr(prefix+": ffmpeg command timed out", err, stderr)
	}

	if errors.Is(ctx.Err(), context.Canceled) {
		return withStderr(prefix+": ffmpeg command canceled", err, stderr)
	}

	return withStderr(prefix, err, stderr)
}

func isRTSP(input string) bool {
	parsedURL, err := url.Parse(input)
	if err != nil {
		return strings.HasPrefix(strings.ToLower(input), "rtsp://") ||
			strings.HasPrefix(strings.ToLower(input), "rtsps://")
	}

	return parsedURL.Scheme == "rtsp" || parsedURL.Scheme == "rtsps"
}

func captureTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return minCommandTimeout
	}

	const timeoutMultiplier = 6

	timeout := time.Duration(seconds*timeoutMultiplier) * time.Second
	timeout = max(timeout, minCommandTimeout)

	return timeout
}

type cancelReadCloser struct {
	io.ReadCloser

	cancel    context.CancelFunc
	closeOnce sync.Once
}

func (c *cancelReadCloser) Close() error {
	var err error

	c.closeOnce.Do(func() {
		err = c.ReadCloser.Close()
		c.cancel()
	})

	if err != nil {
		return fmt.Errorf("close stream: %w", err)
	}

	return nil
}
