package exporter

import (
	"log"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/pkg/errors"
	"github.com/rerorero/netscaler-vpx-exporter/exporter/conf"
	"github.com/rerorero/netscaler-vpx-exporter/exporter/netscaler"
)

type Exporter interface {
	prometheus.Collector
}

type exporterImpl struct {
	conf       *conf.Conf
	netscalers []netscaler.Netscaler
}

func NewExporter(config *conf.Conf) (Exporter, error) {
	nsary := []netscaler.Netscaler{}
	for _, nsconf := range config.Netscaler.StaticTargets {
		ns, err := netscaler.NewNetscalerClient(nsconf)
		if err != nil {
			return nil, errors.Wrap(err, "error : Failed to instantiate Netscaler client")
		}
		nsary = append(nsary, ns)
	}

	return &exporterImpl{
		conf:       config,
		netscalers: nsary,
	}, nil
}

func (e *exporterImpl) Describe(ch chan<- *prometheus.Desc) {
	// global
	for _, metric := range globalMetrics {
		metric.GetCollector().Describe(ch)
	}

	// vserver
	for _, metric := range vserverMetrics {
		metric.GetCollector().Describe(ch)
	}
}

func (e *exporterImpl) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	for _, ns := range e.netscalers {
		wg.Add(1)
		go doCollect(ns, ch, wg)
	}
	wg.Wait()
}

func doCollect(ns netscaler.Netscaler, ch chan<- prometheus.Metric, wg *sync.WaitGroup) {
	stats, errors := ns.GetStats()
	for _, err := range errors {
		log.Println("warn : Failed to get stats from ", ns.GetHost(), err.Error())
	}

	// global
	for _, metric := range globalMetrics {
		labels := prometheus.Labels{LabelNsHost: ns.GetHost()}
		metric.Update(stats, labels)
		metric.GetCollector().Collect(ch)
	}

	// vserver
	for _, metric := range vserverMetrics {
		if stats != nil {
			for vserver, _ := range stats.Http.VServers {
				labels := prometheus.Labels{
					LabelNsHost:  ns.GetHost(),
					LabelVServer: vserver,
				}
				metric.Update(stats, labels)
			}
		} else {
			metric.Update(nil, nil)
		}
		metric.GetCollector().Collect(ch)
	}

	wg.Done()
}
