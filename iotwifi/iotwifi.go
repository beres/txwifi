// IoT Wifi packages is used to manage WiFi AP and Station (client) modes on
// a Raspberry Pi or other arm device. This code is intended to run in it's
// corresponding Alpine docker container.

package iotwifi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// CmdRunner runs internal commands allows output handlers to be attached.
type CmdRunner struct {
	Messages chan CmdMessage
	Handlers map[string]func(CmdMessage)
	Commands map[string]*exec.Cmd
}

// CmdMessage structures command output.
type CmdMessage struct {
	Id      string
	Command string
	Message string
	Error   bool
	Cmd     *exec.Cmd
	Stdin   *io.WriteCloser
}

func hostAPdConfig(setupCfg *SetupCfg) {
	cfg := `interface=uap0
ssid=` + setupCfg.HostApdCfg.Ssid + `
hw_mode=g
channel=` + setupCfg.HostApdCfg.Channel + `
macaddr_acl=0
ignore_broadcast_ssid=0
disassoc_low_ack=0
skip_inactivity_poll=1
ctrl_interface=/var/run/hostapd
ctrl_interface_group=0`

	if setupCfg.HostApdCfg.WpaPassphrase != "" {
		cfg = fmt.Sprintf(`%s
auth_algs=1
wpa=2
wpa_passphrase=%s
wpa_key_mgmt=WPA-PSK
wpa_pairwise=TKIP
rsn_pairwise=CCMP
`, cfg, setupCfg.HostApdCfg.WpaPassphrase)
	}

	err := ioutil.WriteFile("/etc/hostapd/hostapd.conf", []byte(cfg), 0600)
	if err != nil {
		panic(err)
	}

}

// loadCfg loads the configuration.
func LoadCfg(cfgLocation string) (*SetupCfg, error) {

	v := &SetupCfg{}
	var jsonData []byte

	urlDelimR, _ := regexp.Compile("://")
	isUrl := urlDelimR.Match([]byte(cfgLocation))

	// if not a url
	if !isUrl {
		fileData, err := ioutil.ReadFile(cfgLocation)
		if err != nil {
			panic(err)
		}
		jsonData = fileData
	}

	if isUrl {
		res, err := http.Get(cfgLocation)
		if err != nil {
			panic(err)
		}

		defer res.Body.Close()

		urlData, err := ioutil.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		jsonData = urlData
	}

	err := json.Unmarshal(jsonData, v)

	return v, err
}

func MonitorAPD(signal chan<- string, wpaSupplicantConfig string) {
	var apdTimeout int64 = 90
	staticFields := make(map[string]interface{})
	staticFields["cmd_id"] = " ~~ apd monitor ~~"
	log.Info(staticFields, "Start.")
	for {
		apd := apdState("uap0")
		if apd == "ENABLED" {
			startTime := time.Now().Unix()
			log.Info(staticFields, apd+", timeout in "+strconv.FormatInt(apdTimeout, 10)+" seconds")
			for {
				apd = apdState("uap0")
				if startTime+apdTimeout < time.Now().Unix() {
					//check to see if APD has clients, if yes, reset timer
					if apdHasClient("uap0") {
						log.Info(staticFields, "has client(s), timeout aborted")
						break
					}
					//check to see if WPA has any networks configured, if none, reset timer
					if !WpaSupplicantHasNetowrkConfig(wpaSupplicantConfig) {
						log.Info(staticFields, "wpa_supplicant has no network config, timeout aborted")
						break
					}
					log.Info(staticFields, "Timeout.")
					signal <- "CL"
					break
				}
				if apd != "ENABLED" {
					log.Info(staticFields, apd+" timeout aborted ")
					break
				}
				time.Sleep(1 * time.Second)
			}
		}
		time.Sleep(30 * time.Second)
	}

}

func MonitorWPA(signal chan<- string) {
	var wpaTimeout int64 = 90
	staticFields := make(map[string]interface{})
	staticFields["cmd_id"] = " ~~ wpa monitor ~~"
	log.Info(staticFields, "Start.")
	for {
		wpa := wpaState("wlan0")
		if wpa != "COMPLETED" && wpa != "NONE" {
			startTime := time.Now().Unix()
			log.Info(staticFields, wpa+", timeout in "+strconv.FormatInt(wpaTimeout, 10)+" seconds")
			for {
				wpa = wpaState("wlan0")
				if startTime+wpaTimeout < time.Now().Unix() {
					log.Info(staticFields, "Timeout.")
					signal <- "AP"
					break
				}
				if wpa == "COMPLETED" {
					log.Info(staticFields, wpa+" timeout aborted")
					break
				}
				time.Sleep(1 * time.Second)
			}
		}
		time.Sleep(30 * time.Second)
	}
}

