// IoT Wifi Management

// todo: update documentation!!!!
// todo: update Dockerfile
// todo: listen for shutdown signal, remove uap0, kill wpa,apd,dnsmasq

package main

import (
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/txn2/txwifi/iotwifi"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func main() {
	log.Info("Starting IoT Wifi...")

	cfgUrl := setEnvIfEmpty("IOTWIFI_CFG", "cfg/wificfg.json")
	port := setEnvIfEmpty("IOTWIFI_PORT", "8080")
	static := setEnvIfEmpty("IOTWIFI_STATIC", "/static/")

	setupCfg, err := iotwifi.LoadCfg(cfgUrl)
	if err != nil {
		log.Error("Could not load config: %s", err.Error())
		panic(err)
	}

	h := iotwifi.NewHttpHandler(setupCfg, true)

	// setup router and middleware
	r := mux.NewRouter()
	//r.Use(h.LogHandler)

	// set app routes
	r.HandleFunc("/status", h.StatusHandler)
	r.HandleFunc("/connect", h.ConnectHandler).Methods("POST")
	r.HandleFunc("/scan", h.ScanHandler)

	// ---
	if setupCfg.DontFallBackToApMode {
		r.HandleFunc("/reset", h.ResetHandler)
	}

	if setupCfg.AllowStartStop {
		r.HandleFunc("/start", h.StartHandler)
		r.HandleFunc("/stop", h.StopHandler)
	}

	r.PathPrefix("/").Handler(http.FileServer(http.Dir(static)))
	http.Handle("/", r)

	// CORS
	headersOk := handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "Content-Length", "X-Requested-With", "Accept", "Origin"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})

	// serve http
	log.Info("HTTP Listening on " + port)
	http.ListenAndServe(":"+port, handlers.CORS(originsOk, headersOk, methodsOk)(r))

}

// getEnv gets an environment variable or sets a default if
// one does not exist.
func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}

	return value
}

// setEnvIfEmp<ty sets an environment variable to itself or
// fallback if empty.
func setEnvIfEmpty(env string, fallback string) (envVal string) {
	envVal = getEnv(env, fallback)
	os.Setenv(env, envVal)

	return envVal
}
