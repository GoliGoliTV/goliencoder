package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type config struct {
	ListenAddress    string   `json:"listen"`
	CallbackURL      string   `json:"callback"`
	WorkingDirectory string   `json:"work_dir"`
	Cuncurrent       int      `json:"concurrent"`
	VideoCodec       string   `json:"vcodec"`
	VideoCRF         string   `json:"vcrf"`
	CodecPreset      string   `json:"preset"`
	VProfile         string   `json:"vprofile"`
	AudioCodec       string   `json:"acodec"`
	Resolutions      []string `json:"resolutions"`
	MinResolution    string   `json:"min_res"`
	MaxAspectRatio   float32  `json:"asr_max"`
	MinAspectRatio   float32  `json:"asr_min"`
	EnableLog        bool     `json:"-"`
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

type StreamInFile struct {
	Channel string `json:"channel"`
	Type    string `json:"type"`
	Codec   string `json:"codec"`
}

type VideoInfo struct {
	Duration   string         `json:"duration"`
	Bitrate    string         `json:"bitrate"`
	Resolution string         `json:"resolution,omitempty"`
	Streams    []StreamInFile `json:"streams"`
}

func (v *VideoInfo) GetAspectRatio() (ar float32, w int, h int) {
	w, h, e := parseResolution(v.Resolution)
	if e != nil {
		return
	}
	if w < 1 || h < 1 {
		w, h = 0, 0
		return
	}
	ar = float32(w) / float32(h)
	return
}

func (v *VideoInfo) CheckAspectRatio(amin, amax float32, wmin, hmin int) int {
	vr, w, h := v.GetAspectRatio()
	if w < wmin || h < hmin {
		return 1
	}
	if vr < amin || vr > amax {
		return 2
	}
	return 0
}

type apiResponse struct {
	Ok        bool      `json:"ok"`
	ErrorInfo string    `json:"error,omitempty"`
	Result    VideoInfo `json:"result,omitempty"`
}

type apiRequest struct {
	Video string `json:"video"`
}

type callbackRequest struct {
	TaskStatus string `json:"status"`
	ErrorInfo  string `json:"error,omitempty"`
	OriginFile string `json:"origin_file"`
	OutputFile string `json:"output_file"`
	Resolution string `json:"resolution"`
}

type encodeTask struct {
	InputFile  string
	Resolution string
	OutputFile string
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

func generateTasks(width, height int, resolutions []string, inFile string) (ts []encodeTask) {
	for _, r := range resolutions {
		tw, th, e := parseResolution(r)
		if e != nil {
			continue
		}
		if width > tw || height > th {
			rw, rh := calculateResolution(width, height, tw, th)
			outFile := inFile[:len(inFile)-len(path.Ext(inFile))] + fmt.Sprintf("_%dp.mp4", th)
			ts = append(ts, encodeTask{inFile, fmt.Sprintf("%dx%d", rw, rh), outFile})
		}
	}
	if len(ts) == 0 {
		outFile := inFile[:len(inFile)-len(path.Ext(inFile))] + "_orgi.mp4"
		ts = append(ts, encodeTask{inFile, fmt.Sprintf("%dx%d", width, height), outFile})
	}
	return
}

func apiErrorResponse(info string) (b []byte) {
	b, _ = json.Marshal(apiResponse{false, info, VideoInfo{}})
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

func main() {
	var configPath = flag.String("c", "config.json", "config file path")
	var printHelp = flag.Bool("h", false, "print this help")
	var cfg config
	flag.Parse()
	if *printHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}
	configBuffer, err := ioutil.ReadFile(*configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = json.Unmarshal(configBuffer, &cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	minWidth, minHeight, _ := parseResolution(cfg.MinResolution)
	concurrentCounter := make(chan bool, cfg.Cuncurrent)
	tasks := make(chan encodeTask)
	go func() {
		for {
			concurrentCounter <- true
			task := <-tasks
			go func(task encodeTask) {
				var tr callbackRequest
				cmd := exec.Command("ffmpeg",
					"-i", task.InputFile,
					"-c:v", cfg.VideoCodec,
					"-crf", cfg.VideoCRF,
					"-preset", cfg.CodecPreset,
					"-profile:v", cfg.VProfile,
					"-c:a", cfg.AudioCodec,
					"-s:v", task.Resolution,
					task.OutputFile)
				err := cmd.Run()
				tr.OriginFile = task.InputFile
				tr.OutputFile = task.OutputFile
				tr.Resolution = task.Resolution
				if err != nil {
					tr.TaskStatus = "failed"
					tr.ErrorInfo = err.Error()
				} else {
					tr.TaskStatus = "succeed"
				}
				b, _ := json.Marshal(tr)
				resp, err := http.Post(cfg.CallbackURL, "application/json", bytes.NewReader(b))
				if err == nil {
					ioutil.ReadAll(resp.Body)
					resp.Body.Close()
				}
				<-concurrentCounter
			}(task)
		}
	}()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			w.Write(apiErrorResponse("cannot read request body"))
			return
		}
		var req apiRequest
		var res apiResponse
		err = json.Unmarshal(requestBody, req)
		if err != nil {
			w.WriteHeader(400)
			w.Write(apiErrorResponse("cannot parse your request"))
			return
		}
		vi, err := probeVideo(path.Join(cfg.WorkingDirectory, req.Video))
		if err != nil {
			w.Write(apiErrorResponse("err: " + err.Error()))
			return
		}
		res.Result = vi
		videoCheck := vi.CheckAspectRatio(cfg.MinAspectRatio, cfg.MaxAspectRatio, minWidth, minHeight)
		if videoCheck == 1 {
			res.ErrorInfo = "video resolution is too low"
		} else if videoCheck == 2 {
			res.ErrorInfo = "this aspect ratio is not allow"
		} else {
			res.Ok = true
		}
		resBuffer, _ := json.Marshal(res)
		w.Write(resBuffer)
		if res.Ok {

		}
		return
	})
}
