package main

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
)

type probeStream struct {
	Index              uint8  `json:"index"`
	Codec              string `json:"codec_name"`
	CodecFullName      string `json:"codec_long_name,omitempty"`
	Type               string `json:"codec_type"`
	Width              uint   `json:"width,omitempty"`
	Height             uint   `json:"height,omitempty"`
	RawSampleRate      string `json:"sample_rate,omitempty"`
	Channels           uint   `json:"channels,omitempty"`
	RawFrameRateString string `json:"r_frame_rate,omitempty"`
	FrameRate          uint   `json:"-"`
	SampleRate         uint   `json:"-"`
}

func (f *probeStream) RefreshFR() (err error) {
	if f.Type == "video" {
		if f.RawFrameRateString == "" {
			return
		}
		pf := strings.Split(f.RawFrameRateString, "/")
		if len(pf) != 2 {
			return
		}
		var pa, pb uint64
		pa, err = strconv.ParseUint(pf[0], 10, 32)
		if err != nil {
			return
		}
		pb, err = strconv.ParseUint(pf[1], 10, 32)
		if err != nil {
			return
		}
		if pb == 0 {
			return
		}
		f.FrameRate = uint(pa / pb)
	} else if f.Type == "audio" {
		if f.RawSampleRate == "" {
			return
		}
		var ra uint64
		ra, err = strconv.ParseUint(f.RawSampleRate, 10, 32)
		if err != nil {
			return
		}
		f.SampleRate = uint(ra)
	}
	return
}

type probeFormat struct {
	StreamNum         uint   `json:"nb_streams"`
	FormatName        string `json:"format_name"`
	FormatFullName    string `json:"format_long_name"`
	RawDurationString string `json:"duration"`
	RawBitrateString  string `json:"bit_rate"`
	Duration          uint   `json:"-"`
	Bitrate           uint   `json:"-"`
}

func (p *probeFormat) Format() (err error) {
	d, err := strconv.ParseFloat(p.RawDurationString, 32)
	if err != nil {
		return
	}
	p.Duration = uint(d * 1000.0)
	b, err := strconv.ParseUint(p.RawBitrateString, 10, 32)
	if err != nil {
		return
	}
	p.Bitrate = uint(b)
	return
}

type probeResult struct {
	Streams []probeStream `json:"streams"`
	Format  probeFormat   `json:"format"`
}

func (p *probeResult) Init() (err error) {
	sl := len(p.Streams)
	for i := 0; i < sl; i++ {
		err = p.Streams[i].RefreshFR()
		if err != nil {
			return
		}
	}
	err = p.Format.Format()
	return
}

func parseProbeResult(b *[]byte) (pr probeResult, err error) {
	err = json.Unmarshal(*b, &pr)
	if err != nil {
		return
	}
	err = pr.Init()
	return
}

func probeVideo2(videoPath string) (vi videoInfo2, err error) {
	cmd := exec.Command("ffprobe", "-hide_banner", "-loglevel", "-8",
		"-print_format", "json", "-show_format", "-show_streams", "-i", videoPath)
	probeBuffer, err := cmd.Output()
	if err != nil {
		return
	}
	pres, err := parseProbeResult(&probeBuffer)
	if err != nil {
		return
	}
	vi, err = probeResult2VideoInfo(pres)
	return
}
