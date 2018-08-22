package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
)

type apiResponse struct {
	Ok        bool       `json:"ok"`
	ErrorInfo string     `json:"error,omitempty"`
	Result    videoInfo2 `json:"result,omitempty"`
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
	Args       []string
	OutputFile string
	Resolution string
}

func generateTasks(width, height uint, modes []encodeMode, inFile string, dm encodeMode) (ts []encodeTask) {
	for _, m := range modes {
		tw, th, e := parseResolution(m.Resolution)
		if e != nil {
			continue
		}
		if width > tw || height > th {
			rw, rh := calculateResolution(width, height, tw, th)
			outFile := inFile[:len(inFile)-len(path.Ext(inFile))] + fmt.Sprintf("_%dp", th) + dm.FileExtentionName
			ts = append(ts, encodeTask{inFile, ffargs(inFile, outFile, rw, rh, m.FFMpegArgs), outFile, m.Resolution})
		}
	}
	if len(ts) == 0 {
		outFile := inFile[:len(inFile)-len(path.Ext(inFile))] + "_default" + dm.FileExtentionName
		ts = append(ts, encodeTask{inFile, ffargs(inFile, outFile, width, height, dm.FFMpegArgs), outFile, "default"})
	}
	return
}

func apiErrorResponse(info string) (b []byte) {
	b, _ = json.Marshal(apiResponse{false, info, videoInfo2{}})
	return
}

func main() {
	var configPath = flag.String("c", "config.json", "config file path")
	var printHelp = flag.Bool("h", false, "print this help")
	flag.Parse()
	if *printHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}
	cfg, err := loadConfigJSON(*configPath)
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
				cmd := exec.Command("ffmpeg", task.Args...)
				cmd.Dir = cfg.WorkingDirectory
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
				} else {
					fmt.Println("cannot send request to callback address:", err)
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
		err = json.Unmarshal(requestBody, &req)
		if err != nil {
			w.WriteHeader(400)
			w.Write(apiErrorResponse("cannot parse your request"))
			return
		}
		_, err = os.Stat(path.Join(cfg.WorkingDirectory, req.Video))
		if err != nil {
			w.Write(apiErrorResponse("can not stat file: " + err.Error()))
			return
		}
		vi, err := probeVideo2(path.Join(cfg.WorkingDirectory, req.Video))
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
		videoWidth, videoHeight := vi.Videos[0].Width, vi.Videos[0].Height
		if err != nil {
			res.Ok = false
			res.ErrorInfo = "can not parse video resolution, maybe not a video file"
		}
		resBuffer, _ := json.Marshal(res)
		w.Write(resBuffer)
		if res.Ok {
			go func(w, h uint, v string) {
				for _, t := range generateTasks(w, h, cfg.Modes, v, cfg.DefaultMode) {
					tasks <- t
				}
			}(videoWidth, videoHeight, req.Video)
		}
		return
	})
	err = http.ListenAndServe(cfg.ListenAddress, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(4)
	}
}
