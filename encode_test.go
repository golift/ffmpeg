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

	asert := assert.New(t)
	encode := Get(&Config{})

	// Test default values.
	asert.False(encode.SetAudio(""), "Wrong default 'audio' value!")
	asert.Equal(DefaultProfile, encode.SetProfile(""), "Wrong default 'profile' value!")
	asert.Equal(DefaultLevel, encode.SetLevel(""), "Wrong default 'level' value!")
	asert.Equal(DefaultFrameHeight, encode.SetHeight(""), "Wrong default 'height' value!")
	asert.Equal(DefaultFrameWidth, encode.SetWidth(""), "Wrong default 'width' value!")
	asert.Equal(DefaultEncodeCRF, encode.SetCRF(""), "Wrong default 'crf' value!")
	asert.Equal(DefaultCaptureTime, encode.SetTime(""), "Wrong default 'time' value!")
	asert.Equal(DefaultFrameRate, encode.SetRate(""), "Wrong default 'rate' value!")
	asert.Equal(DefaultCaptureSize, encode.SetSize(""), "Wrong default 'size' value!")
	// Text max values.
	asert.Equal(MaximumFrameSize, encode.SetHeight("9000"), "Wrong maximum 'height' value!")
	asert.Equal(MaximumFrameSize, encode.SetWidth("9000"), "Wrong maximum 'width' value!")
	asert.Equal(MaximumEncodeCRF, encode.SetCRF("9000"), "Wrong maximum 'crf' value!")
	asert.Equal(MaximumCaptureTime, encode.SetTime("9000"), "Wrong maximum 'time' value!")
	asert.Equal(MaximumFrameRate, encode.SetRate("9000"), "Wrong maximum 'rate' value!")
	asert.Equal(MaximumCaptureSize, encode.SetSize("999999999"), "Wrong maximum 'size' value!")
	// Text min values.
	asert.Equal(MinimumFrameSize, encode.SetHeight("1"), "Wrong minimum 'height' value!")
	asert.Equal(MinimumFrameSize, encode.SetWidth("1"), "Wrong minimum 'width' value!")
	asert.Equal(MinimumEncodeCRF, encode.SetCRF("1"), "Wrong minimum 'CRF' value!")
	asert.Equal(MinimumFrameRate, encode.SetRate("-1"), "Wrong minimum 'rate' value!")
}

func TestSaveVideo(t *testing.T) {
	t.Parallel()

	asert := assert.New(t)
	encode := Get(&Config{FFMPEG: "echo"})
	fileTemp := "/tmp/go-securityspy-encode-test-12345.txt"

	cmd, out, err := encode.SaveVideo("INPUT", fileTemp, "TITLE")
	require.NoError(t, err, "echo returned an error. Something may be wrong with your environment.")

	// Make sure the produced command has all the expected values.
	asert.Contains(cmd, "-an", "Audio may not be correctly disabled.")
	asert.Contains(cmd, "-i INPUT", "INPUT value appears to be missing")
	asert.Contains(cmd, "-metadata title=TITLE", "TITLE value appears to be missing.")
	asert.Contains(cmd, fmt.Sprintf("-vcodec libx264 -profile:v %v -level %v", DefaultProfile, DefaultLevel),
		"Level or Profile are missing or out of order.")
	asert.Contains(cmd, "-f mov", "File output should use mov container.")
	asert.Contains(cmd, "-movflags faststart", "File output should set faststart for mov.")
	asert.Contains(cmd, fmt.Sprintf("-crf %d", DefaultEncodeCRF), "CRF value is missing or malformed.")
	asert.Contains(cmd, fmt.Sprintf("-t %d", DefaultCaptureTime),
		"Capture Time value is missing or malformed.")
	asert.Contains(cmd, fmt.Sprintf("-s %dx%d", DefaultFrameWidth, DefaultFrameHeight),
		"Framesize is missing or malformed.")
	asert.Contains(cmd, fmt.Sprintf("-r %d", DefaultFrameRate), "Frame Rate value is missing or malformed.")
	asert.Contains(cmd, fmt.Sprintf("-fs %d", DefaultCaptureSize), "Size value is missing or malformed.")
	asert.True(strings.HasPrefix(cmd, "echo"), "The command does not - but should - begin with the Encoder value.")
	asert.True(strings.HasSuffix(cmd, fileTemp),
		"The command does not - but should - end with a dash to indicate output to stdout.")
	asert.Equal(cmd, "echo "+strings.TrimSpace(out), "Somehow the wrong value was written")

	// Make sure audio can be turned on.
	encode = Get(&Config{FFMPEG: "echo", Audio: true})
	cmd, _, err = encode.GetVideo("INPUT", "TITLE")

	require.NoError(t, err, "echo returned an error. Something may be wrong with your environment.")
	asert.Contains(cmd, "-c:a copy", "Audio may not be correctly enabled.")
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

	asert := assert.New(t)
	config := Get(&Config{}).Config()

	asert.Equal(DefaultFFmpegPath, config.FFMPEG)
	asert.Equal(DefaultFrameRate, config.Rate)
	asert.Equal(DefaultFrameHeight, config.Height)
	asert.Equal(DefaultFrameWidth, config.Width)
	asert.Equal(DefaultEncodeCRF, config.CRF)
	asert.Equal(DefaultCaptureTime, config.Time)
	asert.Equal(DefaultCaptureSize, config.Size)
	asert.Equal(DefaultProfile, config.Prof)
	asert.Equal(DefaultLevel, config.Level)
}

/* GoDoc Code Examples */

// Example non-transcode direct-save from securityspy.
func Example_securitySpy() { //nolint:testableexamples // it's an example.
	securitypsy := "rtsp://user:pass@127.0.0.1:8000/++stream?cameraNum=1"
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
	dahua := "rtsp://admin:password@192.168.1.12/live"
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
