package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jmhodges/clock"
)

type stapled struct {
	log                    *Logger
	clk                    clock.Clock
	c                      *cache
	responder              *http.Server
	clientTimeout          time.Duration
	clientBackoff          time.Duration
	entryMonitorTick       time.Duration
	upstreamResponders     []string
	cacheFolder            string
	dontDieOnStaleResponse bool
	certFolderWatcher      *dirWatcher
}

func New(log *Logger, clk clock.Clock, httpAddr string, timeout, backoff, entryMonitorTick time.Duration, responders []string, cacheFolder string, dontDieOnStale bool, certFolder string, entries []*Entry) (*stapled, error) {
	c := newCache(log)
	s := &stapled{
		log:                    log,
		clk:                    clk,
		c:                      c,
		clientTimeout:          timeout,
		clientBackoff:          backoff,
		entryMonitorTick:       entryMonitorTick,
		cacheFolder:            cacheFolder,
		dontDieOnStaleResponse: dontDieOnStale,
		upstreamResponders:     responders,
		certFolderWatcher:      newDirWatcher(certFolder),
	}
	// add entries to cache
	for _, e := range entries {
		c.addMulti(e)
	}
	// initialize OCSP repsonder
	s.initResponder(httpAddr, log)
	return s, nil
}

func (s *stapled) checkCertDirectory() {
	_, removed, err := s.certFolderWatcher.check()
	if err != nil {
		// log
		s.log.Err("Failed to poll certificate directory: %s", err)
		return
	}
	// for _, a := range added {
	// create entry + add to cache
	// }
	for _, r := range removed {
		// remove from cache
		s.c.remove(r)
	}
}

func (s *stapled) watchCertDirectory() {
	s.checkCertDirectory()
	ticker := time.NewTicker(time.Second * 15)
	for _ = range ticker.C {
		s.checkCertDirectory()
	}
}

func (s *stapled) Run() error {
	if s.certFolderWatcher != nil {
		go s.watchCertDirectory()
	}
	err := s.responder.ListenAndServe()
	if err != nil {
		return fmt.Errorf("HTTP server died: %s", err)
	}
	return nil
}
