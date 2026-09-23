package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixValues(t *testing.T) {
	t.Parallel()

	check := assert.New(t)
	encode := Get(&Config{})

	// Test default values.
	check.False(encode.SetAudio(""), "Wrong default 'audio' value!")
	check.Equal(DefaultProfile, encode.SetProfile(""), "Wrong default 'profile' value!")
	check.Equal(DefaultLevel, encode.SetLevel(""), "Wrong default 'level' value!")
	check.Equal(DefaultFrameHeight, encode.SetHeight(""), "Wrong default 'height' value!")
	check.Equal(DefaultFrameWidth, encode.SetWidth(""), "Wrong default 'width' value!")
	check.Equal(DefaultEncodeCRF, encode.SetCRF(""), "Wrong default 'crf' value!")
	check.Equal(DefaultCaptureTime, encode.SetTime(""), "Wrong default 'time' value!")
	check.Equal(DefaultFrameRate, encode.SetRate(""), "Wrong default 'rate' value!")
	check.Equal(DefaultCaptureSize, encode.SetSize(""), "Wrong default 'size' value!")
	// Text max values.
	check.Equal(MaximumFrameSize, encode.SetHeight("9000"), "Wrong maximum 'height' value!")
	check.Equal(MaximumFrameSize, encode.SetWidth("9000"), "Wrong maximum 'width' value!")
	check.Equal(MaximumEncodeCRF, encode.SetCRF("9000"), "Wrong maximum 'crf' value!")
	check.Equal(MaximumCaptureTime, encode.SetTime("9000"), "Wrong maximum 'time' value!")
	check.Equal(MaximumFrameRate, encode.SetRate("9000"), "Wrong maximum 'rate' value!")
	check.Equal(MaximumCaptureSize, encode.SetSize("999999999"), "Wrong maximum 'size' value!")
	// Text min values.
	check.Equal(MinimumFrameSize, encode.SetHeight("1"), "Wrong minimum 'height' value!")
	check.Equal(MinimumFrameSize, encode.SetWidth("1"), "Wrong minimum 'width' value!")
	check.Equal(MinimumEncodeCRF, encode.SetCRF("1"), "Wrong minimum 'CRF' value!")
	check.Equal(MinimumFrameRate, encode.SetRate("-1"), "Wrong minimum 'rate' value!")
}

func TestSaveVideo(t *testing.T) {
	t.Parallel()

	check := assert.New(t)
	encode := Get(&Config{FFMPEG: "echo"})
	fileTemp := "/tmp/go-securityspy-encode-test-12345.txt"

	cmd, out, err := encode.SaveVideo("INPUT", fileTemp, "TITLE")
	require.NoError(t, err, "echo returned an error. Something may be wrong with your environment.")

	// Make sure the produced command has all the expected values.
	check.Contains(cmd, "-an", "Audio may not be correctly disabled.")
	check.Contains(cmd, "-i INPUT", "INPUT value appears to be missing")
	check.Contains(cmd, "-metadata title=TITLE", "TITLE value appears to be missing.")
	check.Contains(cmd, fmt.Sprintf("-vcodec libx264 -profile:v %v -level %v", DefaultProfile, DefaultLevel),
		"Level or Profile are missing or out of order.")
	check.Contains(cmd, "-f mov", "File output should use mov container.")
	check.Contains(cmd, "-movflags faststart", "File output should set faststart for mov.")
	check.Contains(cmd, fmt.Sprintf("-crf %d", DefaultEncodeCRF), "CRF value is missing or malformed.")
	check.Contains(cmd, fmt.Sprintf("-t %d", DefaultCaptureTime),
		"Capture Time value is missing or malformed.")
	check.Contains(cmd, fmt.Sprintf("-s %dx%d", DefaultFrameWidth, DefaultFrameHeight),
		"Framesize is missing or malformed.")
	check.Contains(cmd, fmt.Sprintf("-r %d", DefaultFrameRate), "Frame Rate value is missing or malformed.")
	check.Contains(cmd, fmt.Sprintf("-fs %d", DefaultCaptureSize), "Size value is missing or malformed.")
	check.True(strings.HasPrefix(cmd, "echo"), "The command does not - but should - begin with the Encoder value.")
	check.True(strings.HasSuffix(cmd, fileTemp),
		"The command does not - but should - end with a dash to indicate output to stdout.")
	check.Equal(cmd, "echo "+strings.TrimSpace(out), "Somehow the wrong value was written")

	// Make sure audio can be turned on.
	encode = Get(&Config{FFMPEG: "echo", Audio: true})
	cmd, _, err = encode.GetVideo("INPUT", "TITLE")

	require.NoError(t, err, "echo returned an error. Something may be wrong with your environment.")
	check.Contains(cmd, "-c:a copy", "Audio may not be correctly enabled.")
}

func TestRTSPTransportOption(t *testing.T) {
	t.Parallel()

	encode := Get(&Config{FFMPEG: "echo"})

	rtspCmd, _, err := encode.SaveVideo("rtsp://example.local/stream", "/tmp/out.mov", "TITLE")
	require.NoError(t, err)
	require.Contains(t, rtspCmd, "-rtsp_transport tcp")

	httpsCmd, _, err := encode.SaveVideo("https://example.local/++video", "/tmp/out.mov", "TITLE")
	require.NoError(t, err)
	require.NotContains(t, httpsCmd, "-rtsp_transport tcp")
}

