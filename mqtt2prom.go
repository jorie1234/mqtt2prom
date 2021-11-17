package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"time"

	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type SonoffState struct {
	Time      string  `json:"Time"`
	Uptime    int     `json:"Uptime"`
	UptimeSec int     `json:"UptimeSec"`
	Heap      int     `json:"Heap"`
	SleepMode string  `json:"SleepMode"`
	Sleep     int     `json:"Sleep"`
	LoadAvg   int     `json:"LoadAvg"`
	MqttCount int     `json:"MqttCount"`
	Vcc       float64 `json:"Vcc"`
	POWER     string  `json:"POWER"`
	POWER1    string  `json:"POWER1"`
	POWER2    string  `json:"POWER2"`
	POWER3    string  `json:"POWER3"`
	POWER4    string  `json:"POWER4"`
	Wifi      struct {
		AP        int    `json:"AP"`
		SSID      string `json:"SSId"`
		BSSID     string `json:"BSSId"`
		Channel   int    `json:"Channel"`
		RSSI      int    `json:"RSSI"`
		Signal    int    `json:"Signal"`
		LinkCount int    `json:"LinkCount"`
		Downtime  string `json:"Downtime"`
	} `json:"Wifi"`
}

type SonoffSensor struct {
	Time   string `json:"Time"`
	ENERGY struct {
		Total     float64 `json:"Total"`
		Yesterday float64 `json:"Yesterday"`
		Today     float64 `json:"Today"`
		Period    int     `json:"Period"`
		Power     int     `json:"Power"`
		Factor    float64 `json:"Factor"`
		Voltage   int     `json:"Voltage"`
		Current   float64 `json:"Current"`
	} `json:"ENERGY"`
}

