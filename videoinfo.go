package main

import (
	"fmt"
)

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
