// IoT Wifi Management

// todo: update documentation!!!!
// todo: update Dockerfile
// todo: listen for shutdown signal, remove uap0, kill wpa,apd,dnsmasq

package iotwifi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

// ApiReturn structures a message for returned API calls.
type ApiReturn struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Payload interface{} `json:"payload"`
}

type HttpHandler struct {
	wpacfg   *WpaCfg
	messages chan CmdMessage
	signal   chan string
	disabled bool
}

func NewHttpHandler(setupCfg *SetupCfg, disabled bool) *HttpHandler {
	wpacfg := NewWpaCfg(setupCfg)

	//Todo: is a queue of 1 blocking wpa,hostapd,dnsmasq?
	messages := make(chan CmdMessage, 1)
	signal := make(chan string, 1)

	go HandleLog(messages)
	go RunWifi(messages, signal, setupCfg)
	if !disabled {
		if WpaSupplicantHasNetowrkConfig(setupCfg.WpaSupplicantCfg.CfgFile) {
			signal <- "CL"
		} else {
			signal <- "AP"
		}
	}
	go MonitorWPA(signal)
	go MonitorAPD(signal, setupCfg.WpaSupplicantCfg.CfgFile)

	return &HttpHandler{
		wpacfg:   wpacfg,
		messages: messages,
		signal:   signal,
		disabled: disabled,
	}
}

func apiPayloadReturn(w http.ResponseWriter, message string, payload interface{}) {
	apiReturn := &ApiReturn{
		Status:  "OK",
		Message: message,
		Payload: payload,
	}
	ret, _ := json.Marshal(apiReturn)

	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)
}

// marshallPost populates a struct with json in post body
func marshallPost(w http.ResponseWriter, r *http.Request, v interface{}) {
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	defer r.Body.Close()

	decoder := json.NewDecoder(strings.NewReader(string(bytes)))

	err = decoder.Decode(&v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
}

// common error return from api
func retError(w http.ResponseWriter, err error) {
	apiReturn := &ApiReturn{
		Status:  "FAIL",
		Message: err.Error(),
	}
	ret, _ := json.Marshal(apiReturn)

	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)
}

// handle /status POSTs json in the form of WpaConnect
func (ap *HttpHandler) StatusHandler(w http.ResponseWriter, r *http.Request) {

	status, _ := ap.wpacfg.Status()

	apiPayloadReturn(w, "status", status)
}

// handle /connect POSTs json in the form of WpaConnect
func (ap *HttpHandler) ConnectHandler(w http.ResponseWriter, r *http.Request) {
	var creds WpaCredentials
	marshallPost(w, r, &creds)

	log.Info("Connect Handler Got: ssid:|%s| psk:|redacted|", creds.Ssid)

	go ap.wpacfg.ConnectNetwork(creds)

	apiReturn := &ApiReturn{
		Status:  "OK",
		Message: "Connection",
		Payload: "Attempting to connect to " + creds.Ssid,
	}

	ret, err := json.Marshal(apiReturn)
	if err != nil {
		retError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)
	ap.signal <- "CL"
}

// scan for wifi networks
func (ap *HttpHandler) ScanHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("Got Scan")
	wpaNetworks, err := ap.wpacfg.ScanNetworks()
	if err != nil {
		retError(w, err)
		return
	}

	apiReturn := &ApiReturn{
		Status:  "OK",
		Message: "Networks",
		Payload: wpaNetworks,
	}

	ret, err := json.Marshal(apiReturn)
	if err != nil {
		retError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)
}

// kill the application
func (ap *HttpHandler) ResetHandler(w http.ResponseWriter, r *http.Request) {
	apiReturn := &ApiReturn{
		Status:  "OK",
		Message: "Disconnected from configured network",
	}

	if err := ap.wpacfg.DisconnectNetwork("0"); err != nil {
		apiReturn.Status = "FAILED"
		apiReturn.Message = fmt.Sprintf("Failed %s", err)
	} else {
		ap.signal <- "AP"
	}

	ret, err := json.Marshal(apiReturn)
	if err != nil {
		retError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)
}

// kill the application
func (ap *HttpHandler) StopHandler(w http.ResponseWriter, r *http.Request) {
	ap.signal <- "OFF"

	apiReturn := &ApiReturn{
		Status:  "OK",
		Message: "Stopping service.",
	}
	ret, err := json.Marshal(apiReturn)
	if err != nil {
		retError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)
}

// kill the application
func (ap *HttpHandler) StartHandler(w http.ResponseWriter, r *http.Request) {
	ap.signal <- "AP"

	apiReturn := &ApiReturn{
		Status:  "OK",
		Message: "Starting service.",
	}
	ret, err := json.Marshal(apiReturn)
	if err != nil {
		retError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)
}

/*// common log middleware for api
func (ap *HttpHandler) LogHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		staticFields := make(map[string]interface{})
		staticFields["remote"] = r.RemoteAddr
		staticFields["method"] = r.Method
		staticFields["url"] = r.RequestURI

		log.Info(staticFields, "HTTP")
		next.ServeHTTP(w, r)
	})
}*/
