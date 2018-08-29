# Goli后端：视频编码模块

本模块用于用户视频编码

**随着其他项目及模块的开发，本程序的功能及 API 规范可能会有较大变化**

### 运行

#### 运行要求

ffmpeg ffprobe 已经安装并在 PATH 中

```bash
go get -u github.com/GoliGoliTV/goliencoder
goliencoder -c /path/to/config.json
```

### 配置文件

配置文件采用 Json 格式，下面是一个示例
```json
{
	"listen": "127.0.0.1:1926",
	"callback": "http://127.0.0.1:1708/",
	"work_dir": "/tmp/goliencoder",
	"concurrent": 1,
	"default_mode": {
		"file_ext": ".mp4",
		"ffargs": [
			"-c:v", "hevc",
			"-crf", "20",
			"-preset", "veryslow",
			"-profile:v", "high",
			"-level", "4.2",
			"-c:a", "aac"
		]
	},
	"modes": [
		{
			"resolution": "640x360",
			"file_ext": ".mp4",
			"ffargs": [
				"-c:v", "h264",
				"-crf", "20",
				"-preset", "veryslow",
				"-profile:v", "high",
				"-level", "4.2",
				"-c:a", "aac"
			]
		},
		{
			"resolution": "854x480",
			"file_ext": ".mp4",
			"ffargs": [
				"-c:v", "h264",
				"-crf", "22",
				"-preset", "veryslow",
				"-profile:v", "high",
				"-level", "4.2",
				"-c:a", "aac"
			]
		},
		{
			"resolution": "1280x720",
			"file_ext": ".mp4",
			"ffargs": [
				"-c:v", "h264",
				"-crf", "23",
				"-preset", "slow",
				"-profile:v", "high",
				"-level", "4.2",
				"-c:a", "aac"
			]
		}
	],
	"min_res": "128x96",
	"asr_max": 3.0,
	"asr_min": 0.5
}
```

* `listen` 监听地址（向此地址发送 HTTP 请求以使模块工作，这个地址不应该能被外部访问）
* `callback` 任务结束后，模块将向该地址发送一个 HTTP 请求，包含任务的完成状态及错误码，支持 HTTPS
* `work_dir` 工作目录，程序将在该目录下寻找和生成文件
* `concurrent` 同时执行的 ffmpeg 进程数量，由于 ffmpeg 自身支持多线程，此值推荐为 1
* `default_mode` 默认的编码模式，配置参考 `modes` 配置，省略字段 `resolution`，此为视频分辨率不满足 `modes` 中的任一但满足转码条件时执行的转码模式
* `modes` 编码方案，为一个数组
	* `resolution` 需要编码的分辨率，此分辨率为分辨率限制，当目标视频宽或者高超出此配置规定时将会激活该配置转码方案，实际分辨率会根据视频分辨率调整，但最终的结果分辨率宽高不会超出此限制
	* `file_ext` 保存的视频文件后缀名，注意加 `.` ；在参数为指定时，ffmpeg 会根据该值判断输出文件的封装形式
	* `ffargs` 该模式下的 ffmpeg 参数；视频分辨率参数（`-s:v`）会自动计算并添加
* `min_res` 最小分辨率限制，当视频宽度或高度不及该值时，会拒绝对该视频转码
* `asr_max` 最大宽高比，当视频 宽度/高度 大于此值时，会拒绝对该视频的转码
* `asr_min` 最小宽高比，当视频 宽度/高度 小于此值时，会拒绝对该视频的转码

### API

以HTTP方式向监听地址发送数据，路径 `/`，方式 `POST`
```json
{
	"video": "2018/031922/MDIAO67EOP.mp4",
	"noconvert": false
}
```
* `video` 视频文件路径（相对于设置中的 `work_dir` 路径），程序将在工作目录找到该文件并尝试转码。
* `noconvert` 仅返回信息，不进行视频转码。默认 `false`。

响应任务请求状态及文件详情：
```json
{
	"ok": true,
	"result": {
		"streams_num": 2,
		"file_format": "mov,mp4,m4a,3gp,3g2,mj2; QuickTime / MOV",
		"duration": 13958,
		"bit_rate": 404636,
		"multivideo": false,
		"multiaudio": false,
		"videos": [
			{
				"index": 0,
				"codec": "h264",
				"codec_name":"H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10",
				"width": 1920,
				"height": 1080,
				"framerate": 26
			}
		],
		"audios": [
			{
				"index": 1,
				"codec": "aac",
				"codec_name": "AAC (Advanced Audio Coding)",
				"channels": 2,
				"samplerate": 44100
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
		"streams_num": 1,
		"file_format": "mov,mp4,m4a,3gp,3g2,mj2; QuickTime / MOV",
		"duration": 5810,
		"bit_rate": 40620,
		"multivideo": false,
		"multiaudio": false,
		"videos": [
			{
				"index": 0,
				"codec": "h264",
				"codec_name":"H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10",
				"width": 100,
				"height": 80,
				"framerate": 26
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

一个视频，每个分辨率方案对应一个编码任务

例如：当前配置文件中 `resolutions` 中包含 `640x360`、`854x480` 两个值

1. 要转码的文件分辨率为 `1280x720`，则会生成 2 个任务，分别编码 `640x360` 和 `854x480` 两种分辨率
1. 要转码的文件分辨率为 `700x394`，则只会生成 1 个任务，仅编码 `640x360` 的分辨率
1. 要转码的文件分辨率为 `520x292`，仍然会生成 1 个任务，编码 `520x292` 的分辨率

每个编码任务完成后，会向配置文件中 `callback` 发送一个HTTP请求，格式为 Json，内容如下：
```json
{
	"status": "succeed",
	"origin_file": "2018/031922/MDIAO67EOP.mp4",
	"output_file": "2018/031922/MDIAO67EOP_720p.mp4",
	"resolution": "1280x720"
}
```

* `resolution` 目标分辨率，并不是转码后视频实际的分辨率，与配置文件中 `modes[*].resolution` 相对应

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

1. 当生成 `resolution` 存在于配置文件中时：原文件名去掉后缀名 + `_` + 编码 `resolution` 的 `height` + `p` + 自定义后缀
1. 当生成 `resolution` 不存在配置文件中时（通常见于分辨率低于配置转码分辨率中的最低版本）：原文件名去掉后缀名 + `_default` + 自定义后缀
