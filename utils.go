package main

import (
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func probeVideo(videoPath string) (vi VideoInfo, err error) {
	cmd := exec.Command("ffprobe", "-i", videoPath)
	reg, _ := regexp.Compile(`^Stream (#\d+:\d+).*: (?:(Audio): (\w+).+?|(Video): (\w+).+?, (\d+x\d+)[, \[])`)
	probeBuffer, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	sl := strings.Split(string(probeBuffer), "\n")
	for _, s := range sl {
		data := strings.TrimSpace(s)
		var da []string
		if strings.HasPrefix(data, "Duration:") {
			da = strings.Split(data, ", ")
			if len(da) != 3 {
				continue
			}
			vi.Duration = da[0][10:]
			vi.Bitrate = da[2][9:]
		} else if strings.HasPrefix(data, "Stream #") {
			mresult := reg.FindStringSubmatch(data)
			if len(mresult) == 0 {
				continue
			}
			if mresult[2] == "Audio" {
				vi.Streams = append(vi.Streams, StreamInFile{mresult[1], mresult[2], mresult[3]})
			} else if mresult[4] == "Video" {
				vi.Resolution = mresult[6]
				vi.Streams = append(vi.Streams, StreamInFile{mresult[1], mresult[4], mresult[5]})
			}
		}
	}
	return
}

func calculateResolution(ow, oh, tw, th int) (rw, rh int) {
	if ow <= tw && oh <= th {
		rw, rh = ow, oh
		return
	}
	if ow > tw {
		oh = oh * tw / ow
		if oh%2 != 0 {
			oh = oh + 1
		}
		ow = tw
	}
	if oh > th {
		ow = ow * th / oh
		if ow%2 != 0 {
			ow = ow + 1
		}
		oh = th
	}
	tar := float64(tw) / float64(th)
	oar := float64(ow) / float64(oh)
	ras := math.Abs(tar - oar)
	if ras < 0.1 {
		rw, rh = tw, th
		return
	}
	rw, rh = ow, oh
	return
}

func parseResolution(r string) (w, h int, err error) {
	sa := strings.Split(r, "x")
	wi, err := strconv.ParseInt(sa[0], 10, 32)
	if err != nil {
		return
	}
	he, err := strconv.ParseInt(sa[1], 10, 32)
	if err != nil {
		return
	}
	w, h = int(wi), int(he)
	return
}

func ffargs(input, output string, rw, rh int, args []string) (fullargs []string) {
	fullargs = []string{"-i", input, "-y", "-s:v", fmt.Sprintf("%dx%d", rw, rh)}
	fullargs = append(fullargs, args...)
	fullargs = append(fullargs, output)
	return
}
