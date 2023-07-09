package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
)

type DeviceContext struct {
	defaultId   string
	notifTag    string
	statSection string
	icon        string
	rofiPrompt  string
}

type Device struct {
	id   int
	name string
	nick string
}

var reNodeDesc = regexp.MustCompile(`node.description = "(.+)"`)
var reNodeNick = regexp.MustCompile(`node.nick = "(.+)"`)

func getDevice(id int) Device {
	output := runCmd("wpctl", "inspect", fmt.Sprint(id))
	name := reNodeDesc.FindStringSubmatch(output)[1]
	nick := reNodeNick.FindStringSubmatch(output)[1]

	return Device{
		id:   id,
		name: name,
		nick: nick,
	}
}

func getDeviceList(ctx *DeviceContext) ([]Device, int) {
	cmd := exec.Command("wpctl", "status")

	bytes, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	header := ""
	readingSection := false

	// TODO: Fix this abomination
	devices := []Device{}
	defaultDeviceIdx := -1

	for _, ln := range strings.Split(string(bytes[:]), "\n") {
		if header == "" {
			header = ln
		}

		if ln == "" {
			header = ""
		}

		if header == "Audio" {
			if readingSection {
				if strings.HasSuffix(ln, "]") {
					if ln[6] == '*' {
						defaultDeviceIdx = len(devices)
					}

					idStr := ln[10:strings.Index(ln, ".")]
					id, err := strconv.Atoi(idStr)

					if err != nil {
						log.Fatal(err)
					}

					devices = append(devices, getDevice(id))
				} else {
					break
				}
			} else {
				readingSection = strings.HasSuffix(ln, ctx.statSection+":")
			}
		}
	}

	return devices, defaultDeviceIdx
}

func runCmd(name string, arg ...string) string {
	cmd := exec.Command(name, arg...)
	bytes, err := cmd.Output()

	if err != nil {
		log.Fatal(err)
	}

	output := string(bytes[:])
	return trimStr(output)
}

func notify(ctx *DeviceContext) {
	output := runCmd("wpctl", "get-volume", ctx.defaultId)
	devices, defaultDeviceIdx := getDeviceList(ctx)
	defaultDeviceNick := devices[defaultDeviceIdx].nick

	summary := fmt.Sprintf("%s  %s", ctx.icon, defaultDeviceNick)
	body := output

	runCmd("dunstify",
		"-h", "string:x-dunst-stack-tag:"+ctx.notifTag,
		"-t", "1000",
		summary,
		body)
}

func turnVolume(ctx *DeviceContext, delta int) {
	if delta == 0 {
		return
	}

	var sign string
	if delta > 0 {
		sign = "+"
	} else {
		sign = "-"
		delta *= -1
	}

	deltaVol := fmt.Sprintf("%d%%%s", delta, sign)

	runCmd("wpctl", "set-volume", ctx.defaultId, deltaVol, "-l", "1.0")
	notify(ctx)
}

func toggleMute(ctx *DeviceContext) {
	runCmd("wpctl", "set-mute", ctx.defaultId, "toggle")
	notify(ctx)
}

func cycleDevice(ctx *DeviceContext) {
	devices, defaultDeviceIdx := getDeviceList(ctx)
	if len(devices) == 0 {
		return
	}

	newDeviceIdx := (defaultDeviceIdx + 1) % len(devices)
	setDefaultDevice(devices[newDeviceIdx].id)
	notify(ctx)
}

func selectDevice(ctx *DeviceContext) {
	devices, defaultDeviceIdx := getDeviceList(ctx)
	if len(devices) == 0 {
		return
	}

	rofiSelectArg := ""
	if defaultDeviceIdx >= 0 {
		rofiSelectArg = devices[defaultDeviceIdx].nick
	}

	deviceNicks := make([]string, len(devices))
	for i, device := range devices {
		deviceNicks[i] = device.nick
	}

	cmd := exec.Command("rofi", "-dmenu", "-no-custom",
		"-format", "i",
		"-select", rofiSelectArg,
		"-p", ctx.rofiPrompt)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	io.WriteString(stdin, strings.Join(deviceNicks, "\n"))
	stdin.Close()

	bytes, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	output := trimStr(string(bytes[:]))
	selectedDeviceIdx, err := strconv.Atoi(output)
	if err != nil {
		log.Fatal(err)
	}

	setDefaultDevice(devices[selectedDeviceIdx].id)
}

func setDefaultDevice(id int) {
	runCmd("wpctl", "set-default", fmt.Sprint(id))
}

func trimStr(str string) string {
	return strings.Trim(str, " \n\t\r")
}

func main() {
	var ctx *DeviceContext

	app := &cli.App{
		Name:  "wpak",
		Usage: "Utility commands for wpctl, rofi, dunst",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "source",
				Usage: "operate on audio source devices, instead of sinks",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "mute",
				Usage: "toggle mute on default device",
				Action: func(cCtx *cli.Context) error {
					toggleMute(ctx)
					return nil
				},
			},
			{
				Name:  "select",
				Usage: "select default device via rofi",
				Action: func(cCtx *cli.Context) error {
					selectDevice(ctx)
					return nil
				},
			},
			{
				Name:  "cycle",
				Usage: "select next default device",
				Action: func(cCtx *cli.Context) error {
					cycleDevice(ctx)
					return nil
				},
			},
			{
				Name:            "volume",
				Usage:           "turn volume up or down by DELTA%",
				ArgsUsage:       "DELTA",
				SkipFlagParsing: true,
				Action: func(cCtx *cli.Context) error {
					if cCtx.Args().Len() < 1 {
						return errors.New("first arg DELTA required")
					}

					deltaStr := cCtx.Args().First()
					delta, err := strconv.Atoi(deltaStr)

					if err != nil {
						return err
					}

					turnVolume(ctx, delta)
					return nil
				},
			},
		},
		Before: func(cCtx *cli.Context) error {
			source := cCtx.Bool("source")

			if !source {
				ctx = &DeviceContext{
					defaultId:   "@DEFAULT_AUDIO_SINK@",
					notifTag:    "wpak-sink",
					statSection: "Sinks",
					icon:        "󰓃",
					rofiPrompt:  "sink",
				}
			} else {
				ctx = &DeviceContext{
					defaultId:   "@DEFAULT_AUDIO_SOURCE@",
					notifTag:    "wpak-source",
					statSection: "Sources",
					icon:        "󰍬",
					rofiPrompt:  "source",
				}
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
