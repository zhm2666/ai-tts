package ffmpeg

import "runtime"

var FFmpeg = "pkg/ffmpeg/windows/bin/ffmpeg.exe"
var FFprobe = "pkg/ffmpeg/windows/bin/ffprobe.exe"

func init() {
	if runtime.GOOS != "windows" {
		FFmpeg = "pkg/ffmpeg/linux/ffmpeg"
		FFprobe = "pkg/ffmpeg/linux/ffprobe"
	}
}
