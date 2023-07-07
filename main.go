package main

import (
	"io"
	"log"
	"os/exec"
	"strings"
)

func getAudioSinks() ([]string, int) {
	cmd := exec.Command("wpctl", "status")

	bytes, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	header := ""
	readingSection := false

	entries := []string{}
	activeEntryIdx := 0

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
						activeEntryIdx = len(entries)
					}

					entries = append(entries, ln[10:])
				} else {
					break
				}
			} else {
				readingSection = strings.HasSuffix(ln, "Sinks:")
			}
		}
	}

	return entries, activeEntryIdx
}

func setDefault(deviceId string) {
	cmd := exec.Command("wpctl", "set-default", deviceId)
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	devices, activeDeviceIdx := getAudioSinks()
	activeDeviceName := devices[activeDeviceIdx]

	cmd := exec.Command("rofi", "-dmenu", "-no-custom", "-select", activeDeviceName)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	io.WriteString(stdin, strings.Join(devices, "\n"))
	stdin.Close()

	bytes, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	out := string(bytes[:])
	deviceId := strings.Split(out, ".")[0]

	setDefault(deviceId)
}
