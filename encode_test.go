package ffmpeg

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO: test v.Copy, test GetVideo (better)

func TestFixValues(t *testing.T) {
	a := assert.New(t)
	e := Get(&Config{})

	// Test default values.
	a.False(e.SetAudio(""), "Wrong default 'audio' value!")
	a.EqualValues(DefaultProfile, e.SetProfile(""), "Wrong default 'profile' value!")
	a.EqualValues(DefaultLevel, e.SetLevel(""), "Wrong default 'level' value!")
	a.EqualValues(DefaultFrameHeight, e.SetHeight(""), "Wrong default 'height' value!")
	a.EqualValues(DefaultFrameWidth, e.SetWidth(""), "Wrong default 'width' value!")
	a.EqualValues(DefaultEncodeCRF, e.SetCRF(""), "Wrong default 'crf' value!")
	a.EqualValues(DefaultCaptureTime, e.SetTime(""), "Wrong default 'time' value!")
	a.EqualValues(DefaultFrameRate, e.SetRate(""), "Wrong default 'rate' value!")
	a.EqualValues(DefaultCaptureSize, e.SetSize(""), "Wrong default 'size' value!")
	// Text max values.
	a.EqualValues(MaximumFrameSize, e.SetHeight("9000"), "Wrong maximum 'height' value!")
	a.EqualValues(MaximumFrameSize, e.SetWidth("9000"), "Wrong maximum 'width' value!")
	a.EqualValues(MaximumEncodeCRF, e.SetCRF("9000"), "Wrong maximum 'crf' value!")
	a.EqualValues(MaximumCaptureTime, e.SetTime("9000"), "Wrong maximum 'time' value!")
	a.EqualValues(MaximumFrameRate, e.SetRate("9000"), "Wrong maximum 'rate' value!")
	a.EqualValues(MaximumCaptureSize, e.SetSize("999999999"), "Wrong maximum 'size' value!")
	// Text min values.
	a.EqualValues(MinimumFrameSize, e.SetHeight("1"), "Wrong minimum 'height' value!")
	a.EqualValues(MinimumFrameSize, e.SetWidth("1"), "Wrong minimum 'width' value!")
	a.EqualValues(MinimumEncodeCRF, e.SetCRF("1"), "Wrong minimum 'CRF' value!")
	a.EqualValues(MinimumFrameRate, e.SetRate("-1"), "Wrong minimum 'rate' value!")
}

func TestSaveVideo(t *testing.T) {
	a := assert.New(t)
	e := Get(&Config{FFMPEG: "/bin/echo"})
	fileTemp := "/tmp/go-securityspy-encode-test-12345.txt"

	cmd, out, err := e.SaveVideo("INPUT", fileTemp, "TITLE")
	a.Nil(err, "/bin/echo returned an error. Something may be wrong with your environment.")

	// Make sure the produced command has all the expected values.
	a.Contains(cmd, "-an", "Audio may not be correctly disabled.")
	a.Contains(cmd, "-rtsp_transport tcp -i INPUT", "INPUT value appears to be missing, or rtsp transport is out of order")
	a.Contains(cmd, "-metadata title=\"TITLE\"", "TITLE value appears to be missing.")
	a.Contains(cmd, fmt.Sprintf("-vcodec libx264 -profile:v %v -level %v", DefaultProfile, DefaultLevel),
		"Level or Profile are missing or out of order.")
	a.Contains(cmd, fmt.Sprintf("-crf %d", DefaultEncodeCRF), "CRF value is missing or malformed.")
	a.Contains(cmd, fmt.Sprintf("-t %d", DefaultCaptureTime), "Capture Time value is missing or malformed.")
	a.Contains(cmd, fmt.Sprintf("-s %dx%d", DefaultFrameWidth, DefaultFrameHeight), "Framesize is missing or malformed.")
	a.Contains(cmd, fmt.Sprintf("-r %d", DefaultFrameRate), "Frame Rate value is missing or malformed.")
	a.Contains(cmd, fmt.Sprintf("-fs %d", DefaultCaptureSize), "Size value is missing or malformed.")
	a.True(strings.HasPrefix(cmd, "/bin/echo"), "The command does not - but should - begin with the Encoder value.")
	a.True(strings.HasSuffix(cmd, fileTemp),
		"The command does not - but should - end with a dash to indicate output to stdout.")
	a.EqualValues(cmd, "/bin/echo "+out, "Somehow the wrong value was written")

	// Make sure audio can be turned on.
	e = Get(&Config{FFMPEG: "/bin/echo", Audio: true})
	cmd, _, err = e.GetVideo("INPUT", "TITLE")

	a.Nil(err, "/bin/echo returned an error. Something may be wrong with your environment.")
	a.Contains(cmd, "-c:a copy", "Audio may not be correctly enabled.")
}

func TestValues(t *testing.T) {
	a := assert.New(t)
	e := Get(&Config{})
	c := e.Config()

	a.EqualValues(DefaultFFmpegPath, c.FFMPEG)
	a.EqualValues(DefaultFrameRate, c.Rate)
	a.EqualValues(DefaultFrameHeight, c.Height)
	a.EqualValues(DefaultFrameWidth, c.Width)
	a.EqualValues(DefaultEncodeCRF, c.CRF)
	a.EqualValues(DefaultCaptureTime, c.Time)
	a.EqualValues(DefaultCaptureSize, c.Size)
	a.EqualValues(DefaultProfile, c.Prof)
	a.EqualValues(DefaultLevel, c.Level)
}

/* GoDoc Code Examples */

// Example non-transcode direct-save from securityspy.
func Example_securitySpy() {
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
func Example_dahua() {
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
