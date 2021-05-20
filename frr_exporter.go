package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/tynany/frr_exporter/collector"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddress   = kingpin.Flag("web.listen-address", "Address on which to expose metrics and web interface.").Default(":9342").String()
	telemetryPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	frrVTYSHPath    = kingpin.Flag("frr.vtysh.path", "Path of vtysh.").Default("/usr/bin/vtysh").String()
	frrVTYSHTimeout = kingpin.Flag("frr.vtysh.timeout", "The timeout when running vtysh commends (default 20s).").Default("20s").String()

	collectors = []*collector.Collector{}
)

func initCollectors() {
	bgp := collector.NewBGPCollector()
	collectors = append(collectors, &collector.Collector{
		Name:          bgp.Name(),
		PromCollector: bgp,
		Errors:        bgp,
		CLIHelper:     bgp,
	})
	ospf := collector.NewOSPFCollector()
	collectors = append(collectors, &collector.Collector{
		Name:          ospf.Name(),
		PromCollector: ospf,
		Errors:        ospf,
		CLIHelper:     ospf,
	})
	bgp6 := collector.NewBGP6Collector()
	collectors = append(collectors, &collector.Collector{
		Name:          bgp6.Name(),
		PromCollector: bgp6,
		Errors:        bgp6,
		CLIHelper:     bgp6,
	})
	// bgpl2vpn := collector.NewBGPL2VPNCollector()
	// collectors = append(collectors, &collector.Collector{
	// 	Name:          bgpl2vpn.Name(),
	// 	PromCollector: bgpl2vpn,
	// 	Errors:        bgpl2vpn,
	// 	CLIHelper:     bgpl2vpn,
	// })
	bfd := collector.NewBFDCollector()
	collectors = append(collectors, &collector.Collector{
		Name:          bfd.Name(),
		PromCollector: bfd,
		Errors:        bfd,
		CLIHelper:     bfd,
	})
}

func handler(w http.ResponseWriter, r *http.Request) {
	registry := prometheus.NewRegistry()
	enabledCollectors := []*collector.Collector{}
	for _, collector := range collectors {
		if *collector.Enabled {
			enabledCollectors = append(enabledCollectors, collector)
		}
	}
	ne := collector.NewExporter(enabledCollectors)
	ne.SetVTYSHPath(*frrVTYSHPath)

	// error checking is done as part of parseCLI
	frrTimeout, _ := time.ParseDuration(*frrVTYSHTimeout)
	ne.SetVTYSHTimeout(frrTimeout)

	registry.Register(ne)

	gatheres := prometheus.Gatherers{
		prometheus.DefaultGatherer,
		registry,
	}
	handlerOpts := promhttp.HandlerOpts{
		ErrorLog:      log.NewErrorLogger(),
		ErrorHandling: promhttp.ContinueOnError,
	}
	promhttp.HandlerFor(gatheres, handlerOpts).ServeHTTP(w, r)
}

func parseCLI() {
	for _, collector := range collectors {
		defaultState := "disabled"
		enabledByDefault := collector.CLIHelper.EnabledByDefault()
		if enabledByDefault == true {
			defaultState = "enabled"
		}
		flagName := fmt.Sprintf("collector.%s", collector.CLIHelper.Name())
		helpString := fmt.Sprintf("%s (default: %s).", collector.CLIHelper.Help(), defaultState)
		collector.Enabled = kingpin.Flag(flagName, helpString).Default(strconv.FormatBool(enabledByDefault)).Bool()
	}
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("frr_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	if _, err := time.ParseDuration(*frrVTYSHTimeout); err != nil {
		log.Fatalf("invalid frr.vtysh.timeout flag %q: %s", *frrVTYSHTimeout, err)
	}
}

func main() {
	prometheus.MustRegister(version.NewCollector("frr_exporter"))

	initCollectors()
	parseCLI()

	log.Infof("Starting frr_exporter %s on %s", version.Info(), *listenAddress)

	http.HandleFunc(*telemetryPath, handler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>FRR Exporter</title></head>
			<body>
			<h1>FRR Exporter</h1>
			<p><a href="` + *telemetryPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatal(err)
	}
}
