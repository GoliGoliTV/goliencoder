package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

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
