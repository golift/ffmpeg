package ffmpeg //nolint:testpackage

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// XXX: test v.Copy, test GetVideo (better)

func TestFixValues(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	encode := Get(&Config{})

	// Test default values.
	assert.False(encode.SetAudio(""), "Wrong default 'audio' value!")
	assert.EqualValues(DefaultProfile, encode.SetProfile(""), "Wrong default 'profile' value!")
	assert.EqualValues(DefaultLevel, encode.SetLevel(""), "Wrong default 'level' value!")
	assert.EqualValues(DefaultFrameHeight, encode.SetHeight(""), "Wrong default 'height' value!")
	assert.EqualValues(DefaultFrameWidth, encode.SetWidth(""), "Wrong default 'width' value!")
	assert.EqualValues(DefaultEncodeCRF, encode.SetCRF(""), "Wrong default 'crf' value!")
	assert.EqualValues(DefaultCaptureTime, encode.SetTime(""), "Wrong default 'time' value!")
	assert.EqualValues(DefaultFrameRate, encode.SetRate(""), "Wrong default 'rate' value!")
	assert.EqualValues(DefaultCaptureSize, encode.SetSize(""), "Wrong default 'size' value!")
	// Text max values.
	assert.EqualValues(MaximumFrameSize, encode.SetHeight("9000"), "Wrong maximum 'height' value!")
	assert.EqualValues(MaximumFrameSize, encode.SetWidth("9000"), "Wrong maximum 'width' value!")
	assert.EqualValues(MaximumEncodeCRF, encode.SetCRF("9000"), "Wrong maximum 'crf' value!")
	assert.EqualValues(MaximumCaptureTime, encode.SetTime("9000"), "Wrong maximum 'time' value!")
	assert.EqualValues(MaximumFrameRate, encode.SetRate("9000"), "Wrong maximum 'rate' value!")
	assert.EqualValues(MaximumCaptureSize, encode.SetSize("999999999"), "Wrong maximum 'size' value!")
	// Text min values.
	assert.EqualValues(MinimumFrameSize, encode.SetHeight("1"), "Wrong minimum 'height' value!")
	assert.EqualValues(MinimumFrameSize, encode.SetWidth("1"), "Wrong minimum 'width' value!")
	assert.EqualValues(MinimumEncodeCRF, encode.SetCRF("1"), "Wrong minimum 'CRF' value!")
	assert.EqualValues(MinimumFrameRate, encode.SetRate("-1"), "Wrong minimum 'rate' value!")
}

func TestSaveVideo(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	encode := Get(&Config{FFMPEG: "echo"})
	fileTemp := "/tmp/go-securityspy-encode-test-12345.txt"

	cmd, out, err := encode.SaveVideo("INPUT", fileTemp, "TITLE")
	assert.Nil(err, "echo returned an error. Something may be wrong with your environment.")

	// Make sure the produced command has all the expected values.
	assert.Contains(cmd, "-an", "Audio may not be correctly disabled.")
	assert.Contains(cmd,
		"-rtsp_transport tcp -i INPUT", "INPUT value appears to be missing, or rtsp transport is out of order")
	assert.Contains(cmd, "-metadata title=\"TITLE\"", "TITLE value appears to be missing.")
	assert.Contains(cmd, fmt.Sprintf("-vcodec libx264 -profile:v %v -level %v", DefaultProfile, DefaultLevel),
		"Level or Profile are missing or out of order.")
	assert.Contains(cmd, fmt.Sprintf("-crf %d", DefaultEncodeCRF), "CRF value is missing or malformed.")
	assert.Contains(cmd, fmt.Sprintf("-t %d", DefaultCaptureTime),
		"Capture Time value is missing or malformed.")
	assert.Contains(cmd, fmt.Sprintf("-s %dx%d", DefaultFrameWidth, DefaultFrameHeight),
		"Framesize is missing or malformed.")
	assert.Contains(cmd, fmt.Sprintf("-r %d", DefaultFrameRate), "Frame Rate value is missing or malformed.")
	assert.Contains(cmd, fmt.Sprintf("-fs %d", DefaultCaptureSize), "Size value is missing or malformed.")
	assert.True(strings.HasPrefix(cmd, "echo"), "The command does not - but should - begin with the Encoder value.")
	assert.True(strings.HasSuffix(cmd, fileTemp),
		"The command does not - but should - end with a dash to indicate output to stdout.")
	assert.EqualValues(cmd+"\n", "echo "+out, "Somehow the wrong value was written")

	// Make sure audio can be turned on.
	encode = Get(&Config{FFMPEG: "echo", Audio: true})
	cmd, _, err = encode.GetVideo("INPUT", "TITLE")

	assert.Nil(err, "echo returned an error. Something may be wrong with your environment.")
	assert.Contains(cmd, "-c:a copy", "Audio may not be correctly enabled.")
}

func TestValues(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	config := Get(&Config{}).Config()

	assert.EqualValues(DefaultFFmpegPath, config.FFMPEG)
	assert.EqualValues(DefaultFrameRate, config.Rate)
	assert.EqualValues(DefaultFrameHeight, config.Height)
	assert.EqualValues(DefaultFrameWidth, config.Width)
	assert.EqualValues(DefaultEncodeCRF, config.CRF)
	assert.EqualValues(DefaultCaptureTime, config.Time)
	assert.EqualValues(DefaultCaptureSize, config.Size)
	assert.EqualValues(DefaultProfile, config.Prof)
	assert.EqualValues(DefaultLevel, config.Level)
}

/* GoDoc Code Examples */

// Example non-transcode direct-save from securityspy.
func Example_securitySpy() { //nolint:testableexamples
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
func Example_dahua() { //nolint:testableexamples
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
