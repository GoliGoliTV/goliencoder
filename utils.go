package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type probeStream struct {
	Index              uint8  `json:"index"`
	Codec              string `json:"codec_name"`
	CodecFullName      string `json:"codec_full_name,omitempty"`
	Type               string `json:"codec_type"`
	Width              uint   `json:"width,omitempty"`
	Height             uint   `json:"height,omitempty"`
	SampleRate         string `json:"sample_rate,omitempty"`
	Channels           uint   `json:"channels,omitempty"`
	FrameRate          uint   `json:"-"`
	RawFrameRateString string `json:"r_frame_rate,omitempty"`
}

func (f *probeStream) RefreshFR() (err error) {
	if f.Type != "video" {
		return
	}
	if f.RawFrameRateString == "" {
		return
	}
	pf := strings.Split(f.RawFrameRateString, "/")
	if len(pf) != 2 {
		return
	}
	pa, err := strconv.ParseUint(pf[0], 10, 32)
	if err != nil {
		return
	}
	pb, err := strconv.ParseUint(pf[1], 10, 32)
	if err != nil {
		return
	}
	if pb == 0 {
		return
	}
	f.FrameRate = uint(pa / pb)
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
	for _, s := range p.Streams {
		err = s.RefreshFR()
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

type videoInfoStream struct {
	Index      uint8  `json:"index"`
	Codec      string `josn:"codec"`
	CodecName  string `json:"codec_name,omitempty"`
	Width      uint   `json:"width,omitempty"`
	Height     uint   `json:"height,omitempty"`
	SampleRate string `json:"sample_rate,omitempty"`
	Channels   uint   `json:"channels,omitempty"`
	FrameRate  uint   `json:"framerate,omitempty"`
}

type videoInfo2 struct {
	StreamNum  uint              `json:"streams_num"`
	FormatName string            `json:"file_format"`
	Duration   uint              `json:"duration"`
	Bitrate    uint              `json:"bit_rate"`
	MultiVideo bool              `json:"multivideo"`
	MultiAudio bool              `json:"multiaudio"`
	Videos     []videoInfoStream `json:"videos"`
	Audios     []videoInfoStream `json:"audios"`
}

func probeResult2VideoInfo(pr probeResult) (vi videoInfo2, err error) {
	var vsa []videoInfoStream
	var asa []videoInfoStream
	for _, s := range pr.Streams {
		if s.Type == "audio" {
			asa = append(asa, videoInfoStream{
				Index:      s.Index,
				Codec:      s.Codec,
				CodecName:  s.CodecFullName,
				SampleRate: s.SampleRate,
				Channels:   s.Channels,
			})
		} else if s.Type == "video" {
			vsa = append(vsa, videoInfoStream{
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