// RunWifi starts AP and Station modes.
func RunWifi(messages chan CmdMessage, signal chan string, setupCfg *SetupCfg) {
	staticFields := make(map[string]interface{})

	log.Info("Loading IoT Wifi...")

	cmdRunner := CmdRunner{
		Messages: messages,
		Handlers: make(map[string]func(cmsg CmdMessage), 0),
		Commands: make(map[string]*exec.Cmd, 0),
	}

	command := &Command{
		Runner:   cmdRunner,
		SetupCfg: setupCfg,
	}

	for {
		mode := <-signal
		log.Info(staticFields, "Signal: "+mode)
		if mode == "AP" {
			log.Info(staticFields, "-=-=-=- start Access Point -=-=-=-")
			command.killIt("wpa_supplicant")
			command.killIt("hostapd")
			command.killIt("dnsmasq")
			log.Info(staticFields, "... wait for wpa_supplicant to finish")
			for {
				if wpaState("wlan0") == "NONE" {
					log.Info(staticFields, "wpa_supplicant finished")
					break
				}
			}
			command.RemoveApInterface()
			command.AddApInterface()
			command.UpApInterface()
			command.ConfigureApInterface()
			hostAPdConfig(setupCfg)
			command.StartHostAPD() //hostapd
			log.Info(staticFields, "... wait for host_apd to start")
			for {
				if apdState("uap0") != "NONE" {
					log.Info(staticFields, "host_apd started")
					break
				}
			}
			command.StartDnsmasq() //dnsmasq
		}
		if mode == "CL" {
			if wpaState("wlan0") != "NONE" {
				log.Info(staticFields, "-=-=-=- client already started. -=-=-=-")
				continue
			}
			log.Info(staticFields, "-=-=-=- start Client -=-=-=-")
			command.killIt("wpa_supplicant")
			command.killIt("hostapd")
			command.killIt("dnsmasq")
			log.Info(staticFields, "... wait for host_apd to finish")
			for {
				if apdState("uap0") == "NONE" {
					log.Info(staticFields, "host_apd finished")
					break
				}
			}
			command.RemoveApInterface()
			command.StartWpaSupplicant()
		}
		if mode == "OFF" {
			command.killIt("wpa_supplicant")
			command.killIt("hostapd")
			command.killIt("dnsmasq")
			command.RemoveApInterface()
		}
	}
}

func HandleLog(messages chan CmdMessage) {

	cmdRunner := CmdRunner{
		Messages: messages,
		Handlers: make(map[string]func(cmsg CmdMessage), 0),
		Commands: make(map[string]*exec.Cmd, 0),
	}

	// staticFields for logger
	staticFields := make(map[string]interface{})

	// command output loop (channel messages)
	// loop and log
	//
	for {
		out := <-messages // Block until we receive a message on the channel

		staticFields["cmd_id"] = out.Id
		staticFields["cmd"] = out.Command
		staticFields["is_error"] = out.Error

		log.Info(staticFields, out.Message)

		if handler, ok := cmdRunner.Handlers[out.Id]; ok {
			handler(out)
		}
	}
}

// HandleFunc is a function that gets all channel messages for a command id
func (c *CmdRunner) HandleFunc(cmdId string, handler func(cmdMessage CmdMessage)) {
	c.Handlers[cmdId] = handler
}

// ProcessCmd processes an internal command.
func (c *CmdRunner) ProcessCmd(id string, cmd *exec.Cmd) {
	log.Debug("ProcessCmd got %s", id)

	// add command to the commands map TODO close the readers
	c.Commands[id] = cmd

	cmdStdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	cmdStderrReader, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	stdOutScanner := bufio.NewScanner(cmdStdoutReader)
	go func() {
		for stdOutScanner.Scan() {
			c.Messages <- CmdMessage{
				Id:      id,
				Command: cmd.Path,
				Message: stdOutScanner.Text(),
				Error:   false,
				Cmd:     cmd,
			}
		}
	}()

	stdErrScanner := bufio.NewScanner(cmdStderrReader)
	go func() {
		for stdErrScanner.Scan() {
			c.Messages <- CmdMessage{
				Id:      id,
				Command: cmd.Path,
				Message: stdErrScanner.Text(),
				Error:   true,
				Cmd:     cmd,
			}
		}
	}()

	err = cmd.Start()

	if err != nil {
		panic(err)
	}

	log.Debug("ProcessCmd waiting %s", id)
	err = cmd.Wait()
	log.Debug("ProcessCmd done %s ", id)

}

func WpaSupplicantHasNetowrkConfig(wpaSupplicantConfig string) bool {
	fileData, err := ioutil.ReadFile(wpaSupplicantConfig)
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(fileData), "\n")
	for _, line := range lines {
		if strings.Contains(line, "network={") {
			return true
		}
	}
	return false
}
