# Simple Go FFMPEG Wrapper for Camera Streams

Capture short video clips from camera stream URLs (RTSP and HTTP(S)).

Provides a simple interface to set FFMPEG options and save or stream captured video.

Lots of other libraries out there do ffmpeg and rtsp things, but I couldn't find
any that fit this simple task of "get a video snippet from a camera."

- [GODOC](https://pkg.go.dev/golift.io/ffmpeg)

## Notes

- `SaveVideo`/`SaveVideoContext` write to files and return ffmpeg output text.
- `GetVideo`/`GetVideoContext` return an `io.ReadCloser` stream.
- Input URL scheme is respected:
  - RTSP URLs use `-rtsp_transport tcp`.
  - Non-RTSP URLs do not include RTSP-only options.
- Output container differs by destination:
  - file output: `mov`
  - stream output (`"-"`): fragmented MP4 flags for pipe-safe output
- Errors include a tail of ffmpeg stderr when available for better diagnostics.

## Example

```golang
package main

import (
	"log"

	"golift.io/ffmpeg"
)

func main() {

	/*
	 * Example non-transcode direct-save from securityspy.
	 */

	securitypsy := "https://user:pass@127.0.0.1:8001/++video?cameraNum=1"
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

	/*
	 * Example transcode from a Dahua IP camera.
	 */

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