func TestSaveVideoErrors(t *testing.T) {
	t.Parallel()

	encode := Get(&Config{FFMPEG: "echo"})
	_, _, err := encode.SaveVideoContext(context.Background(), "", "/tmp/nope", "title")
	require.ErrorIs(t, err, ErrInvalidInput)

	_, _, err = encode.SaveVideoContext(context.Background(), "INPUT", "", "title")
	require.ErrorIs(t, err, ErrInvalidOutput)

	_, _, err = encode.SaveVideoContext(context.Background(), "INPUT", "-", "title")
	require.ErrorIs(t, err, ErrInvalidOutput)
}

func TestGetVideoContextErrors(t *testing.T) {
	t.Parallel()

	encode := Get(&Config{FFMPEG: "echo"})
	_, _, err := encode.GetVideoContext(context.Background(), "", "title")
	require.ErrorIs(t, err, ErrInvalidInput)

	encode = Get(&Config{FFMPEG: "/path/that/does/not/exist/ffmpeg"})
	_, stream, err := encode.GetVideoContext(context.Background(), "INPUT", "title")
	require.Error(t, err)
	require.Nil(t, stream)
	require.Contains(t, err.Error(), "run failed")
}

func TestGetVideoStreamLifecycle(t *testing.T) {
	t.Parallel()

	encode := Get(&Config{FFMPEG: "echo"})
	cmd, stream, err := encode.GetVideoContext(context.Background(), "INPUT", "TITLE")
	require.NoError(t, err)
	require.NotNil(t, stream)

	data, readErr := io.ReadAll(stream)
	require.NoError(t, readErr)
	require.NotEmpty(t, data)
	require.Contains(t, string(data), "-metadata title=TITLE")
	require.Contains(t, string(data), "-f mp4")
	require.Contains(t, string(data), "-movflags frag_keyframe+empty_moov")
	require.NotContains(t, string(data), "-movflags faststart")
	require.NoError(t, stream.Close())
	require.Contains(t, cmd, "-metadata title=TITLE")
}

func TestGetVideoTitleFallbackAndCopy(t *testing.T) {
	t.Parallel()

	encode := Get(&Config{
		FFMPEG: "echo",
		Copy:   true,
		Audio:  true,
	})

	cmd, stream, err := encode.GetVideoContext(context.Background(), "INPUT", "")
	require.NoError(t, err)
	require.NotNil(t, stream)
	_, _ = io.ReadAll(stream)
	require.NoError(t, stream.Close())
	require.Contains(t, cmd, "-c copy")
	require.Contains(t, cmd, "-c:a copy")
	require.Contains(t, cmd, "-metadata title=-")
}

func TestGetNilConfig(t *testing.T) {
	t.Parallel()

	config := Get(nil).Config()
	require.Equal(t, DefaultFFmpegPath, config.FFMPEG)
	require.Equal(t, DefaultFrameRate, config.Rate)
}

func TestValues(t *testing.T) {
	t.Parallel()

	check := assert.New(t)
	config := Get(&Config{}).Config()

	check.Equal(DefaultFFmpegPath, config.FFMPEG)
	check.Equal(DefaultFrameRate, config.Rate)
	check.Equal(DefaultFrameHeight, config.Height)
	check.Equal(DefaultFrameWidth, config.Width)
	check.Equal(DefaultEncodeCRF, config.CRF)
	check.Equal(DefaultCaptureTime, config.Time)
	check.Equal(DefaultCaptureSize, config.Size)
	check.Equal(DefaultProfile, config.Prof)
	check.Equal(DefaultLevel, config.Level)
}

/* GoDoc Code Examples */

// Example non-transcode direct-save from securityspy.
func Example_securitySpy() { //nolint:testableexamples // it's an example.
	securitypsy := "rtsp://user:pass@127.0.0.1:8000/++stream?cameraNum=1" //nolint:gosec // it's an example.
	output := "/tmp/securitypsy_captured_file.mov"
	config := &Config{
		FFMPEG: "/usr/local/bin/ffmpeg",
		Copy:   true, // do not transcode
		Audio:  true, // retain audio stream
		Time:   10,   // 10 seconds
	}
	encode := Get(config)
	cmd, out, err := encode.SaveVideo(securitypsy, output, "SecuritySpyVideoTitle")

	log.Println("Command Used:", cmd)
	log.Println("Command Output:", out)

	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Saved file from", securitypsy, "to", output)
}

// Example transcode from a Dahua IP camera.
func Example_dahua() { //nolint:testableexamples // it's an example.
	dahua := "rtsp://admin:password@192.168.1.12/live" //nolint:gosec // it's an example.
	output := "/tmp/dahua_captured_file.m4v"
	encode := Get(&Config{
		Audio:  true, // retain audio stream
		Time:   10,   // 10 seconds
		Width:  1920,
		Height: 1080,
		CRF:    23,
		Level:  "4.0",
		Rate:   5,
		Prof:   "baseline", // or main or high
	})

	cmd, out, err := encode.SaveVideo(dahua, output, "DahuaVideoTitle")

	log.Println("Command Used:", cmd)
	log.Println("Command Output:", out)

	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Saved file from", dahua, "to", output)
}
