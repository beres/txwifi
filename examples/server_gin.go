// IoT Wifi Management

// todo: update documentation!!!!
// todo: update Dockerfile
// todo: listen for shutdown signal, remove uap0, kill wpa,apd,dnsmasq

package main

import (
	"os"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/txn2/txwifi/iotwifi"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func NewSetupCfg() *iotwifi.SetupCfg {
	cfg := iotwifi.SetupCfg{
		iotwifi.DnsmasqCfg{
			"/#/10.10.10.1", "10.10.10.10,10.10.10.20,1h", "set:device,IoT",
		},
		iotwifi.HostApdCfg{
			"Test AP", "", "6", "10.10.10.1",
		},
		iotwifi.WpaSupplicantCfg{
			"/etc/wpa_supplicant/wpa_supplicant.conf",
		},
		true,
		true,
	}
	return &cfg
}

func main() {
	log.Info("Starting IoT Wifi...")
	port := "8080"

	setupCfg := NewSetupCfg()
	h := iotwifi.NewHttpHandler(setupCfg, true)

	r := gin.Default()

	// set app routes
	r.GET("/status", gin.WrapF(h.StatusHandler))
	r.POST("/connect", gin.WrapF(h.ConnectHandler))
	r.GET("/scan", gin.WrapF(h.ScanHandler))

	// ---
	if setupCfg.DontFallBackToApMode {
		r.GET("/reset", gin.WrapF(h.ResetHandler))
	}

	if setupCfg.AllowStartStop {
		r.GET("/start", gin.WrapF(h.StartHandler))
		r.GET("/stop", gin.WrapF(h.StopHandler))
	}

	// CORS
	// headersOk := handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "Content-Length", "X-Requested-With", "Accept", "Origin"})
	// originsOk := handlers.AllowedOrigins([]string{"*"})
	// methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})

	// serve http
	log.Info("HTTP Listening on " + port)
	// http.ListenAndServe(":"+port, handlers.CORS(originsOk, headersOk, methodsOk)(r))
	r.Run(":" + port)
}
