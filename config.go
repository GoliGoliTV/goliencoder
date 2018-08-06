package main

import (
	"encoding/json"
	"io/ioutil"
)

type encodeMode struct {
	Resolution string   `json:"resolution"`
	FFMpegArgs []string `json:"ffargs"`
}

type config struct {
	ListenAddress    string       `json:"listen"`
	CallbackURL      string       `json:"callback"`
	WorkingDirectory string       `json:"work_dir"`
	Cuncurrent       int          `json:"concurrent"`
	FFArgs           []string     `json:"defaultargs"`
	Modes            []encodeMode `json:"modes"`
	MinResolution    string       `json:"min_res"`
	MaxAspectRatio   float32      `json:"asr_max"`
	MinAspectRatio   float32      `json:"asr_min"`
}

func loadConfigJSON(file string) (c config, err error) {
	jb, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = json.Unmarshal(jb, &c)
	if err != nil {
		return
	}
	return
}
