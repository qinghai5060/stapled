package stapled

import (
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/jmhodges/clock"
)

type CertDefinition struct {
	Certificate            string
	Name                   string
	ResponseName           string
	Issuer                 string
	Serial                 string
	Responders             []string
	Proxy                  string
	OverrideGlobalUpstream bool `yaml:"override-global-upstream"`
	OverrideGlobalProxy    bool `yaml:"override-global-proxy"`
}

func DefToEntry(def CertDefinition, globalUpstream []string, globalProxy string) (*Entry, error) {
	e := &Entry{}
	if def.Certificate != "" {
		// load certificate (and opportunistically check for issuer)
		// e.loadCertificate(def.Certificate, def.Issuer)
	} else {
		// load certificate definition info
		// e.loadCertificateInfo(def.Name, def.Serial)
	}
	if e.issuer == nil {
		// load issuer
		// e.loadIssuer(def.Issuer)
	}
	if len(globalUpstream) > 0 && !def.OverrideGlobalUpstream {
		e.responders = globalUpstream
	} else if len(def.Responders) > 0 {
		e.responders = def.Responders
	}
	if globalProxy != "" && !def.OverrideGlobalProxy {
		// e.loadProxy(globalProxy)
	} else if def.Proxy != "" {
		// e.loadProxy(def.Proxy)
	}
	if def.ResponseName != "" {
		e.responseFilename = def.ResponseName
	}
	return e, nil
}

func CertDefToEntryDef(logger *Logger, clk clock.Clock, timeout, backoff time.Duration, cacheFolder string, upstreamStapleds []string, proxy string, def CertDefinition) (*EntryDefinition, error) {
	ed := &EntryDefinition{
		Name:        path.Base(def.Certificate),
		Log:         logger,
		Clk:         clk,
		Timeout:     timeout,
		Backoff:     backoff,
		Serial:      big.NewInt(0),
		CacheFolder: cacheFolder,
	}
	if def.Certificate == "" && (def.Serial == "" || def.Name == "") {
		return nil, fmt.Errorf("either 'certificate' or 'name' and 'serial' are required")
	}
	var cert *x509.Certificate
	var err error

	// this whole thing is... horrific
	if def.Serial != "" {
		ed.Name = def.Name
		serialBytes, err := hex.DecodeString(def.Serial)
		if err != nil {
			return nil, fmt.Errorf("failed to decode serial '%s': %s", def.Serial, err)
		}
		ed.Serial = ed.Serial.SetBytes(serialBytes)
	} else {
		cert, err = ReadCertificate(def.Certificate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate '%s': %s", def.Certificate, err)
		}
		ed.Serial = cert.SerialNumber
		ed.Responders = cert.OCSPServer
	}
	if def.Issuer != "" {
		ed.Issuer, err = ReadCertificate(def.Issuer)
		if err != nil {
			return nil, fmt.Errorf("failed to parse issuer '%s': %s", def.Issuer, err)
		}
	} else if cert != nil {
		if len(cert.IssuingCertificateURL) == 0 {
			return nil, fmt.Errorf("issuer can only be ommited if the certificate contains AIA information about its issuer")
		}
		for _, issuerURL := range cert.IssuingCertificateURL {
			// this should be its own function
			resp, err := http.Get(issuerURL)
			if err != nil {
				logger.Err("Failed to retrieve issuer from '%s': %s", issuerURL, err)
				continue
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				logger.Err("Failed to read issuer body from '%s': %s", issuerURL, err)
				continue
			}
			ed.Issuer, err = ParseCertificate(body)
			if err != nil {
				logger.Err("Failed to parse issuer body from '%s': %s", issuerURL, err)
				continue
			}
		}
	} else {
		return nil, fmt.Errorf("issuer can only be ommited if the certificate contains AIA information about its issuer")
	}
	if ed.Issuer == nil {
		return nil, fmt.Errorf("unable to retrieve issuer")
	}

	if len(def.Responders) > 0 {
		ed.Responders = def.Responders
	}
	if len(upstreamStapleds) > 0 && !def.OverrideUpstream {
		ed.Responders = upstreamStapleds
	}
	if len(ed.Responders) == 0 {
		return nil, fmt.Errorf("no responders provided")
	}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %s", err)
		}
		ed.Proxy = http.ProxyURL(proxyURL)
	}
	return ed, nil
}

type FetcherConfig struct {
	Timeout          string
	BaseBackoff      string `yaml:"base-backoff"`
	Proxy            string
	UpstreamStapleds []string `yaml:"upstream-stapleds"`
}

type CertificateDefinitions struct {
	Folder       string
	IssuerFolder string `yaml:"issuer-folder"`
	Certificates []CertDefinition
}

type Configuration struct {
	DontDieOnStaleResponse bool `yaml:"dont-die-on-stale-response"`
	DontSeedCacheFromDisk  bool `yaml:"dont-seed-cache-from-disk"`
	DontCache              bool `yaml:"dont-cache"`

	Syslog struct {
		Network     string
		Addr        string
		StdoutLevel int `yaml:"stdout-level"`
	}
	StatsAddr string `yaml:"stats-addr"`

	HTTP struct {
		Addr string
	}

	Disk struct {
		CacheFolder string `yaml:"cache-folder"`
	}

	Fetcher FetcherConfig

	Definitions CertificateDefinitions
}
