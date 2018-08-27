package main

import (
	"encoding/json"
	"fmt"
	"math"
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

type channelInfoStream struct {
	Index      uint8  `json:"index"`
	Codec      string `json:"codec"`
	CodecName  string `json:"codec_name,omitempty"`
	Width      uint   `json:"width,omitempty"`
	Height     uint   `json:"height,omitempty"`
	SampleRate uint   `json:"samplerate,omitempty"`
	Channels   uint   `json:"channels,omitempty"`
	FrameRate  uint   `json:"framerate,omitempty"`
}

type videoInfo2 struct {
	StreamNum  uint                `json:"streams_num,omitempty"`
	FormatName string              `json:"file_format,omitempty"`
	Duration   uint                `json:"duration,omitempty"`
	Bitrate    uint                `json:"bit_rate,omitempty"`
	MultiVideo bool                `json:"multivideo,omitempty"`
	MultiAudio bool                `json:"multiaudio,omitempty"`
	Videos     []channelInfoStream `json:"videos,omitempty"`
	Audios     []channelInfoStream `json:"audios,omitempty"`
}

func (v *videoInfo2) GetAspectRatio() (ar float32, w, h uint) {
	w = v.Videos[0].Width
	h = v.Videos[0].Height
	ar = float32(w) / float32(h)
	return
}

func (v *videoInfo2) CheckAspectRatio(amin, amax float32, wmin, hmin uint) int {
	vr, w, h := v.GetAspectRatio()
	if w < wmin || h < hmin {
		return 1
	}
	if vr < amin || vr > amax {
		return 2
	}
	return 0
}

func probeResult2VideoInfo(pr probeResult) (vi videoInfo2, err error) {
	var vsa []channelInfoStream
	var asa []channelInfoStream
	for _, s := range pr.Streams {
		if s.Type == "audio" {
			asa = append(asa, channelInfoStream{
				Index:      s.Index,
				Codec:      s.Codec,
				CodecName:  s.CodecFullName,
				SampleRate: s.SampleRate,
				Channels:   s.Channels,
			})
		} else if s.Type == "video" {
			vsa = append(vsa, channelInfoStream{
				Index:     s.Index,
				Codec:     s.Codec,
				CodecName: s.CodecFullName,
				Width:     s.Width,
				Height:    s.Height,
				FrameRate: s.FrameRate,
			})
		}
	}
	if len(vsa) == 0 {
		err = fmt.Errorf("no video stream found in the file")
		return
	} else if len(vsa) > 1 {
		vi.MultiVideo = true
	}
	if len(asa) > 1 {
		vi.MultiAudio = true
	}
	vi.Videos, vi.Audios = vsa, asa
	vi.StreamNum = uint(len(vsa) + len(asa))
	vi.FormatName = pr.Format.FormatName + "; " + pr.Format.FormatFullName
	vi.Duration = pr.Format.Duration
	vi.Bitrate = pr.Format.Bitrate
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

func calculateResolution(ow, oh, tw, th uint) (rw, rh uint) {
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

func parseResolution(r string) (w, h uint, err error) {
	sa := strings.Split(r, "x")
	wi, err := strconv.ParseInt(sa[0], 10, 32)
	if err != nil {
		return
	}
	he, err := strconv.ParseInt(sa[1], 10, 32)
	if err != nil {
		return
	}
	w, h = uint(wi), uint(he)
	return
}

func ffargs(input, output string, rw, rh uint, args []string) (fullargs []string) {
	fullargs = []string{"-i", input, "-y", "-s:v", fmt.Sprintf("%dx%d", rw, rh)}
	fullargs = append(fullargs, args...)
	fullargs = append(fullargs, output)
	return
}
