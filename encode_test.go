package ffmpeg_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golift.io/ffmpeg"
)

const echoCommand = "echo"

func TestFixValues(t *testing.T) {
	t.Parallel()

	check := assert.New(t)
	encode := ffmpeg.Get(&ffmpeg.Config{})

	// Test default values.
	check.False(encode.SetAudio(""), "Wrong default 'audio' value!")
	check.Equal(ffmpeg.DefaultProfile, encode.SetProfile(""), "Wrong default 'profile' value!")
	check.Equal(ffmpeg.DefaultLevel, encode.SetLevel(""), "Wrong default 'level' value!")
	check.Equal(ffmpeg.DefaultFrameHeight, encode.SetHeight(""), "Wrong default 'height' value!")
	check.Equal(ffmpeg.DefaultFrameWidth, encode.SetWidth(""), "Wrong default 'width' value!")
	check.Equal(ffmpeg.DefaultEncodeCRF, encode.SetCRF(""), "Wrong default 'crf' value!")
	check.Equal(ffmpeg.DefaultCaptureTime, encode.SetTime(""), "Wrong default 'time' value!")
	check.Equal(ffmpeg.DefaultFrameRate, encode.SetRate(""), "Wrong default 'rate' value!")
	check.Equal(ffmpeg.DefaultCaptureSize, encode.SetSize(""), "Wrong default 'size' value!")
	// Text max values.
	check.Equal(ffmpeg.MaximumFrameSize, encode.SetHeight("9000"), "Wrong maximum 'height' value!")
	check.Equal(ffmpeg.MaximumFrameSize, encode.SetWidth("9000"), "Wrong maximum 'width' value!")
	check.Equal(ffmpeg.MaximumEncodeCRF, encode.SetCRF("9000"), "Wrong maximum 'crf' value!")
	check.Equal(ffmpeg.MaximumCaptureTime, encode.SetTime("9000"), "Wrong maximum 'time' value!")
	check.Equal(ffmpeg.MaximumFrameRate, encode.SetRate("9000"), "Wrong maximum 'rate' value!")
	check.Equal(ffmpeg.MaximumCaptureSize, encode.SetSize("999999999"), "Wrong maximum 'size' value!")
	// Text min values.
	check.Equal(ffmpeg.MinimumFrameSize, encode.SetHeight("1"), "Wrong minimum 'height' value!")
	check.Equal(ffmpeg.MinimumFrameSize, encode.SetWidth("1"), "Wrong minimum 'width' value!")
	check.Equal(ffmpeg.MinimumEncodeCRF, encode.SetCRF("1"), "Wrong minimum 'CRF' value!")
	check.Equal(ffmpeg.MinimumFrameRate, encode.SetRate("-1"), "Wrong minimum 'rate' value!")
}

func TestSaveVideo(t *testing.T) {
	t.Parallel()

	check := assert.New(t)
	encode := ffmpeg.Get(&ffmpeg.Config{FFMPEG: echoCommand})
	fileTemp := "/tmp/go-securityspy-encode-test-12345.txt"

	cmd, out, err := encode.SaveVideo("INPUT", fileTemp, "TITLE")
	require.NoError(t, err, "echo returned an error. Something may be wrong with your environment.")

	// Make sure the produced command has all the expected values.
	check.Contains(cmd, "-an", "Audio may not be correctly disabled.")
	check.Contains(cmd, "-i INPUT", "INPUT value appears to be missing")
	check.Contains(cmd, "-metadata title=TITLE", "TITLE value appears to be missing.")
	check.Contains(cmd, fmt.Sprintf("-vcodec libx264 -profile:v %v -level %v", ffmpeg.DefaultProfile, ffmpeg.DefaultLevel),
		"Level or Profile are missing or out of order.")
	check.Contains(cmd, "-f mov", "File output should use mov container.")
	check.Contains(cmd, "-movflags faststart", "File output should set faststart for mov.")
	check.Contains(cmd, fmt.Sprintf("-crf %d", ffmpeg.DefaultEncodeCRF), "CRF value is missing or malformed.")
	check.Contains(cmd, fmt.Sprintf("-t %d", ffmpeg.DefaultCaptureTime),
		"Capture Time value is missing or malformed.")
	check.Contains(cmd, fmt.Sprintf("-s %dx%d", ffmpeg.DefaultFrameWidth, ffmpeg.DefaultFrameHeight),
		"Framesize is missing or malformed.")
	check.Contains(cmd, fmt.Sprintf("-r %d", ffmpeg.DefaultFrameRate), "Frame Rate value is missing or malformed.")
	check.Contains(cmd, fmt.Sprintf("-fs %d", ffmpeg.DefaultCaptureSize), "Size value is missing or malformed.")
	check.True(strings.HasPrefix(cmd, echoCommand), "The command does not - but should - begin with the Encoder value.")
	check.True(strings.HasSuffix(cmd, fileTemp),
		"The command does not - but should - end with a dash to indicate output to stdout.")
	check.Equal(cmd, echoCommand+" "+strings.TrimSpace(out), "Somehow the wrong value was written")

	// Make sure audio can be turned on.
	encode = ffmpeg.Get(&ffmpeg.Config{FFMPEG: echoCommand, Audio: true})
	cmd, _, err = encode.GetVideo("INPUT", "TITLE")

	require.NoError(t, err, "echo returned an error. Something may be wrong with your environment.")
	check.Contains(cmd, "-c:a copy", "Audio may not be correctly enabled.")
}