func main() {
	// Set up channel on which to send signal notifications.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	// Create an MQTT Client.
	cli := client.New(&client.Options{
		// Define the processing of the error handler.
		ErrorHandler: func(err error) {
			fmt.Println(err)
		},
	})

	// Terminate the Client.
	defer cli.Terminate()

	for {
		// Connect to the MQTT Server.
		err := cli.Connect(&client.ConnectOptions{
			Network:  "tcp",
			Address:  "ubuntu:1883",
			ClientID: []byte("mqtt2prom"),
		})
		if err == nil {
			break
		}
		log.Printf("Cannot connect to mqtt server %s\n", err)
		log.Print("retry in 30s")
		time.Sleep(30 * time.Second)
	}

	sonoffGaugesVCC := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "Sonoff",
		Subsystem: "STATE",
		Name:      "VCC",
		Help:      "Sonoff VCC Values",
	},
		[]string{"name", "label"},
	)
	sonoffGaugesRSSI := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "Sonoff",
		Subsystem: "STATE",
		Name:      "RSSI",
		Help:      "Sonoff RSSI Values",
	},
		[]string{"name", "label"},
	)
	sonoffGaugesPower := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "Sonoff",
		Subsystem: "STATE",
		Name:      "Power",
		Help:      "Sonoff Power Values",
	},
		[]string{"name", "label"},
	)
	sonoffGaugesVoltage := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "Sonoff",
		Subsystem: "SENSOR",
		Name:      "Voltage",
		Help:      "Sonoff Voltage Values",
	},
		[]string{"name", "label"},
	)
	sonoffGaugesCurrent := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "Sonoff",
		Subsystem: "SENSOR",
		Name:      "Current",
		Help:      "Sonoff Current Values",
	},
		[]string{"name", "label"},
	)
	sonoffGaugesTotel := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "Sonoff",
		Subsystem: "SENSOR",
		Name:      "Total",
		Help:      "Sonoff Total Values",
	},
		[]string{"name", "label"},
	)
	temperatureGauge := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "environmental",
		Subsystem: "SENSOR",
		Name:      "Temperature",
		Help:      "Temperature",
	},
		[]string{"location", "place"},
	)
	humidityGauge := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "environmental",
		Subsystem: "SENSOR",
		Name:      "Humidity",
		Help:      "Humidity",
	},
		[]string{"location", "place"},
	)
	// Subscribe to topics.
	err := cli.Subscribe(&client.SubscribeOptions{
		SubReqs: []*client.SubReq{
			&client.SubReq{
				TopicFilter: []byte("#"),
				QoS:         mqtt.QoS0,
				// Define the processing of the message handler.
				Handler: func(topicName, message []byte) {
					fmt.Println(string(topicName), string(message))
					//r := regexp.MustCompile(`(?[a-z]*)\/(?P<Label2>[[:alpha:]])\/(?P<Label3>[[:alpha:]])`)
					r := regexp.MustCompile(`^(?P<label1>.+)\/(?P<label2>.+)\/(?P<label3>.+)$`)
					fmt.Printf("%#v\n", r.FindStringSubmatch(string(topicName)))
					fmt.Printf("%#v\n", r.SubexpNames())

					a := r.FindStringSubmatch(string(topicName))
					if len(a) >= 3 {
						if a[1] == "temperature" {
							f, _ := strconv.ParseFloat(string(message), 8)
							temperatureGauge.WithLabelValues(a[2], a[3]).Set(f)
						}
						if a[1] == "humidity" {
							f, _ := strconv.ParseFloat(string(message), 8)
							humidityGauge.WithLabelValues(a[2], a[3]).Set(f)
						}
						if a[3] == "STATE" {
							state := SonoffState{}
							json.Unmarshal(message, &state)
							fmt.Printf("STATE: %#v\n", state)
							sonoffGaugesVCC.WithLabelValues(a[2], a[1]).Set(state.Vcc)
							sonoffGaugesRSSI.WithLabelValues(a[2], a[1]).Set(float64(state.Wifi.RSSI))
							if state.POWER == "ON" {
								sonoffGaugesPower.WithLabelValues(a[2], a[1]).Set(float64(1))
							}
							if state.POWER == "OFF" {
								sonoffGaugesPower.WithLabelValues(a[2], a[1]).Set(float64(0))
							}
							if state.POWER1 == "ON" {
								sonoffGaugesPower.WithLabelValues(a[2]+"1", a[1]).Set(float64(1))
							}
							if state.POWER1 == "OFF" {
								sonoffGaugesPower.WithLabelValues(a[2]+"1", a[1]).Set(float64(0))
							}
							if state.POWER2 == "ON" {
								sonoffGaugesPower.WithLabelValues(a[2]+"2", a[1]).Set(float64(1))
							}
							if state.POWER2 == "OFF" {
								sonoffGaugesPower.WithLabelValues(a[2]+"2", a[1]).Set(float64(0))
							}
							if state.POWER3 == "ON" {
								sonoffGaugesPower.WithLabelValues(a[2]+"3", a[1]).Set(float64(1))
							}
							if state.POWER3 == "OFF" {
								sonoffGaugesPower.WithLabelValues(a[2]+"3", a[1]).Set(float64(0))
							}
							if state.POWER4 == "ON" {
								sonoffGaugesPower.WithLabelValues(a[2]+"4", a[1]).Set(float64(1))
							}
							if state.POWER4 == "OFF" {
								sonoffGaugesPower.WithLabelValues(a[2]+"4", a[1]).Set(float64(0))
							}

						}
						if a[3] == "SENSOR" {
							sensor := SonoffSensor{}
							json.Unmarshal(message, &sensor)
							fmt.Printf("SENSOR: %+v\n", sensor)
							sonoffGaugesCurrent.WithLabelValues(a[2], a[1]).Set(sensor.ENERGY.Current)
							sonoffGaugesVoltage.WithLabelValues(a[2], a[1]).Set(float64(sensor.ENERGY.Voltage))
							sonoffGaugesPower.WithLabelValues(a[2], a[1]).Set(float64(sensor.ENERGY.Power))
							sonoffGaugesTotel.WithLabelValues(a[2], a[1]).Set(float64(sensor.ENERGY.Today))
						}
					}

				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// Wait for receiving a signal.
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)

	<-sigc

	// Disconnect the Network Connection.
	if err := cli.Disconnect(); err != nil {
		panic(err)
	}
}
