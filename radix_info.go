package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jeremywohl/flatten"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var (
	registry = prometheus.NewRegistry()

	radix_validator_peers_count = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "radix_validator_peers_count",
		Help: "Count of Validator Peers",
	})

	radix_validator_next_validators_count = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "radix_validator_next_validators_count",
	})

	radix_validator_next_validators_stake_min = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "radix_validator_next_validators_stake_min",
	})

	radix_validator_next_validators_stake_max = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "radix_validator_next_validators_stake_max",
	})

	radix_validator_stake_total = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "radix_validator_stake_total",
	})

	radix_validator_delegators_count = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "radix_validator_delegators_count",
	})
)

func init() {
	registry.MustRegister(radix_validator_peers_count)
	registry.MustRegister(radix_validator_next_validators_count)
	registry.MustRegister(radix_validator_next_validators_stake_min)
	registry.MustRegister(radix_validator_next_validators_stake_max)
	registry.MustRegister(radix_validator_stake_total)
	registry.MustRegister(radix_validator_delegators_count)
}

func main() {
	var baseUrl string

	flag.StringVar(&baseUrl, "b", "http://localhost:3333", "Specify base url. Default is http://localhost:3333")

	flag.Usage = func() {
		fmt.Printf("Usage: \n")
		fmt.Printf("./main -b baseUrl outputPath \n")
	}

	flag.Parse()

	path := flag.Arg(0)
	if path == "" {
		path = "."
	}

	systemInfo(baseUrl)
	systemPeers(baseUrl)
	systemEpochproof(baseUrl)
	nodeValidator(baseUrl)

	prometheus.WriteToTextfile(path+"/radix_info.prom", registry)
}

func newClient() *http.Client {
	c := &http.Client{
		Timeout: 10 * time.Second,
	}
	return c
}

func systemInfo(baseUrl string) {
	url := baseUrl + "/system/info"
	body := getData(url)

	var info map[string]interface{}
	jsonErr := json.Unmarshal(body, &info)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	flat, flatErr := flatten.Flatten(info, "radix_", flatten.UnderscoreStyle)
	if flatErr != nil {
		log.Fatal(flatErr)
	}

	// Remove unwanted keys
	delete(flat, "radix_info_system_version_system_version_agent_version")
	delete(flat, "radix_info_system_version_system_version_protocol_version")
	delete(flat, "radix_agent_protocol")
	delete(flat, "radix_agent_version")
	delete(flat, "radix_info_configuration_pacemakerRate")
	delete(flat, "radix_info_configuration_pacemakerTimeout")
	delete(flat, "radix_info_configuration_pacemakerMaxExponent")

	// Dynamically create Gauges
	for key, val := range flat {
		v, ok := val.(float64)
		if ok {
			g := prometheus.NewGauge(prometheus.GaugeOpts{Name: key})
			registry.MustRegister(g)
			g.Set(v)
		}
	}
}

func systemPeers(baseUrl string) {
	url := baseUrl + "/system/peers"
	body := getData(url)

	var peers []interface{}
	jsonErr := json.Unmarshal(body, &peers)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	radix_validator_peers_count.Set(float64(len(peers)))
}

func systemEpochproof(baseUrl string) {
	url := baseUrl + "/system/epochproof"
	body := getData(url)

	result := gjson.GetBytes(body, "header.nextValidators.#.stake")

	nextValidators := result.Array()
	minStake, maxStake := minMax(nextValidators)

	radix_validator_next_validators_count.Set(float64(len(nextValidators)))
	radix_validator_next_validators_stake_min.Set((minStake / 1e18))
	radix_validator_next_validators_stake_max.Set((maxStake / 1e18))
}

func nodeValidator(baseUrl string) {
	url := baseUrl + "/node/validator"
	body := postData(url)

	totalStakes := gjson.GetBytes(body, "validator.totalStake").Float()
	stakes := gjson.GetBytes(body, "validator.stakes").Array()

	radix_validator_stake_total.Set(totalStakes)
	radix_validator_delegators_count.Set(float64(len(stakes)))
}

func getData(url string) []byte {
	r, getErr := newClient().Get(url)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if r.Body != nil {
		defer r.Body.Close()
	}

	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	return body
}

func postData(url string) []byte {
	r, postErr := newClient().Post(url, "application/json", nil)
	if postErr != nil {
		log.Fatal(postErr)
	}

	if r.Body != nil {
		defer r.Body.Close()
	}

	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	return body
}

func minMax(array []gjson.Result) (float64, float64) {
	var max float64 = array[0].Float()
	var min float64 = array[0].Float()
	for _, value := range array {
		v := value.Float()
		if max < v {
			max = v
		}
		if min > v {
			min = v
		}
	}
	return min, max
}
