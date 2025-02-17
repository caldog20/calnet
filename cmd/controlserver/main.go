package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caldog20/calnet/control/server/apiservice"
	"github.com/caldog20/calnet/control/server/config"
	"github.com/caldog20/calnet/control/server/controlservice"
	"github.com/caldog20/calnet/control/server/relayservice"
	"github.com/caldog20/calnet/control/server/store"
	"github.com/caldog20/calnet/control/server/stunservice"
	"golang.org/x/crypto/acme/autocert"
)

var (
	httpPort   = flag.Int("http-port", 0, "http listen port")
	stunPort   = flag.Int("stun-port", 0, "stun listen port")
	debugMode  = flag.Bool("debug", false, "enable debug mode disables encryption and ssl")
	configPath = flag.String(
		"config",
		"",
		"path to read config file - if unset, config will try to read from standard os config paths",
	)
)

func main() {
	flag.Parse()
	conf := getConfig()

	db, err := store.NewBoltStore(conf.StorePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	control := controlservice.New(conf, db)
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

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGQUIT,
		syscall.SIGTERM,
	)
	defer cancel()

	go srv.Serve(l)

	err = stunservice.ListenAndServe(ctx, fmt.Sprintf(":%d", uint16(conf.StunPort)))
	if err != nil {
		log.Printf("stun server error: %s", err)
		cancel()
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	err = srv.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf("error gracefully closing http server: %s", err)
		srv.Close()
	}
}

func getConfig() config.Config {
	var conf config.Config

	if *configPath != "" {
		config.SetConfigPath(*configPath)
	}

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
	// These are not currently writen back to the config file
	if *httpPort != 0 {
		conf.HTTPPort = *httpPort
	}

	if *stunPort != 0 {
		conf.StunPort = *stunPort
	}

	if *debugMode {
		log.Println("server running in debug mode!")
		if conf.Debug != *debugMode {
			conf.Debug = *debugMode
		}
	}

	return conf
}
