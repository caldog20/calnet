package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caldog20/calnet/manager"
	"github.com/caldog20/calnet/manager/ipam"
	"github.com/caldog20/calnet/manager/store"
	"golang.org/x/crypto/acme/autocert"
)

var (
	dbPath           = flag.String("db-path", "./calnet.db", "database file path (use :memory: for in memory sqlite db)")
	httpAddr         = flag.String("http-addr", ":443", "http listen address")
	stunAddr         = flag.String("stun-addr", ":3478", "stun listen address")
	noStun           = flag.Bool("nostun", false, "disable stun server")
	useAutocert      = flag.Bool("autocert", false, "enable auto cert")
	autocertEmail    = flag.String("autocert-email", os.Getenv("CALNET_AUTOCERT_EMAIL"), "autocert email address")
	autocertCacheDir = flag.String("autocert-cache-dir", os.Getenv("CALNET_AUTOCERT_CACHE_DIR"), "autocert cache directory")
	autocertDomain   = flag.String("autocert-domain", os.Getenv("CALNET_AUTOCERT_DOMAIN"), "autocert domain")
	netPrefix        = flag.String("net-prefix", "100.70.0.0/24", "network prefix")
	debug            = flag.Bool("debug", true, "enable debug mode (runs http on 127.0.0.1:8080 unless http-addr specified and disables production features)")
)

func main() {
	flag.Parse()

	if *debug {
		if *httpAddr == ":443" {
			*httpAddr = ":8080"
		}
		*useAutocert = false
	}

	prefix, err := netip.ParsePrefix(*netPrefix)
	if err != nil {
		log.Fatal("error parsing netip prefix:", err)
	}

	//db, err := store.NewSqlStore(*dbPath)
	//if err != nil {
	//	log.Fatal("failed to open database:", err)
	//}

	db := store.NewMapStore()

	ipam, err := ipam.New(prefix, db)
	if err != nil {
		log.Fatal("failed to initialize ipam:", err)
	}

	nodeManager := manager.NewNodeManager(db)

	s := manager.NewServer(db, ipam, nodeManager)

	ln, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		log.Fatal("failed to listen for http:", err)
	}

	httpSrv := &http.Server{
		Handler: s,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if !(*noStun) {
		//go func() {
		//	err := stun.ListenAndServe(ctx, *stunAddr)
		//	if err != nil {
		//		log.Println("stun server error:", err)
		//		cancel()
		//	}
		//}()
	}

	if *useAutocert {
		ac := tryEnableAutocert()
		if ac != nil {
			httpSrv.TLSConfig = ac.TLSConfig()
			go func() {
				if err := httpSrv.ServeTLS(ln, "", ""); !errors.Is(err, http.ErrServerClosed) {
					log.Fatal(err)
				}
			}()
		}
	} else {
		go func() {
			if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err)
			}
		}()
	}

	log.Println("manager started successfully")
	log.Println("http listening on", ln.Addr())
	if !(*noStun) {
		log.Println("stun listening on", *stunAddr)
	}

	<-ctx.Done()
	log.Println("shutting down")
	nodeManager.CloseAll()
	cancelCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_ = httpSrv.Shutdown(cancelCtx)
}

func tryEnableAutocert() *autocert.Manager {
	if *autocertDomain == "" {
		log.Println("cannot enable autocert: must specify --autocert-domain")
		return nil
	}
	if *autocertEmail == "" {
		log.Println("cannot enable autocert: must specify --autocert-email")
		return nil
	}
	if *autocertCacheDir == "" {
		log.Println("warning: autocert cache directory defaulting to /tmp - specify with --autocert-cache-dir")
	}
	return &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(*autocertDomain),
		Cache:      autocert.DirCache(*autocertCacheDir),
		Email:      *autocertEmail,
	}
}