func TestRTSPTransportOption(t *testing.T) {
	t.Parallel()

	encode := ffmpeg.Get(&ffmpeg.Config{FFMPEG: echoCommand})

	rtspCmd, _, err := encode.SaveVideo("rtsp://example.local/stream", "/tmp/out.mov", "TITLE")
	require.NoError(t, err)
	require.Contains(t, rtspCmd, "-rtsp_transport tcp")

	httpsCmd, _, err := encode.SaveVideo("https://example.local/++video", "/tmp/out.mov", "TITLE")
	require.NoError(t, err)
	require.NotContains(t, httpsCmd, "-rtsp_transport tcp")
}

func TestSaveVideoErrors(t *testing.T) {
	t.Parallel()

	encode := ffmpeg.Get(&ffmpeg.Config{FFMPEG: echoCommand})
	_, _, err := encode.SaveVideoContext(context.Background(), "", "/tmp/nope", "title")
	require.ErrorIs(t, err, ffmpeg.ErrInvalidInput)

	_, _, err = encode.SaveVideoContext(context.Background(), "INPUT", "", "title")
	require.ErrorIs(t, err, ffmpeg.ErrInvalidOutput)

	_, _, err = encode.SaveVideoContext(context.Background(), "INPUT", "-", "title")
	require.ErrorIs(t, err, ffmpeg.ErrInvalidOutput)
}

func TestGetVideoContextErrors(t *testing.T) {
	t.Parallel()

	encode := ffmpeg.Get(&ffmpeg.Config{FFMPEG: echoCommand})
	_, _, err := encode.GetVideoContext(context.Background(), "", "title")
	require.ErrorIs(t, err, ffmpeg.ErrInvalidInput)

	encode = ffmpeg.Get(&ffmpeg.Config{FFMPEG: "/path/that/does/not/exist/ffmpeg"})
	_, stream, err := encode.GetVideoContext(context.Background(), "INPUT", "title")
	require.Error(t, err)
	require.Nil(t, stream)
	require.Contains(t, err.Error(), "run failed")
}

func TestGetVideoStreamLifecycle(t *testing.T) {
	t.Parallel()

	encode := ffmpeg.Get(&ffmpeg.Config{FFMPEG: echoCommand})
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

	encode := ffmpeg.Get(&ffmpeg.Config{
		FFMPEG: echoCommand,
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

	config := ffmpeg.Get(nil).Config()
	require.Equal(t, ffmpeg.DefaultFFmpegPath, config.FFMPEG)
	require.Equal(t, ffmpeg.DefaultFrameRate, config.Rate)
}

func TestValues(t *testing.T) {
	t.Parallel()

	check := assert.New(t)
	config := ffmpeg.Get(&ffmpeg.Config{}).Config()

	check.Equal(ffmpeg.DefaultFFmpegPath, config.FFMPEG)
	check.Equal(ffmpeg.DefaultFrameRate, config.Rate)
	check.Equal(ffmpeg.DefaultFrameHeight, config.Height)
	check.Equal(ffmpeg.DefaultFrameWidth, config.Width)
	check.Equal(ffmpeg.DefaultEncodeCRF, config.CRF)
	check.Equal(ffmpeg.DefaultCaptureTime, config.Time)
	check.Equal(ffmpeg.DefaultCaptureSize, config.Size)
	check.Equal(ffmpeg.DefaultProfile, config.Prof)
	check.Equal(ffmpeg.DefaultLevel, config.Level)
}

/* GoDoc Code Examples */

// Example non-transcode direct-save from securityspy.
func Example_securitySpy() { //nolint:testableexamples // it's an example.
	securitypsy := "rtsp://user:pass@127.0.0.1:8000/++stream?cameraNum=1" //nolint:gosec // it's an example.
	output := "/tmp/securitypsy_captured_file.mov"
	config := &ffmpeg.Config{
		FFMPEG: "/usr/local/bin/ffmpeg",
		Copy:   true, // do not transcode
		Audio:  true, // retain audio stream
		Time:   10,   // 10 seconds
	}
	encode := ffmpeg.Get(config)
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
	encode := ffmpeg.Get(&ffmpeg.Config{
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
