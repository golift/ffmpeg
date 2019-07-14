# Simple Go FFMPEG Library for RTSP streams (IP cameras)

Capture video footage from RTSP IP cameras.

Provides a simple interface to set FFMPEG options and capture video from an RTSP source.

Lots of other libraries out there do ffmpeg and rtsp things, but I couldn't find
any that fit this simple task of "get a video snippet from a camera."

- [GODOC](https://godoc.org/github.com/golift/ffmpeg)

## Example

```golang
package main

import (
	"log"

	"code.golift.io/ffmpeg"
)

func main() {
	/* Example non-transcode direct-save from securityspy. */

	securitypsy := "rtsp://user:pass@127.0.0.1:8000/++stream?cameraNum=1"
	output := "/tmp/securitypsy_captured_file.mov"
	c := &ffmpeg.Config{
		FFMPEG: "/usr/local/bin/ffmpeg",
		Copy:   true, // do not transcode
		Audio:  true, // retain audio stream
		Time:   10,   // 10 seconds
	}
	encode := ffmpeg.Get(c)
	cmd, out, err := encode.SaveVideo(securitypsy, output, "SecuritySpyVideoTitle")
	log.Println("Command Used:", cmd)
	log.Println("Command Output:", out)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Saved file from", securitypsy, "to", output)

	/* Example transcode from a Dahua IP camera. */

	dahua := "rtsp://admin:password@192.168.1.12/live"
	output = "/tmp/dahua_captured_file.m4v"
	f := ffmpeg.Get(&ffmpeg.Config{
		Audio:  true, // retain audio stream
		Time:   10,   // 10 seconds
		Width:  1920,
		Height: 1080,
		CRF:    23,
		Level:  "4.0",
		Rate:   5,
		Prof:   "baseline",
	})
	cmd, out, err = f.SaveVideo(dahua, output, "DahuaVideoTitle")
	log.Println("Command Used:", cmd)
	log.Println("Command Output:", out)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Saved file from", dahua, "to", output)
}


```
