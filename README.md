# Goli后端：视频编码模块

本模块用于用户视频编码

**本程序尚未完成开发**

### 运行

#### 运行要求

ffmpeg 已经安装并在 PATH 中

```bash
/path/to/bin/goliencoder -c /path/to/config.json
```

### 配置文件

配置文件采用 Json 格式，下面是一个示例
```json
{
	"listen": "127.0.0.1:1926",
	"callback": "http://127.0.0.1:1708/",
	"work_dir": "/tmp/goliencoder",
	"concurrent": 1,
	"vcodec": "libx264",
	"acodec": "aac",
	"vcrf": 20,
	"preset": "slow",
	"vprofile": "high",
	"resolutions": ["640x360", "854x480", "1280x720"],
	"min_res": "128x96",
	"asr_max": 3.0,
	"asr_min": 0.5
}
```

* `listen` 监听地址（向此地址发送 HTTP 请求以使模块工作，这个地址不应该能被外部访问）
* `callback` 任务结束后，模块将向该地址发送一个 HTTP 请求，包含任务的完成状态及错误码，支持 HTTPS。
* `work_dir` 工作目录，程序将在该目录下寻找和生成文件
* `concurrent` 同时执行的 ffmpeg 进程数量，ffmpeg 自身支持多线程，此值推荐为 1
* `vcodec` 视频编码方案，详细请参考 ffmpeg 文档
* `acodec` 音频编码方案，详细请参考 ffmpeg 文档
* `vcrf` 编码 crf 参数，该值越小视频质量越高
* `preset` 编码方案 preset，详细请参考 ffmpeg 文档
* `vprofile` AVC/HEVC 编码 Profile
* `resolutions` 需要编码的分辨率，此分辨率为**分辨率限制**，实际分辨率会根据视频分辨率调整，但最终的结果分辨率宽高不会超出此限制
* `min_res` 最小分辨率限制，当视频宽度或高度不及该值时，会拒绝对该视频转码
* `asr_max` 最大宽高比，当视频 宽度/高度 大于此值时，会拒绝对该视频的转码
* `asr_min` 最小宽高比，当视频 宽度/高度 小于此值时，会拒绝对该视频的转码

### API

以HTTP方式向监听地址发送数据，路径 `/`，方式 `POST`

发送请求，请求参数 `video`，程序将在工作目录找到该文件并尝试转码
```json
{
	"video": "2018/031922/MDIAO67EOP.mp4"
}
```

响应任务请求状态及文件详情：
```json
{
	"ok": true,
	"result": {
		"duration": "00:13:27.19",
		"bitrate": "2876 kb/s",
		"resolution": "1920x1080",
		"streams": [
			{
				"channel": "#0:0",
				"type": "Audio",
				"codec": "aac"
			},
			{
				"channel": "#0:1",
				"type": "Video",
				"codec": "h264"
			}
		]
	}
}
```
错误示例：
```json
{
	"ok": false,
	"error": "video resolution is too low",
	"result": {
		"duration": "00:05:12.01",
		"bitrate": "186 kb/s",
		"resolution": "120x100",
		"streams": [
			{
				"channel": "#0:0",
				"type": "Audio",
				"codec": "aac"
			},
			{
				"channel": "#0:1",
				"type": "Video",
				"codec": "h264"
			}
		]
	}
}
```

错误示例2：
```json
{
	"ok": false,
	"error": "can not stat file: stat /tmp/goliencoder/xyz: no such file or directory",
	"result": {}
}
```

### 编码任务

一个视频，每个分辨率方案对应一个编码任务。

例如：当前配置文件中 `resolutions` 中包含 `640x360`、`854x480` 两个值。

1. 要转码的文件分辨率为 `1280x720`，则会生成 2 个任务，分别编码 `640x360` 和 `854x480` 两种分辨率。
1. 要转码的文件分辨率为 `700x394`，则只会生成 1 个任务，仅编码 `640x360` 的分辨率。
1. 要转码的文件分辨率为 `520x292`，仍然会生成 1 个任务，编码 `520x292` 的分辨率。

每个编码任务完成后，会向配置文件中 `callback` 发送一个HTTP请求，格式为 Json，内容如下：
```json
{
	"status": "succeed",
	"origin_file": "2018/031922/MDIAO67EOP.mp4",
	"output_file": "2018/031922/MDIAO67EOP_720p.mp4",
	"resolution": "1280x720"
}
```

* `resolution` 目标分辨率，并不是转码后视频实际的分辨率

失败例：
```json
{
	"status": "failed",
	"error": "exit exit status 1",
	"origin_file": "2018/031922/MDIAO67EOP.mp4",
	"output_file": "2018/031922/MDIAO67EOP_720p.mp4",
	"resolution": "1280x720"
}
```

#### 转码任务生成文件名规律

1. 当生成 `resolution` 存在于配置文件中时：原文件名去掉后缀名 + `_` + 编码 `resolution` 的 `height` + `p.mp4`
1. 当生成 `resolution` 不存在配置文件中时（通常见于分辨率低于配置转码分辨率中的最低版本）：原文件名去掉后缀名 + `_orgi.mp4`