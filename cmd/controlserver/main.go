package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/caldog20/calnet/control/server/apiservice"
	"github.com/caldog20/calnet/control/server/config"
	control "github.com/caldog20/calnet/control/server/controlservice"
	"github.com/caldog20/calnet/control/server/relayservice"
	"github.com/caldog20/calnet/control/server/store"
	"golang.org/x/crypto/acme/autocert"
)

var (
	httpPort  = flag.Int("http-port", 0, "http listen port")
	debugMode = flag.Bool("debug", false, "enable debug mode disables encryption and ssl")
)

func main() {
	flag.Parse()
	conf := getConfig()

	db, err := store.NewBoltStore(conf.StorePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	control := control.New(conf, db)
	defer control.Close()

	relay := relayservice.New()
	relay.SetKeyVerifier(control.VerifyKeyForRelay)
	defer relay.Close()

	api := apiservice.New(db)


	mux := http.NewServeMux()
	control.RegisterRoutes(mux)
	relay.RegisterRoutes(mux)
	api.RegisterRoutes(mux)

	srv := &http.Server{
		Handler: mux,
	}

	var l net.Listener
	if conf.AutoCertDomain != "" {
		l = autocert.NewListener(conf.AutoCertDomain)
	} else {
		l, err = net.Listen("tcp", fmt.Sprintf(":%d", uint16(conf.HTTPPort)))
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("http server listening on %s", l.Addr().String())
	defer l.Close()

	// ctx, cancel := signal.NotifyContext(
	// 	context.Background(),
	// 	os.Interrupt,
	// 	syscall.SIGQUIT,
	// 	syscall.SIGTERM,
	// )

	srv.Serve(l)
}

func getConfig() config.Config {
	var conf config.Config

	err := conf.ReadConfigFromFile()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			conf.SetDefaults()
			err = conf.WriteConfigFile()
			if err != nil {
				log.Printf("error writing config file to disk: %s", err)
			}
		}
	}

	// Flags are prioritized over config entries for now
	if *httpPort != 0 {
		conf.HTTPPort = *httpPort
	}
	if *debugMode {
		if conf.Debug != *debugMode {
			conf.Debug = *debugMode
		}
	}

	return conf
}
